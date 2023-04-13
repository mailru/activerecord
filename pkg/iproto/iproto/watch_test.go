package iproto

import (
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mailru/activerecord/pkg/iproto/syncutil"

	"golang.org/x/net/context"
)

func Broadcast(ctx context.Context, c *Cluster, method uint32, data []byte) {
	peers := c.Peers()
	syncutil.Each(bg, len(peers), func(_ context.Context, i int) {
		_, _ = peers[i].Call(bg, method, data)
	})
}

func TestPoolStartStopNotifyChange(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)
	ln, _ := runEchoServer(t, stop)

	p := NewPool(ln.Addr().Network(), ln.Addr().String(), &PoolConfig{
		Size: 2,
	})

	ch := make(chan PoolChange)
	handle := NotifyPoolChange(ch, p)

	calls := new(uint32)
	go func() {
		for range ch {
			atomic.AddUint32(calls, 1)
		}
	}()

	// Trigger first change notice.
	dialed, err := p.DialForeground(bg)
	if err != nil {
		t.Fatal(err)
	}
	<-dialed

	// Ignore further notifications.
	handle.Stop()

	// Trigger second change notice.
	dialed, err = p.DialForeground(bg)
	if err != nil {
		t.Fatal(err)
	}
	<-dialed

	if act := atomic.LoadUint32(calls); act != 1 {
		t.Errorf("unexpected number of notifications: got %v; want %v", act, 1)
	}
}

// TestPoolNotifyChangeRestoreErr expects that notice will be sent only when
// OnDial callback does not return error.
func TestPoolNotifyChangeRestoreErr(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)
	ln, _ := runEchoServer(t, stop)

	i := new(int32)
	p := NewPool(ln.Addr().Network(), ln.Addr().String(), &PoolConfig{
		Size:           4,
		DisablePreDial: true,
		ChannelConfig: &ChannelConfig{
			Init: func(ctx context.Context, conn *Channel) error {
				// Return error only on odd calls.
				if atomic.AddInt32(i, 1)%2 == 0 {
					return nil
				}
				return io.EOF
			},
		},
	})

	ch := make(chan PoolChange)
	done := make(chan struct{})
	NotifyPoolChange(ch, p)
	var conns int
	var calls int
	go func() {
		defer close(done)
		for change := range ch {
			calls++
			conns += int(change.Conn)
		}
	}()

	for i := 0; i < 4; i++ {
		dialed, err := p.DialForeground(bg)
		if err != nil {
			t.Fatal(err)
		}
		<-dialed
	}

	close(ch)
	<-done

	if exp := 2; calls != exp {
		t.Errorf("actual calls with state change is %d; want %d", calls, exp)
	}
	if exp := 2; conns != exp {
		t.Errorf("actual state is %d connections; want %d", conns, exp)
	}
}

