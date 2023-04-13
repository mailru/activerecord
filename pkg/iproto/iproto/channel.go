package iproto

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobwas/pool/pbytes"

	pbufio "github.com/mailru/activerecord/pkg/iproto/util/bufio"
	wio "github.com/mailru/activerecord/pkg/iproto/util/io"
	egotime "github.com/mailru/activerecord/pkg/iproto/util/time"

	"golang.org/x/net/context"
)

// Constant control message codes.
// All control message codes should be greater than MessagePing.
const (
	MessagePing     = 0xff00
	MessageShutdown = 0xff01
)

const (
	DefaultPingInterval    = time.Minute
	DefaultShutdownTimeout = 5 * time.Second

	DefaultReadBufferSize = 256
	DefaultSizeLimit      = 1e8

	DefaultWriteQueueSize  = 50
	DefaultWriteTimeout    = 5 * time.Second
	DefaultWriteBufferSize = 4096

	DefaultRequestTimeout = 5 * time.Second
	DefaultNoticeTimeout  = 1 * time.Second
)

var ErrTimeout = errors.New("timed out")
var ErrDroppedConn = errors.New("connection is gone")

var (
	logReadWriteGoroutine = false
)

type Logger interface {
	Printf(ctx context.Context, fmt string, v ...interface{})
	Debugf(ctx context.Context, fmt string, v ...interface{})
}

// BytePool describes an object that contains bytes buffer reuse logic.
type BytePool interface {
	// Get obtains buffer from pool with given length.
	Get(int) []byte
	// Put reclaims given buffer for further reuse.
	Put([]byte)
}

type ChannelConfig struct {
	Handler Handler

	ReadBufferSize int
	SizeLimit      uint32

	WriteQueueSize  int
	WriteTimeout    time.Duration
	WriteBufferSize int

	RequestTimeout time.Duration
	NoticeTimeout  time.Duration

	ShutdownTimeout time.Duration
	IdleTimeout     time.Duration
	PingInterval    time.Duration
	DisablePing     bool
	DisableShutdown bool

	Logger   Logger
	BytePool BytePool

	// Init is called right after Channel initialization inside Run() method.
	// That is, when Init is called, reader and writer goroutines are started
	// already. Thus two things should be noticed: first, you can send any
	// packets to peer inside Init; second, channel is already handling packets
	// when Init is called, so beware of races.
	//
	// If Init returns error the channel will be closed immediately.
	//
	// It is guaranteed that the callbacks set by Channel.On*() methods inside
	// Init can only be called after Init returns.
	//
	// It is Init implementation responsibility to handle timeouts during
	// initialization. That is, it could block some significant iproto parts
	// depending on caller – iproto.Server within accept loop, iproto.Pool
	// withing dialing, or some custom user initiator.
	//
	// If Init is nil then no additional initialization prepared.
	Init func(context.Context, *Channel) error

	// GetCustomRequestTimeout provides the ability to change request timeout
	// For example to increase the timeout only for a first request
	GetCustomRequestTimeout func() time.Duration
}

var (
	defaultBytePool = func(p *pbytes.Pool) BytePool {
		return BytePoolFunc(
			p.GetLen, p.Put,
		)
	}(pbytes.New(256, 65536))
)

func (cc *ChannelConfig) withDefaults() (c ChannelConfig) {
	if cc != nil {
		c = *cc
	}

	if c.Logger == nil {
		c.Logger = DefaultLogger{}
	}

	if c.WriteQueueSize == 0 {
		c.WriteQueueSize = DefaultWriteQueueSize
	}

	if c.RequestTimeout == 0 {
		c.RequestTimeout = DefaultRequestTimeout
	}

	if c.NoticeTimeout == 0 {
		c.NoticeTimeout = DefaultNoticeTimeout
	}

	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = DefaultShutdownTimeout
	}

	if c.WriteBufferSize == 0 {
		c.WriteBufferSize = DefaultWriteBufferSize
	}

	if c.WriteTimeout == 0 {
		c.WriteTimeout = DefaultWriteTimeout
	}

	if c.ReadBufferSize == 0 {
		c.ReadBufferSize = DefaultReadBufferSize
	}

	if c.SizeLimit == 0 {
		c.SizeLimit = DefaultSizeLimit
	}

	if c.BytePool == nil {
		c.BytePool = defaultBytePool
	}

	return c
}

