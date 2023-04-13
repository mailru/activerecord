package iproto

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mailru/activerecord/pkg/iproto/context/ctxlog"
	"github.com/mailru/activerecord/pkg/iproto/netutil"
	"github.com/mailru/activerecord/pkg/iproto/syncutil"
	egoTime "github.com/mailru/activerecord/pkg/iproto/util/time"

	"golang.org/x/net/context"
	"golang.org/x/time/rate"
)

const (
	DefaultPoolConnectTimeout = time.Second
	DefaultPoolSize           = 1
	DefaultPoolRedialInterval = time.Millisecond * 10
	DefaultPoolRedialTimeout  = time.Hour
)

var (
	ErrStopped   = fmt.Errorf("channel is stopped")
	ErrHijacked  = fmt.Errorf("channel connection hijacked")
	ErrCutoff    = fmt.Errorf("pool is offline")
	ErrPoolFull  = fmt.Errorf("pool is full")
	ErrThrottled = fmt.Errorf("dial attempt was throttled")
	ErrNoChannel = fmt.Errorf("get channel error: no channel before timeout")
	ErrPolicied  = fmt.Errorf("policied due to rate limit")
)

var bg = context.Background()

// RateError provides the ability to determine if the request exceeds current pool's rate limit
type RateError struct {
	Err error
}

func (e *RateError) Error() string { return "rate limit exceeded" }

func (e *RateError) Unwrap() error { return e.Err }

// PoolConfig contains options for Pool configuration.
type PoolConfig struct {
	// Size limits number of connections used by Pool.
	// When Size is zero, single connection is used.
	Size int

	// ChannelConfig will be passed to Channel constructor when new connection
	// established.
	ChannelConfig *ChannelConfig

	// NetDial allows to override standard net.Dial logic.
	NetDial func(ctx context.Context, network, addr string) (net.Conn, error)

	// ConntectTimeout is the maximum time spent on awaiting for alive iproto connection
	// for sending packet.
	ConnectTimeout time.Duration

	// DialTimeout is the maximum time spent on establishing new iproto connection.
	DialTimeout time.Duration

	// RedialInterval is used inside dial routine for delaying next attempt to establish
	// new iproto connection after previous attempt was failed.
	RedialInterval time.Duration

	// MaxRedialInterval is the maximum delay before next attempt to establish new iproto
	// connection is prepared. Note that RedialInterval is used as initial delay time,
	// and could be increased by every dial attempt up to MaxRedialInterval.
	MaxRedialInterval time.Duration

	// RedialTimeout is the maximum time spent in dial routine.
	// If RedialForever is true this parameter is not used.
	RedialTimeout time.Duration

	// RedialForever configures the underlying dialer to dial without any
	// timeout.
	RedialForever bool

	// FailOnCutoff prevents Pool of waiting for a dial for a new
	// Channel when there are no online Channels to proceed the request.
	// If this field is true, then any Call() Notify() or NextChannel() calls
	// will be failed with ErrCutoff error if no online Channels currently are
	// available.
	FailOnCutoff bool

	// DialThrottle allows to limit rate of the dialer routine execution.
	DialThrottle time.Duration

	// Logger is used to handle some log messages.
	// If nil, default logger is used.
	Logger Logger

	// If true, pool will not try to establish a new connection right after one
	// is closed.
	DisablePreDial bool

	// RateLimit is the limit parameter for golang.org/x/time/rate token bucket rate limiter.
	// A zero value causes the rate limiter to reject all events if `RateBurst' is nonzero.
	RateLimit rate.Limit

	// RateBurst is the burst parameter of time/rate token bucket rate limiter.
	// A zero value disables the rate limiter.
	RateBurst int

	// RateWait is used to determine the desired behaviour of request rate limiting
	// on the pool. A zero value sets the requests to rather be "policied" than "shaped",
	// and the corresponding function calls will return `RateError' caused by `ErrPolicied'.
	// when limiter is unable to satisfy the request. With a value of `true' the limiter will
	// rather provide "shaping" of the request flow, and the corresponding function calls
	// will either block until the request can be satisfied, or return at once if the
	// predicted wait time exceeds the context deadline. Cancellation of the context also
	// forces a premature return with an error.
	// See golang.org/x/time/rate documentation for .Allow() and .Wait() behaviour.
	RateWait bool
}

