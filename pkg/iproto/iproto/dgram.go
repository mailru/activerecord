package iproto

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"

	"golang.org/x/net/context"

	pbufio "github.com/mailru/activerecord/pkg/iproto/util/bufio"
)

const (
	DefaultPacketReadBufferSize = 1 << 16 // maximum udp datagram size.
	DefaultMaxTransmissionUnit  = 1500
)

type PacketServerConfig struct {
	Handler Handler

	MaxTransmissionUnit int
	WriteQueueSize      int
	WriteTimeout        time.Duration

	ReadBufferSize int
	SizeLimit      uint32

	BytePool BytePool
	Logger   Logger

	ShutdownTimeout time.Duration

	OnPeerShutdown func(net.Addr)
}

func (pc *PacketServerConfig) withDefaults() (c PacketServerConfig) {
	if pc != nil {
		c = *pc
	}

	if c.MaxTransmissionUnit == 0 {
		c.MaxTransmissionUnit = DefaultMaxTransmissionUnit
	}

	if c.WriteQueueSize == 0 {
		c.WriteQueueSize = DefaultWriteQueueSize
	}

	if c.WriteTimeout == 0 {
		c.WriteTimeout = DefaultWriteTimeout
	}

	if c.ReadBufferSize == 0 {
		c.ReadBufferSize = DefaultPacketReadBufferSize
	}

	if c.SizeLimit == 0 {
		c.SizeLimit = DefaultSizeLimit
	}

	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = DefaultShutdownTimeout
	}

	if c.BytePool == nil {
		c.BytePool = defaultBytePool
	}

	if c.Logger == nil {
		c.Logger = DefaultLogger{}
	}

	return c
}

// PacketServer contains logic of handling iproto packets from datagram
// oriented network protocols such as "udp" or "unixgram".
type PacketServer struct {
	once sync.Once
	mu   sync.Mutex

	conn      net.PacketConn
	config    PacketServerConfig
	out       chan bytesWithAddr
	readDone  chan struct{}
	writeDone chan struct{}
	stopper   chan struct{}
	done      chan struct{}
	closed    bool
	stopped   bool
	err       error
}

func ListenPacket(network, addr string, config *PacketServerConfig) (*PacketServer, error) {
	conn, err := net.ListenPacket(network, addr)
	if err != nil {
		return nil, err
	}

	ps := NewPacketServer(conn, config)

	return ps, ps.Init()
}

func NewPacketServer(conn net.PacketConn, config *PacketServerConfig) *PacketServer {
	c := config.withDefaults()
	c.Logger = prefixLogger{fmt.Sprintf(
		`iproto: datagram server %q %s: `,
		conn.LocalAddr().Network(), conn.LocalAddr(),
	), c.Logger}

	return &PacketServer{
		conn:      conn,
		config:    c,
		out:       make(chan bytesWithAddr, c.WriteQueueSize),
		readDone:  make(chan struct{}),
		writeDone: make(chan struct{}),
		stopper:   make(chan struct{}),
		done:      make(chan struct{}),
	}
}

func (p *PacketServer) Init() (err error) {
	p.once.Do(func() {
		go p.reader()
		go p.writer()
		p.debugf(bg, "initialized successfully")
	})

	return
}

func (p *PacketServer) GetBytes(n int) []byte {
	return p.config.BytePool.Get(n)
}

func (p *PacketServer) PutBytes(bts []byte) {
	p.config.BytePool.Put(bts)
}

func (p *PacketServer) LocalAddr() net.Addr {
	return p.conn.LocalAddr()
}

func (p *PacketServer) Shutdown() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}

	p.stopped = true
	p.mu.Unlock()

	timer := time.AfterFunc(p.config.ShutdownTimeout, func() {
		p.logf(bg, "shutdown timeout exceeded")
		p.Close()
	})

	defer p.Close()
	defer timer.Stop()

	close(p.stopper)
	<-p.writeDone
}

func (p *PacketServer) Close() {
	p.drop()
	<-p.done
}

func (p *PacketServer) writer() {
	defer close(p.writeDone)

	// writerTo helps to use bufio.Writer for glueing same destination packets.
	w := writerTo{p.conn, nil}
	mtu := p.config.MaxTransmissionUnit
	buf := pbufio.AcquireWriterSize(&w, mtu)

	defer pbufio.ReleaseWriter(buf, mtu)

	var exit bool
	for !exit {
		var ba bytesWithAddr
		select {
		case ba = <-p.out:
		case <-p.stopper:
			exit = true
			// Give a chance for last queued packets to be written.
			select {
			case ba = <-p.out:
			default:
			}
		}

		if ba.bytes == nil {
			// No bytes to write. Probably stopping.
			continue
		}

		for q := len(p.out); ; q-- {
			switch {
			case w.addr != ba.addr:
				// Next packet can not be buffered due to its new destination address.
				// We must flush packets for previous address before buffering.
				buf.Flush()

				w.addr = ba.addr

			case buf.Available() < len(ba.bytes):
				// Must flush previously buffered packets. That is, we want the
				// datagram to have only complete packets.
				buf.Flush()
			}

			// Buffer packet bytes.
			_, _ = buf.Write(ba.bytes)
			// Return packet bytes to the pool because we have copied them to
			// the bufio.Writer. We took them from the pool inside p.send().
			p.PutBytes(ba.bytes)

			if q == 0 {
				break
			}

			ba = <-p.out
		}
		// Give other goroutines a chance to write more packets into p.out.
		// This is used to reduce the number of write system calls.
		runtime.Gosched()
		// Note that len(p.out) could only be >0 if p is not stopped.
		// When we get <-p.stopper signal then no more packets can be queued.
		// Thus draining the p.out above guarantees us that p.out is zero here
		// when stopped is true.
		if len(p.out) > 0 {
			// Avoid buffer flushing to prevent additional write syscall.
			continue
		}

		// Finally flush the datagram to the last destination.
		if err := buf.Flush(); err != nil {
			p.fatalf("flush buffer error: %v", err)
			return
		}
	}
}