// Channel provides low-level packet send/receive operations that are same for
// both server and client.
type Channel struct {
	mu   sync.RWMutex
	once sync.Once

	config ChannelConfig

	conn net.Conn
	ctx  context.Context // Used for graceful interrupt channel

	hijacked         bool
	hijackReadBuffer []byte

	out       chan []byte
	stopper   chan struct{} // Used for stopping writer goroutine.
	shutdown  chan struct{} // Used for signaling about receiving shutdown packet.
	writeDone chan struct{} // Used for signaling that writer goroutine is exited.
	readDone  chan struct{} // Used for signaling that reader goroutine is exited.
	done      chan struct{} // Used for signaling that channel is completely closed.

	cb         sync.Mutex // Used for callbacks serialization.
	onShutdown []func()
	onClose    []func()

	stopped bool
	dropped bool

	pending *store

	err error // Error that caused close of the channel.

	idleTimer *time.Timer
	pingTimer *time.Timer
	lastPing  time.Time

	stats ChannelStats
}

func NewChannel(conn net.Conn, config *ChannelConfig) *Channel {
	c := config.withDefaults()
	c.Logger = prefixLogger{
		connPrefix(conn),
		c.Logger,
	}

	return &Channel{
		conn:      conn,
		ctx:       bg,
		config:    c,
		pending:   newStore(),
		out:       make(chan []byte, c.WriteQueueSize),
		done:      make(chan struct{}),
		stopper:   make(chan struct{}),
		shutdown:  make(chan struct{}, 1),
		writeDone: make(chan struct{}),
		readDone:  make(chan struct{}),
	}
}

// SetContext saves context inside Channel and used for graceful channel interrupt
func (c *Channel) SetContext(ctx context.Context) {
	c.ctx = ctx
}

// RunChannel is a helper function that calls NewChannel and ch.Init
// sequentially.
func RunChannel(conn net.Conn, config *ChannelConfig) (*Channel, error) {
	ch := NewChannel(conn, config)
	err := ch.Init()

	return ch, err
}

// Init makes Channel "alive". It starts necessary goroutines for packet
// processing and does other things like setting ping and idle timers.
//
// Note that Run could be called only once. All non-first calls will do nothing
// and return nil as an error.
//
// Callers should call ch.Close() or ch.Shutdown() to close the Channel and its
// underlying connection or cancel context set by ch.SetContext.
func (c *Channel) Init() (err error) {
	c.once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		c.OnClose(cancel)

		c.cb.Lock()
		{
			go c.reader(ctx)
			go c.writer()

			if init := c.config.Init; init != nil {
				err = init(ctx, c)
			}
		}
		c.cb.Unlock()

		if err != nil {
			c.logf(ctx, "initialization error: %s", err)
			c.Close()
			return
		}

		c.mu.Lock()
		defer c.mu.Unlock()

		if c.stopped {
			c.logf(ctx, "initialization interrupted: channel become stopped")
			err = ErrStopped
			return
		}
		if c.hijacked {
			c.logf(ctx, "initialization interrupted: channel become hijacked")
			err = ErrHijacked
			return
		}

		if p := c.config.PingInterval; p != 0 && !c.config.DisablePing {
			c.pingTimer = time.AfterFunc(p, func() {
				c.ping(p)
			})
		}
		if c.config.IdleTimeout != 0 {
			c.idleTimer = time.AfterFunc(c.config.IdleTimeout, func() {
				c.fatalf("inactivity for %s", c.config.IdleTimeout)
			})
		}

		// Remove pointer to Init function to prevent holding closures.
		// Note that this is safe here to modify c.config because it is
		// shallow copy of user passed ChannelConfig.
		c.config.Init = nil

		c.debugf(ctx, "initialized successfully")
	})

	return
}

