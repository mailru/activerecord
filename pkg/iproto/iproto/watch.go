package iproto

import (
	"sync"

	"golang.org/x/net/context"
)

// PoolChange represents pool state change.
type PoolChange struct {
	// Pool is a source of changes.
	Pool *Pool

	// Channel points to channel, that was opened/closed.
	Channel *Channel

	// Conn contains difference of connection count in pool.
	Conn int8
}

// PoolChangeHandle helps to control PoolChange notifications flow.
type PoolChangeHandle struct {
	stop chan struct{}
}

func NewPoolChangeHandle() *PoolChangeHandle {
	return &PoolChangeHandle{
		stop: make(chan struct{}),
	}
}

func (p *PoolChangeHandle) Stop() {
	close(p.stop)
}

func (p *PoolChangeHandle) Stopped() <-chan struct{} {
	return p.stop
}

// NotifyPoolChange makes every given pool to relay its changes into ch.
//
// It will block sending to ch: the caller must ensure that ch has sufficient
// buffer space to not block the pool.
//
// Caller should pass not initialized pool. That is, caller should not make
// iproto.Dial and then WatchPoolConnCount. The right way to do this is to make
// NewPool, followed by WatchPoolConnCount, optionally followed by pool.Init.
//
// Note that if some pool config.ChannelConfig.Init returns error, given ch
// will not be notified about the established connection nor the closure of it.
//
// It returns PoolChangeHandle that could be used to control notifications sent
// to ch.
func NotifyPoolChange(ch chan<- PoolChange, pools ...*Pool) *PoolChangeHandle {
	h := NewPoolChangeHandle()
	NotifyPoolChangeWith(h, ch, pools...)

	return h
}

// NotifyPoolChangeWith is the same as NotifyPoolChange except that it uses
// already existing PoolChangeHandle.
//
// This could be useful to stop notifications after multiple NotifyPoolChange*()
// calls by a single call to h.Stop().
func NotifyPoolChangeWith(h *PoolChangeHandle, ch chan<- PoolChange, pools ...*Pool) {
	for _, p := range pools {
		pool := p // For closure.

		// Need to copy *ChannelConfig to prevent overwriting user data at
		// passed pointer.
		cfg := CopyChannelConfig(p.config.ChannelConfig)
		p.config.ChannelConfig = cfg

		userInit := cfg.Init
		cfg.Init = func(ctx context.Context, conn *Channel) error {
			if userInit != nil {
				if err := userInit(ctx, conn); err != nil {
					return err
				}
			}

			notify := func(n int8) {
				select {
				case <-h.Stopped():
				default:
					ch <- PoolChange{pool, conn, n}
				}
			}

			notify(1)
			conn.OnClose(func() {
				notify(-1)
			})

			return nil
		}
	}
}

// WatchPoolConnCount is a helper function that makes channel to receive pool
// changes, starts a goroutine to accumulate that changes and calls cb with
// total number of connections at every change.
//
// Caller should pass not initialized pool. That is caller should not make
// iproto.Dial and then WatchPoolConnCount. The right way to do this is to make
// NewPool, followed by WatchPoolConnCount, optionally followed by pool.Init.
// In other way the value passed to cb will contain number of connections which
// are established/closed only after WatchPoolConnCount call, not the all
// connections number.
//
// Note that if pool config.ChannelConfig.Init returns error, callback will not
// be called about the established connection nor the closure of it.
func WatchPoolConnCount(p *Pool, cb func(int)) {
	ch := make(chan PoolChange, 1)
	h := NotifyPoolChange(ch, p)

	p.OnClose(func() {
		h.Stop()
		close(ch)
	})

	go func() {
		n := 0
		for change := range ch {
			n += int(change.Conn)
			cb(n)
		}
	}()
}

// MultiWatcher allows to watch on the state of the multiple pools.
//
// The initial state of watcher is always detached.
type MultiWatcher struct {
	// MinAlivePeers specifies a number of alive peers, less than which
	// we consider the cluster to be detached.
	//
	// If MinAlivePeers is zero, then even an empty list of peers means that
	// cluster is attached.
	MinAlivePeers int

	// StrictGraceful expects only graceful connections closure. Non-graceful
	// closure of the connection leads to OnDetached() call.
	// After non-graceful peer reestablished connection, OnAttached will be
	// called.
	StrictGraceful bool

	// OnDetached is called when there are no enough alive peers or some peers
	// are broken or uninitialized.
	OnDetached func()

	// OnAttached is called when cluster get enough alive peers with no any
	// broken or uninitialized peer.
	OnAttached func()

	// OnStateChange is called when some peer from cluster changes its state.
	// It receives peer itself, number of its alive connections and the
	// sign of the broken state (e.g. when closed not gracefully). The last
	// argument is the total number of alive peers in the cluster.
	OnStateChange func(peer *Pool, alive int, broken bool, alivePeers int)

	// Logger contains logger interface.
	Logger Logger

	once sync.Once

	change        chan PoolChange
	init          chan watcherSignal
	shutdown      chan watcherSignal
	remove        chan watcherSignal
	initialized   chan watcherSignal
	uninitialized chan watcherSignal
	stop          chan watcherSignal

	handle *PoolChangeHandle
}