func (p *PacketServer) reader() {
	defer close(p.readDone)

	var (
		onShutdown = p.config.OnPeerShutdown
		handler    = p.config.Handler
	)

	buf := p.GetBytes(p.config.ReadBufferSize)
	defer p.PutBytes(buf)

	// We will use bytes.Reader backed by buf as io.Reader for StreamReader.
	r := bytes.NewReader(nil)
	s := StreamReader{
		Source:    r,
		SizeLimit: p.config.SizeLimit,
		Alloc:     p.config.BytePool.Get,
	}

	for {
		n, addr, err := p.conn.ReadFrom(buf)
		if err != nil {
			if err == io.EOF {
				p.drop()
				return
			}

			p.fatalf("read packet error: %v", err)

			return
		}
		// Reset the reader to read n received bytes.
		r.Reset(buf[:n])
		// Single datagram can contain multiple iproto packets. Thus we must
		// read them all. If some packet broken, we drop whole datagram.
		for r.Len() > 0 {
			pkt, err := s.ReadPacket()
			if err != nil {
				p.logf(bg,
					"can not read packet from %s: %v; dropping datagram",
					addr, err,
				)

				break
			}
			select {
			case <-p.stopper:
				// Do not handle packet if we are stopped.
				return
			default:
			}

			switch pkt.Header.Msg {
			case MessagePing:
				// Just reply to the ping. It is always a ping from peer – we
				// are not pinging any one.
				_ = p.send(bg, addr, pingPacket)

			case MessageShutdown:
				if onShutdown != nil {
					// If client has shutdown logic, let it happen and then
					// reply to shutdown.
					go func() {
						onShutdown(addr)
						_ = p.send(bg, addr, shutdownPacket)
					}()
				} else {
					// Client has no shutdown logic, reply immediately.
					_ = p.send(bg, addr, shutdownPacket)
				}
			default:
				// TODO(s.kamardin): peer escapes to the heap here.
				if handler != nil {
					handler.ServeIProto(bg, peer{p, addr}, pkt)
				}
			}
		}
	}
}

func (p *PacketServer) send(ctx context.Context, addr net.Addr, pkt Packet) (err error) {
	bts := p.GetBytes(PacketSize(pkt))
	PutPacket(bts, pkt)

	select {
	case p.out <- bytesWithAddr{bts, addr}:
		return nil
	default:
		select {
		case p.out <- bytesWithAddr{bts, addr}:
			return nil
		case <-p.writeDone:
			err = ErrStopped
		case <-ctx.Done():
			err = ctx.Err()
		}
	}

	// Release bytes because of an error.
	p.PutBytes(bts)

	p.logf(ctx,
		"outgoing packet %#02x has been dropped: %s",
		pkt.Header.Msg, err,
	)

	return
}

func (p *PacketServer) drop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.closed = true
	p.debugf(bg, "dropping connection")

	if !p.stopped {
		p.stopped = true
		close(p.stopper)
	}

	p.conn.Close()

	go func() {
		<-p.writeDone
		<-p.readDone
		close(p.done)
	}()
}

func (p *PacketServer) fatalf(f string, args ...interface{}) {
	p.fatal(fmt.Errorf(f, args...))
}

func (p *PacketServer) fatal(err error) {
	p.mu.Lock()
	if !p.closed && p.err == nil {
		p.err = err
		p.logf(bg, "fatal error: %v; closing packet connection", err)
	}
	p.mu.Unlock()
	p.drop()
}

func (p *PacketServer) logf(ctx context.Context, f string, args ...interface{}) {
	p.config.Logger.Printf(ctx, f, args...)
}

func (p *PacketServer) debugf(ctx context.Context, f string, args ...interface{}) {
	p.config.Logger.Debugf(ctx, f, args...)
}

type peer struct {
	s    *PacketServer
	addr net.Addr
}

func (p peer) Call(_ context.Context, _ uint32, _ []byte) ([]byte, error) {
	return nil, errors.New("Call() is not supported")
}

func (p peer) Notify(ctx context.Context, method uint32, data []byte) error {
	return p.s.send(ctx, p.addr, Packet{
		Data: data,
		Header: Header{
			Msg: method,
			Len: uint32(len(data)),
		},
	})
}

func (p peer) Send(ctx context.Context, pkt Packet) error {
	return p.s.send(ctx, p.addr, pkt)
}

func (p peer) PutBytes(b []byte)     { p.s.PutBytes(b) }
func (p peer) GetBytes(n int) []byte { return p.s.GetBytes(n) }
func (p peer) LocalAddr() net.Addr   { return p.s.LocalAddr() }
func (p peer) RemoteAddr() net.Addr  { return p.addr }
func (p peer) Done() <-chan struct{} { return nil }
func (p peer) Close()                {}
func (p peer) Shutdown()             {}
func (p peer) OnClose(func())        {}

type bytesWithAddr struct {
	bytes []byte
	addr  net.Addr
}

type writerTo struct {
	net.PacketConn
	addr net.Addr
}

func (w writerTo) Write(p []byte) (int, error) {
	return w.WriteTo(p, w.addr)
}