// Hijack lets the caller take over the channel connection.
// After a call to Hijack channel gracefully stops its i/o routines and after
// that it will not do anything with the connection.
//
// Actually it calls Shutdown() under the hood, with exception that it does
// not sends shutdown packet to the peer and not closes the connection after
// reader and writer routines are done.
//
// It returns reader bytes buffer. It may contain data that was read from the
// connection after a call to Hijack and before Shutdown() returns. It may be
// not aligned to the Packet bounds.
//
// It returns non-nil error when channel is already stopped or hijacked or when
// error occured somewhere while shutting down the channel.
func (c *Channel) Hijack() (conn net.Conn, rbuf []byte, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return nil, nil, ErrStopped
	}

	if c.hijacked {
		return nil, nil, ErrHijacked
	}

	c.hijacked = true
	c.mu.Unlock()

	c.Shutdown()

	c.mu.Lock()

	if err = c.err; err != nil {
		return nil, nil, err
	}

	// Reset the deadlines. Do not check the error due to no things to do with
	// it.
	_ = c.conn.SetDeadline(noDeadline)

	c.logf(bg, "hijacked with %d buffered bytes to read", len(c.hijackReadBuffer))

	return c.conn, c.hijackReadBuffer, nil
}

// GetBytes obtains bytes from the Channel's byte pool.
func (c *Channel) GetBytes(n int) []byte {
	return c.config.BytePool.Get(n)
}

// PutBytes reclaims bytes to the Channel's byte pool.
func (c *Channel) PutBytes(p []byte) {
	c.config.BytePool.Put(p)
}

// RemoteAddr returns remote network address of the underlying connection.
func (c *Channel) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }

// LocalAddr returns local network address of the underlying connection.
func (c *Channel) LocalAddr() net.Addr { return c.conn.LocalAddr() }

// Call sends request with given method and data.
// It guarantees that caller will receive response or corresponding error.
//
// Note that it will return error after c.config.RequestTimeout even if ctx has
// higher timeout.
func (c *Channel) Call(ctx context.Context, method uint32, data []byte) (resp []byte, err error) {
	atomic.AddUint32(&c.stats.CallCount, 1)

	res := acquireResultFunc()
	defer releaseResultFunc(res)

	requestTimeout := c.config.RequestTimeout
	if c.config.GetCustomRequestTimeout != nil {
		requestTimeout = c.config.GetCustomRequestTimeout()
	}

	timer := egotime.AcquireTimer(requestTimeout)
	defer egotime.ReleaseTimer(timer)

	sync := c.pending.push(method, res.cb)

	pkt := Packet{
		Data: data,
		Header: Header{
			Msg:  method,
			Len:  uint32(len(data)),
			Sync: sync,
		},
	}

	if err = c.send(ctx, timer.C, false, pkt); err != nil {
		c.pending.resolve(method, sync, nil, err)
	}

	var r result
	select {
	case r = <-res.ch:

	case <-timer.C:
		c.pending.resolve(method, sync, nil, ErrTimeout)

		r = <-res.ch

	case <-ctx.Done():
		c.pending.resolve(method, sync, nil, ctx.Err())

		r = <-res.ch
	}

	if resp, err = r.data, r.err; err != nil {
		atomic.AddUint32(&c.stats.CanceledCount, 1)
	}

	return resp, err
}

// Notify sends request with given method and data in 'fire and forget' manner.
//
// Note that it will return error after c.config.RequestTimeout even if ctx has
// higher timeout.
func (c *Channel) Notify(ctx context.Context, method uint32, data []byte) (err error) {
	atomic.AddUint32(&c.stats.NoticesCount, 1)

	timer := egotime.AcquireTimer(c.config.NoticeTimeout)
	defer egotime.ReleaseTimer(timer)

	pkt := Packet{
		Data: data,
		Header: Header{
			Msg: method,
			Len: uint32(len(data)),
		},
	}

	return c.send(ctx, timer.C, false, pkt)
}

// Send sends given packet.
//
// Note that it is preferable way of sending request response.
//
// It does not check state of channel as Call and Notify do. That is, it allows
// to try to send packet even if Channel is in shutdown state.
func (c *Channel) Send(ctx context.Context, p Packet) error {
	return c.send(ctx, nil, true, p)
}