func (w *MultiWatcher) Init() {
	w.once.Do(func() {
		// TODO(s.kamardin): maybe guess more accurate buffer size for
		//					 these channels?
		w.change = make(chan PoolChange, 1)
		w.handle = NotifyPoolChange(w.change)

		// NOTE: Prepare non-buffered channels to avoid deadlocks on
		// signalAndWait() calls on stopped watcher.
		w.init = make(chan watcherSignal)
		w.shutdown = make(chan watcherSignal)
		w.remove = make(chan watcherSignal)
		w.initialized = make(chan watcherSignal)
		w.uninitialized = make(chan watcherSignal)
		w.stop = make(chan watcherSignal)

		go w.watcher()
	})
}

func (w *MultiWatcher) Stop() {
	w.signalAndWait(w.stop, nil)
}

func (w *MultiWatcher) Watch(p *Pool) {
	// Need to copy *ChannelConfig to prevent overwriting user data at
	// passed pointer.
	cfg := CopyChannelConfig(p.config.ChannelConfig)
	p.config.ChannelConfig = cfg

	userInit := cfg.Init
	cfg.Init = func(ctx context.Context, ch *Channel) error {
		ch.OnShutdown(func() {
			w.signalAndWait(w.shutdown, p)
		})

		if userInit != nil {
			return userInit(ctx, ch)
		}

		return nil
	}

	w.signalAndWait(w.init, p)
	NotifyPoolChangeWith(w.handle, w.change, p)
}

// MarkNotInitialized marks given peer as not initialized yet.
// That is, cluster will become detached if there are any not initialized
// members. Call to MarkNotInitialized() suggests further MarkInitialized()
// call.
func (w *MultiWatcher) MarkNotInitialized(p *Pool) {
	w.signalAndWait(w.uninitialized, p)
}

// MarkInitialized marsk given peer as initialized.
// It makes sense to call MarkInitialized() only when MarkNotInitialized() was
// called before.
func (w *MultiWatcher) MarkInitialized(p *Pool) {
	w.signalAndWait(w.initialized, p)
}

// Remove removes any data related to pool p. Note that it may lead watcher to
// constantly detached state, if some channel of p was closed gracelessly. That
// is, caller must call p.Shutdown() before calling w.Remove(p).
func (w *MultiWatcher) Remove(p *Pool) {
	w.signalAndWait(w.remove, p)
}

func (w *MultiWatcher) signalAndWait(dest chan watcherSignal, p *Pool) {
	processed := make(chan struct{})
	select {
	case dest <- watcherSignal{p, processed}:
	case <-w.handle.Stopped():
	}
	select {
	case <-processed:
	case <-w.handle.Stopped():
	}
}

func (w *MultiWatcher) logf(f string, args ...interface{}) {
	const prefix = "iproto: cluster watcher: "

	logger := w.Logger
	if logger == nil {
		logger = DefaultLogger{
			Prefix: prefix,
		}
	} else {
		logger = prefixLogger{
			logger: logger,
			prefix: prefix,
		}
	}

	logger.Printf(bg, f, args...)
}