func TestMultiWatcherHandleRemoval(t *testing.T) {
	for _, test := range []struct {
		n        int
		minAlive int
		strict   bool
		close    []int
		remove   []int
		exp      []int
	}{
		{
			n:        2,
			minAlive: 1,
			strict:   true,
			close:    []int{0},
			remove:   []int{0},
			exp:      []int{1, 0, 1},
		},
		{
			n:        3,
			minAlive: 1,
			strict:   true,
			close:    []int{0, 1},
			remove:   []int{0, 1},
			exp:      []int{1, 0, 1},
		},
		{
			n:        2,
			minAlive: 2,
			remove:   []int{0},
			exp:      []int{1, 0},
		},
		{
			n:        4,
			minAlive: 2,
			remove:   []int{0, 1},
			exp:      []int{1},
		},
		{
			n:        4,
			minAlive: 2,
			remove:   []int{0, 1, 2, 3},
			exp:      []int{1, 0},
		},
	} {
		t.Run("", func(t *testing.T) {
			timeline := make(chan int, len(test.exp)*2)

			w := MultiWatcher{
				MinAlivePeers:  test.minAlive,
				StrictGraceful: test.strict,
				OnDetached: func() {
					log.Printf("[test] cluster become detached")
					timeline <- 0
				},
				OnAttached: func() {
					log.Printf("[test] cluster become attached")
					timeline <- 1
				},
			}
			w.Init()

			var (
				client = make([]net.Conn, test.n)
				server = make([]net.Conn, test.n)
				pool   = make([]*Pool, test.n)
			)
			for i := range pool {
				pool[i] = NewPool("", "", &PoolConfig{
					DisablePreDial: true,
					NetDial: func(_ context.Context, _, _ string) (net.Conn, error) {
						panic("dial prohibited")
					},
				})
				pool[i].network = "stub"
				pool[i].addr = "#" + strconv.Itoa(i)

				w.Watch(pool[i])

				client[i], server[i] = net.Pipe()
				if err := pool[i].StoreConn(client[i]); err != nil {
					t.Fatal(err)
				}
			}

			// Emulate non-graceful close: should become detached.
			for _, i := range test.close {
				server[i].Close()
			}

			// Let closure to be handled.
			time.Sleep(100 * time.Millisecond)

			// Remove broken pool: should become attached.
			for _, i := range test.remove {
				w.Remove(pool[i])
			}

			for i, exp := range test.exp {
				select {
				case act := <-timeline:
					if act != exp {
						t.Errorf(
							"unexpected #%d state: %v; want %v",
							i, act, exp,
						)
					}
				case <-time.After(100 * time.Millisecond):
					t.Errorf("no #%d event during 100ms", 1)
				}
			}
		})
	}
}