func (p *PoolConfig) withDefaults() (c PoolConfig) {
	if p != nil {
		c = *p
	}

	// Copy Channel config preventing its mutation. This is also helps us to be
	// sure that pool.config.ChannelConfig is not nil.
	cp := c.ChannelConfig.withDefaults()
	c.ChannelConfig = &cp

	if c.Logger == nil {
		c.Logger = DefaultLogger{}
	}

	if c.Size == 0 {
		c.Size = DefaultPoolSize
	}

	if c.RedialInterval == 0 {
		c.RedialInterval = DefaultPoolRedialInterval
	}

	if c.RedialTimeout == 0 {
		c.RedialTimeout = DefaultPoolRedialTimeout
	}

	if c.ConnectTimeout == 0 {
		c.ConnectTimeout = DefaultPoolConnectTimeout
	}

	return
}

// Pool represents struct that could manage multiple iproto connections to one host.
// It establishes/reestablishes connections automatically.
type Pool struct {
	mu sync.RWMutex

	closed  bool
	stopped bool
	onClose []func()

	dialer *netutil.BackgroundDialer

	done chan struct{}

	network string
	addr    string

	online []*Channel
	next   uint32

	config PoolConfig

	dialThrottle   *syncutil.Throttle
	dialTaskGroup  *syncutil.TaskGroup
	dialTaskRunner *syncutil.TaskRunner

	rateLimiter *rate.Limiter
	rateWait    bool

	stats PoolStats
}

// NewPool creates new pool of iproto connections at addr.
func NewPool(network, addr string, config *PoolConfig) *Pool {
	c := config.withDefaults()
	c.Logger = prefixLogger{
		fmt.Sprintf(`iproto: pool %q %s: `, network, addr),
		c.Logger,
	}

	p := &Pool{
		network: network,
		addr:    addr,
		done:    make(chan struct{}),
		config:  c,

		dialThrottle:   syncutil.NewThrottle(c.DialThrottle),
		dialTaskGroup:  &syncutil.TaskGroup{N: c.Size},
		dialTaskRunner: &syncutil.TaskRunner{},

		rateWait: c.RateWait,
	}

	if c.RateBurst != 0 {
		if c.RateLimit != 0 {
			p.rateLimiter = rate.NewLimiter(c.RateLimit, c.RateBurst)
		} else {
			// The zero value is a valid Limiter, but it will reject all events.
			p.rateLimiter = new(rate.Limiter)
		}
	}

	d := &netutil.Dialer{
		Network: network,
		Addr:    addr,

		Timeout: c.DialTimeout,

		// Do not set LoopTimeout cause we making the same behaviour with
		// p.dialer.SetDeadline().
		LoopTimeout: 0,

		LoopInterval:    c.RedialInterval,
		MaxLoopInterval: c.MaxRedialInterval,

		Closed: p.done,

		Logf:           p.logf,
		Debugf:         p.debugf,
		DisableLogAddr: true,

		NetDial: c.NetDial,

		OnAttempt: func(err error) {
			atomic.AddUint32(&p.stats.DialCount, 1)
			if err != nil {
				atomic.AddUint32(&p.stats.DialErrors, 1)
			}
		},
	}

	p.dialer = &netutil.BackgroundDialer{
		Dialer:     d,
		TaskGroup:  p.dialTaskGroup,
		TaskRunner: p.dialTaskRunner,
	}

	return p
}

// Dial creates new pool of iproto channel(s).
// It returns error if initialization of first channel was failed.
//
// Callers should call pool.Close() or pool.Shutdown() methods when
// work with created pool is complete.
//
// Given ctx will not be saved inside Pool.
// If ctx implements ctxlog.Context and config does not contains Logger, then
// DefaultLogger will be used with Prefix set to ctx.LogPrefix().
func Dial(ctx context.Context, network, addr string, config *PoolConfig) (pool *Pool, err error) {
	config = setContextLogger(ctx, config)

	pool = NewPool(network, addr, config)
	if err = pool.Init(ctx); err != nil {
		pool.Close()
		pool = nil
	}

	return
}

// Init initializes Pool and tries to create single iproto connection.
func (p *Pool) Init(ctx context.Context) (err error) {
	return p.connect(ctx, 1)
}