// Done returns channel which closure means that underlying connection and
// other resources are closed.
// Returned channel is closed before any OnClose callback is called.
func (c *Channel) Done() <-chan struct{} {
	return c.done
}

// WriteDone returns channel which closure means that channel stopped to write.
func (c *Channel) WriteDone() <-chan struct{} {
	return c.writeDone
}

// ReadDone returns channel which closure means that channel stopped to read.
func (c *Channel) ReadDone() <-chan struct{} {
	return c.readDone
}

// OnClose allows to run a callback after channel and all its underlying
// resources are closed.
//
// Note that your cb could be delayed by other cb registered before and
// vise versa – if you doing some long job, you could delay other
// callbacks registered before.
//
// If you want to be informed as soon as possible, use c.Done(), which is
// closed before any callback is called.
func (c *Channel) OnClose(cb func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dropped {
		// Call the callback immediately because the channel is closing or
		// already closed.
		go func() {
			<-c.done
			c.execCallbacks(cb)
		}()
	} else {
		c.onClose = append(c.onClose, cb)
	}
}

// OnShutdown registers given callback to be called right before sending
// MessageShutdown to the peer (as response to already received message or as
// initial packet in shutdown "handshake").
//
// Note that a timer with ChannelConfig.ShutdownTimeout is initialized after
// callbacks execution. That is, if some callback did stuck, then whole
// Shutdown routine will also do.
// Note that if channel is already in shutdown phase, then callback will not be
// called.
func (c *Channel) OnShutdown(cb func()) {
	c.mu.Lock()

	defer c.mu.Unlock()

	if !c.stopped {
		c.onShutdown = append(c.onShutdown, cb)
	}
}

// Shutdown closes channel gracefully. It stops writing to the underlying connection and
// continues read from connection for a while if any pending requests were done.
// Then Close method called.
func (c *Channel) Shutdown() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		<-c.Done()

		return
	}

	// Stop subsequent calls/notices.
	c.stopped = true
	callbacks := c.onShutdown
	hijacked := c.hijacked
	c.mu.Unlock()

	// Wait for all pre-shutdown callbacks to be called.
	c.execCallbacks(callbacks...)

	timer := time.AfterFunc(c.config.ShutdownTimeout, func() {
		c.logf(bg, "shutdown timeout exceeded")
		c.Close()
	})

	defer c.Close()
	defer timer.Stop()

	// Wait for previous outgoing calls to become complete.
	// Pending size here can not grow after we've set stopped flag to true.
	// If timeout occurs then after c.Drop() all pending calls will be
	// rejected. Thus no deadlock here is possible.
	<-c.pending.empty()

	// We want prepare shutdown handshake only for non-hijacked connection.
	if !hijacked {
		// Send shutdown message to the peer.
		c.sendShutdownPacket()

		// Wait for shutdown-ack.
		// Receiving a shutdown means all remote client requests have been satisfied.
		// Will not block here if we are on shutdown receiving side.
		// Will block here until receive of response shutdown if we are on shutdown
		// initiator side.
		//
		// If timeout occurs then after c.Close() reader will exit and close
		// c.shutdown chan. Thus no deadlock here is possible.
		<-c.shutdown
	}

	// Let the shutdown packet to be sent. This is neccessary if we are on
	// shutdown receiving side. In other way it will be a racy write.
	//
	// We must close the stopper only after <-c.shutdown. That is, receiving
	// shutdown message means that other side does not expect any packets from
	// us anymore.
	close(c.stopper)
	<-c.writeDone
}

// Close drops the channel and returns when all resources are done.
func (c *Channel) Close() {
	c.Drop()
	<-c.done
}