func TestMultiWatcherHandleShutdown(t *testing.T) {
	for _, test := range []struct {
		label           string
		peers           int
		minAlive        int
		onDetached      int
		onAttached      int
		strictGraceful  bool
		gracefulClosure bool
	}{
		{
			peers:           4,
			minAlive:        0,
			onDetached:      0,
			onAttached:      1,
			strictGraceful:  true,
			gracefulClosure: true,
		},
		{
			minAlive:        1,
			peers:           2,
			onDetached:      1,
			onAttached:      1,
			strictGraceful:  true,
			gracefulClosure: false,
		},
		{
			minAlive:        2,
			peers:           4,
			onDetached:      1,
			onAttached:      1,
			strictGraceful:  true,
			gracefulClosure: true,
		},
		{
			peers:           1,
			minAlive:        1,
			onDetached:      1,
			onAttached:      1,
			strictGraceful:  true,
			gracefulClosure: true,
		},
	} {
		t.Run(test.label, func(t *testing.T) {
			var (
				mu                 sync.Mutex
				rebusServerClients []*Channel
			)

			var (
				connected = make([]sync.WaitGroup, test.peers)
				once      = make([]sync.Once, test.peers)
			)
			for i := range connected {
				connected[i].Add(1)
			}
			// Create N rebus servers.
			peers, err := getPeers(test.peers, nil,
				func(i int, ch *Channel) {
					defer once[i].Do(func() {
						connected[i].Done()
					})

					mu.Lock()
					rebusServerClients = append(rebusServerClients, ch)
					mu.Unlock()
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			var (
				attached int
				detached int
			)
			w := MultiWatcher{
				MinAlivePeers:  test.minAlive,
				StrictGraceful: test.strictGraceful,
				OnDetached: func() {
					log.Printf("[test] cluster become detached")
					mu.Lock()
					detached++
					mu.Unlock()
				},
				OnAttached: func() {
					log.Printf("[test] cluster become attached")
					mu.Lock()
					attached++
					mu.Unlock()
				},
			}
			w.Init()

			// Initiate dialing.
			for _, addr := range peers {
				p := NewPool("tcp", addr, &PoolConfig{
					DisablePreDial: true,
					ChannelConfig: &ChannelConfig{
						DisablePing: true,
						Logger:      devnullLogger{},
					},
					Logger: devnullLogger{},
				})
				w.Watch(p)
				if err := p.Init(bg); err != nil {
					t.Fatal(err)
				}
			}
			// Wait for all peers receive client connection.
			for i := range connected {
				connected[i].Wait()
			}
			time.Sleep(10 * time.Millisecond)

			log.Printf("[test] terminating rebus servers")
			log.Printf("[test] -------------------------")

			mu.Lock()
			clients := rebusServerClients
			mu.Unlock()
			// Emulate here rebus server graceful restart.
			for _, ch := range clients {
				log.Printf(
					"[test] terminating client %s > %s (graceful=%t)",
					ch.LocalAddr().String(), ch.RemoteAddr().String(), test.gracefulClosure,
				)
				if test.gracefulClosure {
					ch.Shutdown()
				} else {
					ch.Close()
				}
			}

			// Give a chance to callbacks be called.
			time.Sleep(100 * time.Millisecond)

			mu.Lock()

			if act, exp := attached, test.onAttached; act != exp {
				t.Errorf("OnAttached called %d; want %d", act, exp)
			}
			if act, exp := detached, test.onDetached; act != exp {
				t.Errorf("OnDetached called %d; want %d", act, exp)
			}
		})
	}
}

func TestMultiWatcherAttachedDetached(t *testing.T) {
	t.Skip("unstable")
	for _, n := range []int{1, 2, 8} {
		name := fmt.Sprintf("%d peers", n)
		t.Run(name, func(t *testing.T) {
			connected := make(chan *Channel, n)
			// Create N rebus servers.
			peers, err := getPeers(n, nil,
				func(_ int, ch *Channel) {
					connected <- ch
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			var (
				xmu       sync.Mutex
				incx      []int
				decx      []int
				attached  int
				detached  int
				peerAlive int
			)
			type expect struct {
				decrement int
				increment int
				attached  int
				detached  int
			}
			check := func(prefix string, exp expect) {
				xmu.Lock()
				defer xmu.Unlock()

				if exp, act := exp.decrement, len(decx); act != exp {
					t.Errorf(
						"%sdecremented alive peers %d times: %v; want %d",
						prefix, act, decx, exp,
					)
				}
				if exp, act := exp.increment, len(incx); act != exp {
					t.Errorf(
						"%sincremented alive peers %d times: %v; want %d",
						prefix, act, incx, exp,
					)
				}
				if exp, act := exp.attached, attached; act != exp {
					t.Errorf(
						"%sOnAttached() called %d times; want %d",
						prefix, act, exp,
					)
				}
				if exp, act := exp.detached, detached; act != exp {
					t.Errorf(
						"%sOnDetached() called %d times; want %d",
						prefix, act, exp,
					)
				}

				log.Printf(
					"state: increment: %v; decrement: %v; attached: %d; detached: %d",
					incx, decx, attached, detached,
				)

				decx = nil
				incx = nil
				attached = 0
				detached = 0
			}

			w := MultiWatcher{
				MinAlivePeers: 1,

				StrictGraceful: true,

				OnDetached: func() {
					xmu.Lock()
					detached++
					xmu.Unlock()
					log.Printf("[test] cluster become detached")
				},
				OnAttached: func() {
					xmu.Lock()
					attached++
					xmu.Unlock()
					log.Printf("[test] cluster become attached")
				},
				OnStateChange: func(p *Pool, alive int, broken bool, n int) {
					xmu.Lock()
					defer xmu.Unlock()

					if peerAlive == n {
						return
					}
					peerAlive = n

					switch alive {
					case 1:
						// Alive peer count increased.
						incx = append(incx, n)
					case 0:
						// Alive peer count decreased.
						decx = append(decx, n)
					}
				},
			}
			w.Init()

			c := new(Cluster)

			for _, addr := range peers {
				p := NewPool("tcp", addr, &PoolConfig{
					ChannelConfig: &ChannelConfig{
						RequestTimeout:  time.Hour,
						ShutdownTimeout: time.Second,
					},
					DisablePreDial: true,
				})

				w.Watch(p)

				if err := p.Init(bg); err != nil {
					t.Fatal(err)
				}
				c.Insert(p)
			}

			// Wait until cluster establish connection to all peers.
			// Remember the any single peer for further close.
			var peer *Channel
			for i := 0; i < n; i++ {
				peer = <-connected
			}

			<-time.After(10 * time.Millisecond)
			check("dial: ", expect{
				decrement: 0,
				increment: n,
				attached:  1,
				detached:  0,
			})

			w.MarkNotInitialized(c.Peers()[0])
			<-time.After(10 * time.Millisecond)
			check("not initialized: ", expect{
				decrement: 0,
				increment: 0,
				attached:  0,
				detached:  1,
			})

			w.MarkInitialized(c.Peers()[0])
			<-time.After(10 * time.Millisecond)
			check("initialized: ", expect{
				decrement: 0,
				increment: 0,
				attached:  1,
				detached:  0,
			})

			// Make "forever"-pending call to each peer, blocking their Shutdown().
			//
			// We will use this to prevent removing previously saved peer from
			// cluster after receiving shutdown message from him below.
			// That is, we want to reach the state, when cluster has 2 "alive"
			// connections for one peer.
			ctx, cancel := context.WithCancel(context.Background())
			go Broadcast(ctx, c, 47, nil)

			// Send the shutdown message from one peer to initiate cluster to shutdown it.
			// Subsequent Call() on cluster will initiate redial to this peer.
			// Until we cancel requests from above, we must get 2 "alive"
			// connection for this single peer.
			<-time.After(10 * time.Millisecond)
			go peer.Shutdown()

			// Let cluster handle shutdown message.
			<-time.After(10 * time.Millisecond)
			// Initiate cluster to reestablish connection for one that shutting
			// down.
			go Broadcast(bg, c, 47, nil)

			// Wait for shutdown to be completed.
			<-peer.Done()

			// Wait for reconnection complete.
			peer = <-connected

			// Cancel first part of request, leting cluster to shutdown peer
			// that sent shutdown message.
			// After that, we will get 1 alive connection, but must not
			// increase alive peers counter.
			<-time.After(10 * time.Millisecond)
			cancel()

			<-time.After(10 * time.Millisecond)
			check("peer shutdown: ", expect{
				decrement: 0,
				increment: 0,
				attached:  0,
				detached:  0,
			})

			// Now close the peer non-gracefully. This must lead to detached state.
			peer.Close()
			<-time.After(10 * time.Millisecond)
			check("peer close: ", expect{
				decrement: 1,
				increment: 0,
				attached:  0,
				detached:  1,
			})

			// Initiate cluster to reestablish connection for one that shutting
			// down.
			go Broadcast(bg, c, 47, nil)

			<-time.After(10 * time.Millisecond)

			check("peer close: ", expect{
				decrement: 0,
				increment: 1,
				attached:  1,
				detached:  0,
			})

			// Remove every peer from watcher.
			// We expect OnDetached to be called because number of alive peers
			// will become less than MinAlivePeers (which is 1 in test).
			for _, pool := range c.Peers() {
				w.Remove(pool)
			}
			check("after remove: ", expect{
				decrement: 0,
				increment: 0,
				attached:  0,
				detached:  1,
			})

			// Now close the cluster, expecting that increment or decrement
			// will not made, neither OnDetached or OnAttached will called.
			c.Close()
			<-time.After(10 * time.Millisecond)
			check("after close: ", expect{
				decrement: 0,
				increment: 0,
				attached:  0,
				detached:  0,
			})
		})
	}
}

func TestMultiWatcherStop(t *testing.T) {
	m := new(MultiWatcher)

	stateChanges := new(int32)
	m.OnStateChange = func(_ *Pool, _ int, _ bool, _ int) {
		atomic.AddInt32(stateChanges, 1)
	}

	m.Init()

	p := NewPool("", "", &PoolConfig{
		Size: 2,
		NetDial: func(_ context.Context, _, _ string) (net.Conn, error) {
			t.Fatalf("unexpected dial")
			return nil, nil
		},
	})

	m.Watch(p)

	connA, connB := net.Pipe()

	if err := p.StoreConn(connA); err != nil {
		t.Fatal(err)
	}

	m.Stop()

	if err := p.StoreConn(connB); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(stateChanges); n != 1 {
		t.Errorf("unexpected number of state changes: %v; want 1", n)
	}
}