// InitAll is the same as Init, except that it tries to get N connections, N = PoolConfig.Size || 1.
func (p *Pool) InitAll(ctx context.Context) (err error) {
	return p.connect(ctx, p.config.Size)
}

func (p *Pool) connect(ctx context.Context, n int) error {
	if n > p.config.Size {
		n = p.config.Size
	}

	m := len(p.Online())

	n -= m
	if n <= 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, p.config.ConnectTimeout)
	defer cancel()

	results := p.dialTaskGroup.Do(ctx, n, func(ctx context.Context, i int) error {
		conn, err := p.dialer.Dialer.Dial(ctx)
		if err == nil {
			err = p.StoreConn(conn)
		}
		// We do not want to fail on ErrPoolFull from StoreConn() – it just
		// means that some of our dials where senseless.
		if err == ErrPoolFull {
			// Dial OK.
			err = nil
		}
		return err
	})

	for _, r := range results {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-r:
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// GetBytes obtains bytes from the Channel config's byte pool.
func (p *Pool) GetBytes(n int) []byte {
	return p.config.ChannelConfig.BytePool.Get(n)
}

// PutBytes reclaims bytes to the Channel config's byte pool.
func (p *Pool) PutBytes(bts []byte) {
	p.config.ChannelConfig.BytePool.Put(bts)
}

func (p *Pool) RemoteAddr() net.Addr {
	return p.remoteAddr()
}

func (p *Pool) remoteAddr() poolAddr {
	return poolAddr{p.network, p.addr}
}

// Call finds next alive channel in pool and returns its Call() result.
func (p *Pool) Call(ctx context.Context, method uint32, data []byte) ([]byte, error) {
	_, resp, err := p.CallNext(ctx, method, data)
	return resp, err
}

// CallNext finds next alive channel in pool and returns found channel and its
// Call() result.
func (p *Pool) CallNext(ctx context.Context, method uint32, data []byte) (ch *Channel, resp []byte, err error) {
	for {
		ch, err = p.NextChannel(ctx)
		if err != nil {
			return
		}

		if resp, err = ch.Call(ctx, method, data); !isNoConnError(err) {
			return
		}
	}
}

// Notify finds for alive channel in pool and make Notify() on it.
func (p *Pool) Notify(ctx context.Context, method uint32, data []byte) (err error) {
	var c *Channel

	for {
		c, err = p.NextChannel(ctx)
		if err != nil {
			return
		}

		if err = c.Notify(ctx, method, data); !isNoConnError(err) {
			return
		}
	}
}

// Send finds alive channel in pool and make Send() on it.
func (p *Pool) Send(ctx context.Context, pkt Packet) (err error) {
	var c *Channel

	for {
		c, err = p.NextChannel(ctx)
		if err != nil {
			return
		}

		if err = c.Send(ctx, pkt); !isNoConnError(err) {
			return
		}
	}
}

// Close closes all underlying channels with channel.Close() method.
func (p *Pool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}

	p.closed = true
	p.stopped = true

	online := p.online
	p.online = nil
	p.saveChannelsStats(online)

	callbacks := p.onClose
	p.onClose = nil
	p.mu.Unlock()

	p.dialer.Cancel()

	p.debugf("closing channels for %s", p.addr)
	closeChannels(online)

	close(p.done)

	if len(callbacks) == 0 {
		return
	}

	// Run callbacks calls in a separate goroutine preventing possible
	// deadlocks.
	// Set p.onClose to nil to let custom runtime finalizers work.
	go func() {
		for _, cb := range callbacks {
			cb()
		}
	}()
}

// Shutdown closes all underlying channels by calling channel.Shutdown().
func (p *Pool) Shutdown() {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		<-p.Done()

		return
	}

	p.stopped = true

	online := p.online
	p.online = nil
	p.mu.Unlock()

	p.logf("shutting down channels for %s", p.addr)
	shutdownChannels(online)

	p.mu.Lock()
	p.saveChannelsStats(online)
	p.mu.Unlock()

	p.Close()
}