// Drop closes underlying connection and sends signals to reader and writer
// goroutines to stop.
// It returns immediately, without awaiting for goroutines to exit.
func (c *Channel) Drop() {
	c.mu.Lock()
	if c.dropped {
		c.mu.Unlock()
		return
	}

	c.dropped = true

	if !c.stopped {
		c.stopped = true
		close(c.stopper)
	}

	if c.pingTimer != nil {
		c.pingTimer.Stop()
		c.pingTimer = nil
	}

	if c.idleTimer != nil {
		c.idleTimer.Stop()
		c.idleTimer = nil
	}

	if c.hijacked {
		// Cancel current read().
		err := c.conn.SetReadDeadline(aLongTimeAgo)
		if err != nil {
			// Connection does not provide mechanism to cancel the read routines.
			// In that case we can not prorcess Hijack() well.
			c.logf(bg, "hijack failed: %v; dropping connection", err)
			c.saveError(err)
			c.conn.Close()
		}
	} else {
		c.debugf(bg, "dropping connection")
		c.conn.Close()
	}

	c.pending.rejectAll(ErrDroppedConn)

	// Read callbacks and set them to nil to let
	// custom runtime finalizers work on the channel.
	onClose := c.onClose
	c.onClose = nil
	c.onShutdown = nil

	c.mu.Unlock()

	// Run callback calls in a separate goroutine preventing possible
	// deadlocks.
	go func() {
		<-c.writeDone
		<-c.readDone
		close(c.done)
		c.execCallbacks(onClose...)
	}()
}

var (
	// aLongTimeAgo is a non-zero time, far in the past, used for immediate
	// cancelation of readers.
	aLongTimeAgo = time.Unix(0, 0)
	noDeadline   = time.Time{}
)

func (c *Channel) Stats() ChannelStats {
	return c.stats.Copy()
}

// WriteClosed returns true if channel is closed for writing.
func (c *Channel) WriteClosed() bool {
	c.mu.RLock()
	stopped := c.stopped
	c.mu.RUnlock()

	return stopped
}

// Closed returns true if channel is closed.
func (c *Channel) Closed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

// Error returns error that caused close of the channel.
func (c *Channel) Error() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.err
}

var (
	pingPacket = Packet{Header{Msg: MessagePing}, nil}

	shutdownPacket     = Packet{Header{Msg: MessageShutdown}, nil}
	shutdownPacketSize = PacketSize(shutdownPacket)
)

func (c *Channel) sendShutdownPacket() {
	if c.config.DisableShutdown {
		return
	}

	// Prepare shutdown packet bytes to send. Need to copy it because writer
	// returns received from c.out bytes to the pool.
	p := c.GetBytes(shutdownPacketSize)
	PutPacket(p, shutdownPacket)

	select {
	case c.out <- p:
	case <-c.writeDone:
		return
	}
}

func (c *Channel) writer() {
	defer close(c.writeDone)

	if logReadWriteGoroutine {
		defer c.logf(bg, "iproto: write goroutine for conn(%s -> %s) done; channel: %+v", c.LocalAddr(), c.RemoteAddr(), c)
		c.logf(bg, "iproto: write goroutine for conn(%s -> %s) start; channel: %+v", c.LocalAddr(), c.RemoteAddr(), c)
	}

	buf := buffers{
		b:       make(net.Buffers, 0, 16),
		conn:    c.conn,
		timeout: c.config.WriteTimeout,
		size:    c.config.WriteBufferSize,
		free:    c.config.BytePool.Put,
	}

	var (
		stopped bool
		packets uint32
	)

	for !stopped {
		var p []byte
		select {
		case <-c.ctx.Done():
			c.ctx = bg
			go c.Shutdown()
		case p = <-c.out:
		case <-c.stopper:
			// stopper is closed always after c.stopped set to true. That is,
			// no more packets can arrive into c.out at this moment.
			stopped = true

			// Give a chance for last queued packets to be written.
			select {
			case p = <-c.out:
			default:
			}
		}

		if p == nil {
			continue
		}
		// Cache current length of c.out in q and read them all.
		for q := len(c.out); ; q-- {
			if err := buf.Append(p); err != nil {
				c.fatalf("buffer packet error: %v", err)
				return
			}

			packets++

			if q == 0 {
				break
			}

			p = <-c.out
		}
		// Give other goroutines a chance to write more packets into c.out.
		// This is used to reduce the number of write syscall calls and
		// increase performance.
		runtime.Gosched()
		// Note that len(c.out) could only be >0 if stopped is false.
		// When we get <-c.stopper signal then no more packets can be queued.
		// Thus draining the c.out above guarantees us that c.out is zero here
		// when stopped is true.
		if len(c.out) > 0 {
			// Avoid buffer flushing to prevent additional write syscall.
			continue
		}

		if err := buf.Flush(); err != nil {
			c.fatalf("flush buffer error: %v", err)
			return
		}

		atomic.StoreUint32(&c.stats.BytesSent, uint32(buf.sent))
		atomic.AddUint32(&c.stats.PacketsSent, packets)
		packets = 0
	}
}