//nolint:gocognit,gocyclo
func (w *MultiWatcher) watcher() {
	var (
		ready         = make(map[*Pool]bool)
		alive         = make(map[*Pool]int)
		graceful      = make(map[*Pool]int)
		broken        = make(map[*Pool]bool)
		uninitialized = make(map[*Pool]bool)

		attached = w.MinAlivePeers == 0

		peerAlive       = 0
		peerBroken      = 0
		peerUnitialized = 0

		onDetached = func() {
			if w.OnDetached != nil {
				w.OnDetached()
			}
		}
		onAttached = func() {
			if w.OnAttached != nil {
				w.OnAttached()
			}
		}
		onStateChange = func(peer *Pool, alive int, broken bool, total int) {
			w.logf(
				"state changed for %s: alive=%d; broken=%t",
				peer.RemoteAddr(), alive, broken,
			)
			if w.OnStateChange != nil {
				w.OnStateChange(peer, alive, broken, total)
			}
		}
	)

	if attached {
		onAttached()
	}

	handleChange := func(change PoolChange) {
		pool := change.Pool
		if !ready[pool] {
			return
		}

		var (
			increment bool
			decrement bool
		)

		switch change.Conn {
		case 1:
			alive[pool]++

			increment = alive[pool] == 1
			if increment {
				peerAlive++
				w.logf("alive peer count increased to %d", peerAlive)
			}

			if broken[pool] {
				broken[pool] = false
				peerBroken--
			}
		case -1:
			alive[pool]--

			decrement = alive[pool] == 0
			if decrement {
				peerAlive--
				w.logf("alive peer count decreased to %d", peerAlive)
			}

			if n := graceful[pool] - 1; n < 0 {
				if !broken[pool] {
					broken[pool] = true
					peerBroken++
				}
			} else {
				graceful[pool] = n
			}
		}

		onStateChange(pool, alive[pool], broken[pool], peerAlive)

		switch {
		case decrement:
			// Do nothing if we already in "detached" state.
			if !attached {
				return
			}

			switch {
			case w.StrictGraceful && broken[pool]:
				w.logf(
					"cluster become detached: %s was not closed gracefully",
					pool.RemoteAddr().String(),
				)
			case peerAlive < w.MinAlivePeers:
				w.logf(
					"cluster become detached: %d alive peers; want at least %d",
					peerAlive, w.MinAlivePeers,
				)
			default:
				// If pool connection was closed gracefully and number of
				// alive peers is ok, do not change state to "detached".
				return
			}

			attached = false

			onDetached()
		case increment:
			// Do nothing if we already in "attached" state.
			if attached {
				return
			}

			// Do nothing if alive peers is less than expected number to
			// think that we "attached".
			if peerAlive < w.MinAlivePeers {
				return
			}

			// Do not send "attached" message if some peer is still has no
			// alive connections and some of its connections were not closed
			// gracefully.
			if w.StrictGraceful && peerBroken > 0 {
				return
			}

			// Do not send "attached" message if some peer is still uninitialized.
			if peerUnitialized > 0 {
				return
			}

			w.logf(
				"cluster become attached: %d alive peers; want at least %d",
				peerAlive, w.MinAlivePeers,
			)

			attached = true

			onAttached()
		}
	}

	handleShutdown := func(s watcherSignal) {
		pool := s.peer
		// Increase number of expected closures for pool.
		//
		// This number is used when pool connection is closed and we should
		// decide what is this situation. If number of expected closures is
		// greater than zero, then we should not call "detached" callback,
		// because this is graceful shutdown. That is, graceful closure
		// could not lead to missed calls or notices.
		graceful[pool]++
		w.logf(
			"%s marked as shutting down: graceful=%d",
			pool.RemoteAddr().String(), graceful[pool],
		)
		close(s.done)
	}

	handleInit := func(s watcherSignal) {
		pool := s.peer
		if !ready[pool] {
			ready[pool] = true

			w.logf(
				"watching peer %s",
				pool.RemoteAddr().String(),
			)
		}

		close(s.done)
	}
	handleRemove := func(s watcherSignal) {
		defer close(s.done)

		pool := s.peer
		if !ready[pool] {
			return
		}

		if alive[pool] > 0 {
			peerAlive--
		}

		if broken[pool] {
			peerBroken--
		}

		if uninitialized[pool] {
			peerUnitialized--
		}

		delete(ready, pool)
		delete(alive, pool)
		delete(graceful, pool)
		delete(broken, pool)
		delete(uninitialized, pool)

		w.logf("removed peer %s", pool.RemoteAddr().String())

		switch {
		case !attached:
			if peerAlive < w.MinAlivePeers {
				return
			}

			if w.StrictGraceful && peerBroken > 0 {
				return
			}

			if peerUnitialized > 0 {
				return
			}

			w.logf("cluster become attached", peerBroken)

			attached = true

			onAttached()

		case attached:
			if peerAlive >= w.MinAlivePeers {
				return
			}

			w.logf(
				"cluster become detached: %d alive peers; want at least %d",
				peerAlive, w.MinAlivePeers,
			)

			attached = false

			onDetached()
		}
	}
	handleUninitialized := func(s watcherSignal) {
		defer close(s.done)

		pool := s.peer
		if !ready[pool] || uninitialized[pool] {
			return
		}

		w.logf("%s marked as uninitialized", pool.RemoteAddr())

		uninitialized[pool] = true
		peerUnitialized++

		if attached {
			w.logf("cluster become detached")

			attached = false

			onDetached()
		}
	}

	handleInitialized := func(s watcherSignal) {
		defer close(s.done)

		pool := s.peer
		if !uninitialized[pool] {
			return
		}

		w.logf("%s marked as initialized", pool.RemoteAddr())

		uninitialized[pool] = false
		peerUnitialized--

		if peerUnitialized > 0 {
			return
		}

		if w.StrictGraceful && peerBroken > 0 {
			return
		}

		if peerAlive < w.MinAlivePeers {
			return
		}

		w.logf("cluster become attached")

		attached = true

		onAttached()
	}

	handleStop := func(s watcherSignal) {
		w.logf("stopped watching all peers")
		w.handle.Stop()
		close(s.done)
	}

	for {
		select {
		case s := <-w.init:
			handleInit(s)
		case c := <-w.change:
			handleChange(c)
		case s := <-w.shutdown:
			handleShutdown(s)
		case s := <-w.remove:
			handleRemove(s)
		case s := <-w.uninitialized:
			handleUninitialized(s)
		case s := <-w.initialized:
			handleInitialized(s)
		case s := <-w.stop:
			handleStop(s)
			return
		}
	}
}

type watcherSignal struct {
	peer *Pool
	done chan<- struct{}
}