// Stats returns pool usage statistics.
func (p *Pool) Stats() (ret PoolStats) {
	p.mu.RLock()
	online := p.online
	ret.ChannelStats = p.stats.ChannelStats
	p.mu.RUnlock()

	ret.DialCount = atomic.LoadUint32(&p.stats.DialCount)
	ret.WaitErrors = atomic.LoadUint32(&p.stats.WaitErrors)
	ret.DialErrors = atomic.LoadUint32(&p.stats.DialErrors)
	ret.Online = len(online)

	for _, c := range online {
		ret.ChannelStats = ret.ChannelStats.Add(c.Stats())
	}

	return
}

// Done returns channel which closure signaling that pool resources are freed
// and no more actions could be performed.
// Note that Done is useful only after pool.{Close,Shutdown} calls.
func (p *Pool) Done() <-chan struct{} {
	return p.done
}

// OnClose allows to run a callback after pool resources are freed and no more actions could be performed.
// Note that your cb could be delayed by other cb registered before.
// And vise versa – if you doing some hard long job, you could delay other callbacks registered before.
// If you want to be informed as soon as possible, use pool.Done().
// Note that cb will be called only after pool.{Close,Shutdown} calls.
func (p *Pool) OnClose(cb func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onClose = append(p.onClose, cb)
}

// DialBackground initiates background dial for a new connection.
// If pool is configured to not RedialForever, then dial will end up after
// RedialTimeout config parameter.
//
// It returns a chan which closure means that dial were finished. Non-nil error
// means that dial was not started.
//
// Callers must take care of senseless dial when poll is full.
func (p *Pool) DialBackground() (<-chan error, error) {
	if !p.config.RedialForever {
		// If we do not want to dial forever, set deadline to now +
		// redial_timeout parameter from configuration.
		now := time.Now()
		p.dialer.SetDeadlineAtLeast(now.Add(p.config.RedialTimeout))
	}

	return p.dial()
}

// DialForeground initiates background dial for a new connection.
// If pool is configured to not RedialForever, then dial will end up after
// context's deadline exceeded.
//
// It returns a chan which closure means that dial were finished. Non-nil error
// means that dial was not started.
//
// Callers must take care of senseless dial when poll is full.
func (p *Pool) DialForeground(ctx context.Context) (<-chan error, error) {
	if !p.config.RedialForever {
		// If we do not want to dial forever, try to dial up to given deadline.
		deadline := p.connectDeadline(ctx)
		if !deadline.IsZero() && deadline.Before(time.Now()) {
			return nil, ErrTimeout
		}

		p.dialer.SetDeadlineAtLeast(deadline)
	}

	return p.dial()
}

// ResetDialThrottle resets the dial throttle timeout such that next dial
// attempt for a new connection will not be canceled due to throttled logic.
// That is, it works for pool which was configured with non-zero DialThrottle
// option.
func (p *Pool) ResetDialThrottle() {
	p.dialThrottle.Reset()
}

// SetDialThrottle sets up time point before which underlying dialer will fail
// its dial attempts.
func (p *Pool) SetDialThrottle(m time.Time) {
	p.dialThrottle.Set(m)
}

// CancelDial cancels current dial routine.
func (p *Pool) CancelDial() {
	p.dialer.Cancel()
}

// dial calls pool's dialer's Dial method with background context.
// It returns channel which fulfillment indicates that dial routine completed.
// On success it stores established connection by calling StoreConn().
// Note that if dial() called while previous routine is in progress, it returns
// the same channel as to the first call.
func (p *Pool) dial() (ch <-chan error, err error) {
	if !p.dialThrottle.Next() {
		p.logf("start to dial throttled")
		return nil, ErrThrottled
	}
	// NOTE: We use background context here to not affect clients in case when
	// first call is made with context A and second with B, in such way that
	// lifetime of A is shorter than B. This will sweep B's dial expectation
	// deadline up to A's. This is not good.
	return p.dialer.Dial(bg, func(conn net.Conn, err error) {
		if err != nil {
			p.logf("dial failed: %v", err)
			return
		}
		if err = p.StoreConn(conn); err != nil {
			p.logf(
				"store dialed connection %s error: %v",
				nameConn(conn), err,
			)
		}
	}), nil
}

// limitRate returns an error if request is not within pool rate limits
// or context error if ctx is done.
func (p *Pool) limitRate(ctx context.Context) error {
	if p.rateLimiter == nil {
		return nil
	}

	if p.rateWait {
		if err := p.rateLimiter.Wait(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			return &RateError{Err: err}
		}
	} else if !p.rateLimiter.Allow() {
		return &RateError{Err: ErrPolicied}
	}

	return nil
}