//nolint:gocognit
func (c *Channel) reader(ctx context.Context) {
	defer close(c.readDone)
	defer close(c.shutdown)

	if logReadWriteGoroutine {
		defer c.logf(ctx, "iproto: reader goroutine for conn(%s -> %s) done; channel: %+v", c.LocalAddr(), c.RemoteAddr(), c)
		c.logf(ctx, "iproto: reader goroutine for conn(%s -> %s) start; channel: %+v", c.LocalAddr(), c.RemoteAddr(), c)
	}

	// Wrap c.conn to get read stats.
	wrapper := wio.WrapReader(c.conn)

	// Buffer reads from wrapper.
	bufSize := c.config.ReadBufferSize
	buf := pbufio.AcquireReaderSize(wrapper, bufSize)

	defer pbufio.ReleaseReader(buf, bufSize)

	stream := StreamReader{
		Source:    buf,
		SizeLimit: c.config.SizeLimit,
		Alloc:     c.config.BytePool.Get,
	}

	var hijackBuffer []byte

	for {
		packet, err := stream.ReadPacket()
		if err != nil {
			// Since Go 1.12 TCP keep-alives are enabled by default, and an error on a
			// read from a connection that was closed by a keep-alive has .Timeout() == true.
			// Therefore we must ensure that the timeout error was intentional to prevent
			// leaving channel half-broken through an abnormal reader quit.
			c.mu.RLock()
			hijacked := c.hijacked
			c.mu.RUnlock()

			if neterr, ok := err.(net.Error); ok && neterr.Timeout() && hijacked {
				// Read deadline was set to unblock the reader. That is,
				// someone wants to hijack the connection.
				if n := stream.LastRead(); n != 0 {
					// Hijack splits the incoming packet.
					// Must buffer n read bytes from the packet.
					//
					// Marshaled packet bytes will probably contain not
					// complete and incorrect data. Thats okay for us because
					// bytes ordered the same way that they streamed. So we
					// just take n bytes from the stream and buffer them.
					p := MarshalPacket(packet)
					hijackBuffer = append(hijackBuffer, p[:n]...)
				}

				// Do not loose the buffered bytes inside the bufio.Reader.
				buffered, _ := buf.Peek(buf.Buffered())
				hijackBuffer = append(hijackBuffer, buffered...)

				c.mu.Lock()
				c.hijackReadBuffer = hijackBuffer
				c.mu.Unlock()

				return
			}

			// NOTE: we do not check io.EOF here because we rely on MessageShutdown
			// packet as a signal to gracefully shutdown the connection.
			if p := c.pending.size(); err == io.EOF && p == 0 {
				c.Drop()
			} else {
				c.fatalf(
					"receive packet error while having %d pending call(s): %v",
					p, err,
				)
			}

			return
		}

		atomic.AddUint32(&c.stats.PacketsReceived, 1)

		stat := wrapper.Stat()
		atomic.StoreUint32(&c.stats.BytesReceived, stat.Bytes)

		isResponse := c.pending.resolve(
			packet.Header.Msg,
			packet.Header.Sync,
			packet.Data,
			nil,
		)

		c.mu.RLock()
		stopped := c.stopped
		dropped := c.dropped
		hijacked := c.hijacked
		idleTimer := c.idleTimer
		c.mu.RUnlock()

		if idleTimer != nil {
			idleTimer.Reset(c.config.IdleTimeout)
		}

		if isResponse {
			continue
		}

		if hijacked {
			hijackBuffer = append(hijackBuffer, MarshalPacket(packet)...)
			continue
		}

		switch packet.Header.Msg {
		case MessagePing:
			c.mu.Lock()
			waitPingAck := !c.lastPing.IsZero()
			c.lastPing = time.Time{}
			c.mu.Unlock()

			// When both sides ping and we ping with sync = 0
			// and the other side with sync != 0, it is necessary to process such ping anyway
			if !waitPingAck || packet.Header.Sync != 0 {
				err := c.Send(ctx, Packet{
					Header: Header{
						Msg:  MessagePing,
						Sync: packet.Header.Sync,
					},
				})
				if err != nil {
					c.logf(ctx, "warning: sending ping(ack) failed: %v", err)
				}
			}

		case MessageShutdown:
			select {
			case c.shutdown <- struct{}{}:
			default:
				c.logf(bg, "warning: peer is sending too many shutdown packets")
				continue
			}

			if stopped {
				continue
			}

			go c.Shutdown()

		default:
			if !dropped && c.config.Handler != nil {
				c.config.Handler.ServeIProto(ctx, c, packet)
			}
		}
	}
}