// NextChannel returns next alive channel.
// It could try to establish new connection and create new Channel.
// If establishing of new connection is failed, it tries to wait some time and repeat.
//
// Note that returned Channel could be closed while receiving it from the Pool.
// In this case additional NextChannel() may be made to get next alive channel.
func (p *Pool) NextChannel(ctx context.Context) (c *Channel, err error) {
	var (
		waitTm    = p.config.ConnectTimeout
		waitTimer *time.Timer
		dialed    <-chan error
		poolFull  bool
	)

	if err = p.limitRate(ctx); err != nil {
		return nil, err
	}

	for {
		p.mu.RLock()

		if p.stopped {
			p.mu.RUnlock()
			return nil, ErrStopped
		}

		online := p.online
		p.mu.RUnlock()

		if n := len(online); n > 0 {
			i := int(atomic.AddUint32(&p.next, 1))
			c = online[i%n]
			poolFull = n == p.config.Size
		}

		switch {
		case c != nil:
			if c.WriteClosed() {
				// Received already closed channel.
				p.removeChannel(c)
				c = nil

				continue
			}

			// NOTE: we do not check Channel load average here.
			//       To use some algorithm of making decision to dial for
			//       additional channel, we also should implement logic of
			//       out-of-use channel closure.
			if !poolFull {
				// Try to create new channel if there is a space for a new
				// channel and current channel is under high load (load
				// detection is not implemented yet).
				_, _ = p.DialBackground()
			}

			return

		case p.config.FailOnCutoff:
			// Initiate a background dial to prepare a new Channel for further
			// calls.
			_, _ = p.DialBackground()

			// Return ErrCutoff signaling that currently pool is offline and
			// can not prepare request. That is, client set up
			// FailOnCutoff to prevent blocking on dial when there are
			// no online Channels.
			return nil, ErrCutoff

		default:
			// Must call for a new channel.
			dialed, err = p.DialForeground(ctx)
			if err != nil {
				return
			}
		}

		if waitTimer == nil {
			// We use timer here instead of context.WithTimeout
			// because we could reuse timers and in some use cases
			// this is more efficient.
			waitTimer = egoTime.AcquireTimer(waitTm)
			defer egoTime.ReleaseTimer(waitTimer)
		}

		select {
		case <-dialed:
			continue

		case <-ctx.Done():
			err = ctx.Err()
		case <-waitTimer.C:
			err = ErrNoChannel
		}

		atomic.AddUint32(&p.stats.WaitErrors, 1)

		return
	}
}

// Online returns current list of online Channels.
func (p *Pool) Online() []*Channel {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.online
}

// Hijack resets a list of online channels.
// It calls callback cb for every ex-online channel with result of its Hijack()
// call. If Hijack returns error it does not call a callback.
//
// Note that it is a caller responsibility to manage ownership of the returned
// channels that could be shared across goroutines by NextChannel(), Call(),
// Notify() and Send() calls.
func (p *Pool) Hijack(cb func(conn net.Conn, rbuf []byte)) {
	for _, ch := range p.Online() {
		conn, rbuf, err := ch.Hijack()
		if err == nil {
			cb(conn, rbuf)
		}
	}
}

// ShutdownConnections resets a list of online channels.
// It calls Shutdown() on each of ex-online channel and returns when all calls
// are done.
func (p *Pool) ShutdownConnections() {
	shutdownChannels(p.Online())
}

// CloseConnections resets a list of online channels.
// It calls Close() on each of ex-online channel and returns when all calls are
// done.
func (p *Pool) CloseConnections() {
	closeChannels(p.Online())
}

// DropConnections resets a list of online channels.
// It calls Drop() on each of ex-online channel and returns when all calls are
// done.
func (p *Pool) DropConnections() {
	dropChannels(p.Online())
}