func (c *Channel) send(ctx context.Context, timeout <-chan time.Time, force bool, pkt Packet) (err error) {
	if !force {
		c.mu.RLock()
		stopped := c.stopped
		hijacked := c.hijacked
		c.mu.RUnlock()

		if stopped {
			return ErrStopped
		}

		if hijacked {
			return ErrHijacked
		}
	}

	p := c.GetBytes(PacketSize(pkt))
	PutPacket(p, pkt)

	select {
	case c.out <- p:
		return nil
	default:
		select {
		case c.out <- p:
			return nil

		case <-c.writeDone:
			err = ErrStopped
		case <-timeout:
			err = ErrTimeout
		case <-ctx.Done():
			err = ctx.Err()
		}
	}

	// Release bytes because of error.
	c.PutBytes(p)

	atomic.AddUint32(&c.stats.PacketsDropped, 1)

	return
}

func (c *Channel) ping(d time.Duration) {
	var wait time.Duration

	c.mu.Lock()
	{
		if c.dropped {
			c.mu.Unlock()
			return
		}

		if c.lastPing.IsZero() {
			c.lastPing = time.Now()
		} else {
			wait = time.Since(c.lastPing)
		}
	}
	c.mu.Unlock()

	if wait > 2*d {
		c.fatalf("ping is missed for %s", time.Since(c.lastPing))
		return
	}

	if wait == 0 {
		// Send only one ping at a time to avoid confusion between original pings and acks,
		// see TestChannelDelayedPingInfiniteLoop.
		err := c.Notify(bg, MessagePing, nil)
		if err != nil {
			c.logf(bg, "warning: sending ping(request) failed: %v", err)
		}
	}

	c.resetPingTimer(d)
}

func (c *Channel) resetPingTimer(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dropped {
		return
	}

	c.pingTimer.Reset(d)
}

// fatalf is the same as fatal excepts f.
func (c *Channel) fatalf(cause string, args ...interface{}) {
	c.fatal(fmt.Errorf(cause, args...))
}

// fatal closes channel with given error as a cause.
// Given err is ignored if the channel was errored before.
func (c *Channel) fatal(err error) {
	c.mu.Lock()
	if !c.dropped && c.saveError(err) {
		c.logf(bg, "fatal error: %v; channel will be dropped", err)
	}
	c.mu.Unlock()
	c.Drop()
}

func (c *Channel) saveError(err error) bool {
	if c.err != nil {
		return false
	}

	c.err = err

	return true
}

func (c *Channel) logf(ctx context.Context, f string, arg ...interface{}) {
	c.config.Logger.Printf(ctx, f, arg...)
}

func (c *Channel) debugf(ctx context.Context, f string, arg ...interface{}) {
	c.config.Logger.Debugf(ctx, f, arg...)
}

func (c *Channel) execCallbacks(cbs ...func()) {
	if len(cbs) == 0 {
		return
	}

	c.cb.Lock()

	defer c.cb.Unlock()

	for _, cb := range cbs {
		cb()
	}
}

// ResponseTo constructs new Packet that is a response to p.
func ResponseTo(p Packet, b []byte) Packet {
	p.Data = b
	p.Header.Len = uint32(len(b))

	return p
}