// StoreConn stores given connection inside the pool if there are space for it.
// Given ctx is used for Channel.Init() call.
// If err is non-nol given conn will be closed.
func (p *Pool) StoreConn(conn net.Conn) (err error) {
	ch := NewChannel(conn, p.config.ChannelConfig)

	var once sync.Once

	cb := func() {
		once.Do(func() {
			p.removeChannel(ch)
		})
	}

	ch.OnClose(cb)
	ch.OnShutdown(cb)

	if err = ch.Init(); err != nil {
		ch.Close()
		return err
	}

	p.mu.Lock()

	switch {
	case p.stopped:
		err = ErrStopped
	case len(p.online) == p.config.Size:
		err = ErrPoolFull
	default:
		p.online = append(p.online, ch)
	}

	p.mu.Unlock()

	if err != nil {
		ch.Close()
	}

	return err
}

func (p *Pool) removeChannel(ch *Channel) {
	var ok bool

	p.mu.Lock()
	// Read stopped flag to prevent redial below.
	stopped := p.stopped

	// Remove given channel from the online list.
	p.online, ok = removeChannel(p.online, ch)
	if ok {
		// If channel was really online, save it stats before forget.
		p.saveChannelStats(ch)
	}

	p.mu.Unlock()

	if !ok {
		return
	}

	p.debugf("removed channel %s > %s", ch.LocalAddr().String(), ch.RemoteAddr().String())

	if !p.config.DisablePreDial && !stopped {
		_, _ = p.DialBackground()
	}
}

func (p *Pool) connectDeadline(ctx context.Context) time.Time {
	t := time.Now().Add(p.config.ConnectTimeout)

	deadline, ok := ctx.Deadline()
	if !ok || t.Before(deadline) {
		deadline = t
	}

	return deadline
}

// mutex must be held
func (p *Pool) saveChannelsStats(chs []*Channel) {
	for _, ch := range chs {
		p.saveChannelStats(ch)
	}
}

// mutex must be held
func (p *Pool) saveChannelStats(ch *Channel) {
	p.stats.ChannelStats = p.stats.ChannelStats.Add(ch.Stats())
}

func (p *Pool) logf(s string, args ...interface{}) {
	p.config.Logger.Printf(bg, s, args...)
}

func (p *Pool) debugf(s string, args ...interface{}) {
	p.config.Logger.Debugf(bg, s, args...)
}

func closeChannels(list []*Channel)    { stopChannels(list, (*Channel).Close, true) }
func shutdownChannels(list []*Channel) { stopChannels(list, (*Channel).Shutdown, true) }
func dropChannels(list []*Channel)     { stopChannels(list, (*Channel).Drop, false) }

// stopChannels calls fn on each channel in separate goroutine.
// fn should be one of Channel's Drop, Close or Shutdown method.
// It blocks until all channels are Done() if await is true.
func stopChannels(list []*Channel, fn func(*Channel), await bool) {
	for _, c := range list {
		go fn(c)
	}

	if await {
		for _, c := range list {
			<-c.Done()
		}
	}
}

func removeChannel(list []*Channel, c *Channel) ([]*Channel, bool) {
	for i, e := range list {
		if e == c {
			ret := make([]*Channel, len(list)-1)
			copy(ret[:i], list[:i])
			copy(ret[i:], list[i+1:])

			return ret, true
		}
	}

	return list, false
}

type PoolStats struct {
	ChannelStats

	DialCount  uint32 // total number of net.Dial calls
	DialErrors uint32 // total number of failed net.Dial calls
	WaitErrors uint32 // total number of failed expectation for free channel

	Online int // number of online channels
}

type poolAddr struct {
	network string
	addr    string
}

func (p poolAddr) Network() string { return p.network }
func (p poolAddr) String() string  { return p.addr }

func setContextLogger(ctx context.Context, config *PoolConfig) *PoolConfig {
	if config != nil && config.Logger != nil {
		// Logger already set, nothing to do.
		return config
	}

	logger, ok := contextLogger(ctx)
	if !ok {
		// No log prefix for context.
		return config
	}

	cp := CopyPoolConfig(config)
	cp.Logger = logger

	// Accessing ChannelConfig.Logger directly is safe here, cause
	// CopyPoolConfig always set ChannelConfig.
	if cp.ChannelConfig.Logger == nil {
		cp.ChannelConfig.Logger = logger
	}

	return cp
}

func contextLogger(ctx context.Context) (Logger, bool) {
	c, ok := ctx.(ctxlog.Context)
	if !ok {
		return nil, false
	}

	return DefaultLogger{
		Prefix: c.LogPrefix(),
	}, true
}