// ChannelStats shows statistics of channel usage.
type ChannelStats struct {
	PacketsSent     uint32 // total number of sent packets
	PacketsReceived uint32 // total number of received packets
	PacketsDropped  uint32 // total number of dropped packets

	BytesSent     uint32 // total number of sent bytes
	BytesReceived uint32 // total number of received bytes

	CallCount     uint32 // total number of call attempts
	CanceledCount uint32 // total number of call attempts (failed or not) that has been dropped by any reason

	PendingCount uint32 // number of calls sent, for which we still expect a response

	NoticesCount uint32 // total number of notify attempts
}

func (c ChannelStats) Add(s ChannelStats) ChannelStats {
	return ChannelStats{
		PacketsSent:     c.PacketsSent + s.PacketsSent,
		PacketsReceived: c.PacketsReceived + s.PacketsReceived,
		PacketsDropped:  c.PacketsDropped + s.PacketsDropped,
		BytesSent:       c.BytesSent + s.BytesSent,
		BytesReceived:   c.BytesReceived + s.BytesReceived,
		CallCount:       c.CallCount + s.CallCount,
		PendingCount:    c.PendingCount + s.PendingCount,
		CanceledCount:   c.CanceledCount + s.CanceledCount,
		NoticesCount:    c.NoticesCount + s.NoticesCount,
	}
}

func (c *ChannelStats) Copy() (s ChannelStats) {
	s.PacketsSent = atomic.LoadUint32(&c.PacketsSent)
	s.PacketsReceived = atomic.LoadUint32(&c.PacketsReceived)
	s.PacketsDropped = atomic.LoadUint32(&c.PacketsDropped)
	s.BytesSent = atomic.LoadUint32(&c.BytesSent)
	s.BytesReceived = atomic.LoadUint32(&c.BytesReceived)
	s.CallCount = atomic.LoadUint32(&c.CallCount)
	s.PendingCount = atomic.LoadUint32(&c.PendingCount)
	s.CanceledCount = atomic.LoadUint32(&c.CanceledCount)
	s.NoticesCount = atomic.LoadUint32(&c.NoticesCount)

	return
}

// DefaultLogger implements Logger interface.
type DefaultLogger struct {
	// Prefix is used as a default prefix when given ctx does not implement a
	// ctxlog.Context.
	Prefix string
}

// Printf logs message in Sprintf form.
// If ctx implements ctxlog.Context then ctxlog package will be used to print
// the message. In other way it prints message with d.Prefix via log package.
func (d DefaultLogger) Printf(ctx context.Context, f string, args ...interface{}) {
	log.Print(d.Prefix, fmt.Sprintf(f, args...))
}

// Debugf logs message in Sprintf form.
// If ctx implements ctxlog.Context then ctxlog package will be used to print
// the message. In other way it prints message with d.Prefix via log package.
func (d DefaultLogger) Debugf(ctx context.Context, f string, args ...interface{}) {
	log.Printf(d.Prefix+f, args...)
}

type prefixLogger struct {
	prefix string
	logger Logger
}

func (p prefixLogger) Printf(ctx context.Context, f string, args ...interface{}) {
	p.logger.Printf(ctx, fmt.Sprint(p.prefix, fmt.Sprintf(f, args...)))
}

func (p prefixLogger) Debugf(ctx context.Context, f string, args ...interface{}) {
	p.logger.Debugf(ctx, fmt.Sprint(p.prefix, fmt.Sprintf(f, args...)))
}

type addressor interface {
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

func nameConn(conn addressor) string {
	return fmt.Sprintf(
		"%q %s > %s",
		conn.LocalAddr().Network(), conn.LocalAddr(), conn.RemoteAddr(),
	)
}

func connPrefix(conn addressor) string {
	local := conn.LocalAddr()
	remote := conn.RemoteAddr()

	if remote == nil {
		return fmt.Sprintf(
			`iproto: datagram chan %q %s: `,
			local.Network(), local,
		)
	}

	return fmt.Sprintf(
		`iproto: chan %q %s > %s: `,
		conn.LocalAddr().Network(),
		conn.LocalAddr().String(),
		conn.RemoteAddr().String(),
	)
}
