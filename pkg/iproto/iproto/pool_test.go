package iproto

import (
	"fmt"
	"log"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/context"
)

// TestPoolShareChannel expects that just established channel will be shared
// with Call() and other actors only when config.OnDial callback returns.
func TestPoolShareChannel(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)
	ln, _ := runEchoServer(t, stop)

	const (
		released = 1
		received = 2
	)
	var (
		order  = make(chan int, 2)
		onDial = make(chan struct{})
	)

	p := NewPool(ln.Addr().Network(), ln.Addr().String(), &PoolConfig{
		Size:           1,
		DisablePreDial: true,
		ChannelConfig: &ChannelConfig{
			Init: func(ctx context.Context, conn *Channel) error {
				<-onDial
				order <- released
				return nil
			},
		},
	})

	//nolint:staticcheck
	go func() {
		_, err := p.NextChannel(context.Background())
		if err != nil {
			//nolint:govet
			t.Fatal(err)
		}
		order <- received
	}()

	time.Sleep(time.Millisecond * 100)
	close(onDial)

	if first := <-order; first != released {
		t.Errorf("channel was shared after OnDial released")
	}
}

// TestPoolCallAfterDrop expects that caller of Call() method will not receive
// ErrStopped error even if pool has lost connections moment before.
func TestPoolCallAfterDrop(t *testing.T) {
	stop := make(chan struct{})
	defer close(stop)
	ln, _ := runEchoServer(t, stop)

	p := NewPool(ln.Addr().Network(), ln.Addr().String(), &PoolConfig{
		Size: 1,
	})
	if err := p.Init(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := p.Call(context.Background(), 42, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	closeChannels(p.online)
	if _, err := p.Call(context.Background(), 42, nil); err != nil {
		t.Fatalf("unexpected error after redial: %s", err)
	}
}

func TestPoolFailOnCutoff(t *testing.T) {
	accept := make(chan struct{}, 1)
	p := NewPool("stubnet", "stubaddr", &PoolConfig{
		Size:         1,
		FailOnCutoff: true,
		NetDial: func(_ context.Context, n string, a string) (net.Conn, error) {
			<-accept
			c, _ := net.Pipe()
			return c, nil
		},
	})
	accept <- struct{}{}
	if err := p.Init(bg); err != nil {
		t.Fatal(err)
	}

	ch, err := p.NextChannel(bg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ch.Close()

	if _, err := p.NextChannel(bg); err != ErrCutoff {
		t.Errorf("unexpected error: %v; want %v", err, ErrCutoff)
	}

	// Allow "server" to accept new connection.
	accept <- struct{}{}
	// Let new connection to be stored.
	time.Sleep(time.Millisecond)

	if _, err := p.NextChannel(bg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoolInit(t *testing.T) {
	p := NewPool("stubnet", "stubaddr", &PoolConfig{
		ConnectTimeout: time.Millisecond * 100,
		NetDial: func(ctx context.Context, n, a string) (net.Conn, error) {
			return nil, fmt.Errorf("whoa")
		},
	})
	init := make(chan error, 1)
	go func() {
		init <- p.Init(bg)
	}()

	select {
	case <-init:
		// Ok.
	case <-time.After(time.Second):
		t.Errorf("no result after 1s")
	}
}

func TestPoolStoreConn(t *testing.T) {
	p := NewPool("stubnet", "stubaddr", &PoolConfig{
		ConnectTimeout: time.Millisecond,
		NetDial: func(_ context.Context, _, _ string) (net.Conn, error) {
			return nil, fmt.Errorf("whoa")
		},
	})
	if _, err := p.NextChannel(bg); err == nil {
		t.Fatalf("unexpected nil error")
	}
	conn, _ := net.Pipe()
	if err := p.StoreConn(conn); err != nil {
		t.Fatalf("unexpected StoreConn() error: %v", err)
	}
	if _, err := p.NextChannel(bg); err != nil {
		t.Fatalf("unexpected NextChannel() error: %v", err)
	}
}

func TestPoolDialThrottle(t *testing.T) {
	dial := new(int32)
	throttleTime := 100 * time.Millisecond
	var wantDials int32 = 10
	p := NewPool("stubnet", "stubaddr", &PoolConfig{
		Size:           1,
		DialThrottle:   throttleTime,
		FailOnCutoff:   true,
		DisablePreDial: true,
		NetDial: func(_ context.Context, n string, a string) (net.Conn, error) {
			atomic.AddInt32(dial, 1)
			c, _ := net.Pipe()
			return c, nil
		},
	})
	timeo := time.After(throttleTime * time.Duration(wantDials))
	// For the timeo time we want wantDials retries to connect
THROTT_WAITLOOP:
	for {
		ch, err := p.NextChannel(bg)
		if err == nil {
			ch.Close()
		}
		select {
		case <-time.After(time.Millisecond * 10):
		case <-timeo:
			break THROTT_WAITLOOP
		}
	}
	if act := atomic.LoadInt32(dial); act != wantDials {
		t.Fatalf("dial count is %d; want %d", act, wantDials)
	}
}

func TestPoolShutdown(t *testing.T) {
	rcvShutdown := new(int32)
	n := new(int32)
	p := NewPool("stubnet", "S", &PoolConfig{
		Size: 4,
		NetDial: func(_ context.Context, _, _ string) (net.Conn, error) {
			c, s := net.Pipe()
			client := "C" + strconv.Itoa(int(atomic.AddInt32(n, 1)))
			c = withAddr(c, client, "S")
			s = withAddr(s, "S", client)

			// Run server channel. We use it to calculate number of received
			// shutdown messages.
			ch, err := RunChannel(s, nil)
			if err != nil {
				t.Fatal(err)
			}
			ch.OnShutdown(func() {
				atomic.AddInt32(rcvShutdown, 1)
			})

			return c, nil
		},
	})
	err := p.InitAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	p.Shutdown()

	if act, exp := len(p.Online()), 0; act != exp {
		t.Fatalf("online is %d; want %d", act, exp)
	}
	if act, exp := int(atomic.LoadInt32(rcvShutdown)), 4; act != exp {
		t.Fatalf("server recevied shutdown from %d channels; want %d", act, exp)
	}

}

func TestPoolPeerShutdown(t *testing.T) {
	// sem used to limit successful redial attempts.
	sem := make(chan struct{}, 1)
	// srv used to retreive "accepted" connections.
	srv := make(chan net.Conn, 1)
	n := new(int32)
	p := NewPool("stubnet", "S", &PoolConfig{
		Size:           1,
		DisablePreDial: true,
		NetDial: func(_ context.Context, _, _ string) (net.Conn, error) {
			select {
			case sem <- struct{}{}:
				c, s := net.Pipe()
				client := "C" + strconv.Itoa(int(atomic.AddInt32(n, 1)))
				c = withAddr(c, client, "S")
				s = withAddr(s, "S", client)
				srv <- s
				return c, nil
			default:
				return nil, fmt.Errorf("stub dialer whoa")
			}
		},
	})

	err := p.InitAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	ch, err := RunChannel(<-srv, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Send shutdown message to the pool's channel.
	// We expect that pool will remove it from online list.
	ch.Shutdown()

	if act, exp := len(p.Online()), 0; act != exp {
		t.Fatalf("online is %d; want %d", act, exp)
	}

	p.Shutdown()
}

func TestPoolClose(t *testing.T) {
	ln := getListener(t)
	defer ln.Close()
	addr := ln.Addr().String()

	p := NewPool("tcp", addr, &PoolConfig{
		Size: 4,
	})
	err := p.InitAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	p.Close()
	for _, c := range p.online {
		if !c.Closed() {
			t.Fatal("expected every channel to be closed")
		}
	}
}

func TestPoolStats(t *testing.T) {
	t.Skip("Doesn't work on linux")
	stopAnswer := make(chan struct{})
	ln, done := runEchoServer(t, stopAnswer)
	defer ln.Close()
	addr := ln.Addr().String()

	p := NewPool("tcp", addr, &PoolConfig{
		Size:           1,
		ConnectTimeout: time.Millisecond * 10,
		RedialInterval: time.Millisecond * 15,
		DisablePreDial: true,
		ChannelConfig: &ChannelConfig{
			DisablePing:    true,
			RequestTimeout: time.Millisecond * 100,
			NoticeTimeout:  time.Millisecond * 100,
		},
	})

	data := []byte("hello")
	var exp PoolStats
	for i := 0; i < 1000; i++ {
		exp.DialCount++

		if err := p.Notify(context.Background(), 24, data); err != nil {
			t.Fatal(err)
		}
		exp.ChannelStats.BytesSent += headerLen + uint32(len(data))
		exp.ChannelStats.PacketsSent++
		exp.ChannelStats.NoticesCount++

		if _, err := p.Call(context.Background(), 42, nil); err != nil {
			t.Fatal(err)
		}
		exp.ChannelStats.BytesSent += headerLen
		exp.ChannelStats.BytesReceived += headerLen
		exp.ChannelStats.PacketsSent++
		exp.ChannelStats.PacketsReceived++
		exp.ChannelStats.CallCount++

		// Force dial for new channel on next iteration.
		closeChannels(p.online)
	}

	close(stopAnswer)

	_, err := p.Call(context.Background(), 42, nil)
	if err != ErrTimeout {
		t.Fatalf("want timeout error, got %s", err)
	}
	exp.DialCount++
	exp.ChannelStats.BytesSent += headerLen
	exp.ChannelStats.PacketsSent++
	exp.ChannelStats.CallCount++
	exp.ChannelStats.CanceledCount++

	// Force dial for new channel on next call.
	closeChannels(p.online)

	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	<-done

	if err := p.Notify(context.Background(), 42, data); err == nil {
		t.Errorf("expected error on dial to closed listener; got nothing")
	}
	exp.DialCount++
	exp.DialErrors++
	exp.WaitErrors++

	p.Shutdown()

	stats := p.Stats()
	if !reflect.DeepEqual(stats, exp) {
		t.Errorf("p.Stats():\n\tact:  %#v\n\twant: %#v", stats, exp)
	}
}

func TestPoolFullSize(t *testing.T) {
	ln := getListener(t)
	defer ln.Close()
	addr := ln.Addr().String()

	p := NewPool("tcp", addr, &PoolConfig{
		Size: 4,
	})
	for i := 0; i < 4+2; i++ { // get all channels and do not return them back
		if _, err := p.NextChannel(context.Background()); err != nil {
			t.Fatalf("p.NextChannel() error: %s", err)
		}

		// emulate channel overload
		//ch.stats.ReaderWorkTime = 100
		//ch.stats.WriterWorkTime = 100

		time.Sleep(time.Millisecond * 10) // wait until next connection is established
	}
	if s := p.Stats(); s.Online != 4 { // expecting to get no free channel error
		t.Errorf("pool.online is %d; want 4", s.Online)
	}
}

func TestPoolReconnect(t *testing.T) {
	ln := getListener(t)
	defer ln.Close()
	addr := ln.Addr().String()

	p := NewPool("tcp", addr, &PoolConfig{
		Size:           1,
		DisablePreDial: true,
	})

	ch1, err := p.NextChannel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	ch1.Close()

	ch2, err := p.NextChannel(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if ch1 == ch2 {
		t.Fatal("expected second p.get() returns a different channel")
	}
}

func TestPoolReconnectTimeout(t *testing.T) {
	t.Skip("Fails on centos (unstable)")
	for i, test := range []struct {
		connectTimeout time.Duration
		redialInterval time.Duration
		contextTimeout time.Duration
		expAttempts    int
	}{
		// Simple case when we expecting that after connectTimeout
		// pool will stop trying to reconnect because of caller is
		// going away.
		{
			redialInterval: time.Millisecond * 110,
			connectTimeout: time.Millisecond * 500,
			contextTimeout: time.Millisecond * 500,
			expAttempts:    5,
		},
	} {
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			attempts := new(int32)
			p := NewPool("tcp", "<stub-addr>", &PoolConfig{
				ConnectTimeout: test.connectTimeout,
				RedialInterval: test.redialInterval,
				NetDial:        stubNetDial(attempts, 0),
			})

			ctx := bg
			if test.contextTimeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, test.contextTimeout)
				defer cancel()
			}

			err := p.Notify(ctx, 42, nil)
			if err == nil {
				t.Errorf("want error; got nil")
			} else {
				log.Printf("okay; error: %s", err)
			}

			if n := int(atomic.LoadInt32(attempts)); n != test.expAttempts {
				t.Errorf("unexpected dial attempts: %d; want %d", n, test.expAttempts)
			}
		})
	}
}

func TestPoolReconnectCancel(t *testing.T) {
	t.Skip("Doesn't work on linux: unexpected dial attempts: 6; want 5")
	attempts := new(int32)
	p := NewPool("tcp", "not existing addr", &PoolConfig{
		Size:           1,
		DisablePreDial: true,

		RedialInterval:    time.Millisecond * 11,
		MaxRedialInterval: time.Millisecond * 11,

		NetDial: stubNetDial(attempts, 0),
	})

	ctx1, _ := context.WithTimeout(context.Background(), time.Millisecond*25)
	ctx2, _ := context.WithTimeout(context.Background(), time.Millisecond*55)

	//nolint:errcheck
	go p.Notify(ctx1, 42, nil)

	//nolint:errcheck
	go p.Notify(ctx2, 42, nil)

	<-ctx2.Done()
	time.Sleep(time.Millisecond * 15) // Give pool a chance to dial again.
	if n := int(atomic.LoadInt32(attempts)); n != 5 {
		t.Errorf("unexpected dial attempts: %d; want %d", n, 5)
	}
}

func TestPoolConcurrentReconnect(t *testing.T) {
	ln := getListener(t)
	defer ln.Close()
	addr := ln.Addr().String()

	p := NewPool("tcp", addr, &PoolConfig{
		Size:           1,
		DisablePreDial: true,
	})

	wg := &sync.WaitGroup{}

	grNumber := 2
	errs := make([]error, grNumber)

	for i := 0; i < grNumber; i++ {
		wg.Add(1)
		go func(grIndex int) {
			defer wg.Done()
			_, errs[grIndex] = p.NextChannel(context.Background())
		}(i)
	}

	wg.Wait()

	for _, e := range errs {
		if e != nil {
			t.Error("reconnect result for concurrent goroutines must be successful")
		}
	}
}

func stubNetDial(attempts *int32, delay time.Duration) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		atomic.AddInt32(attempts, 1)
		tm := time.NewTimer(delay)
		select {
		case <-tm.C:
			return nil, fmt.Errorf("could not dial =(")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func BenchmarkPoolNextChannel(b *testing.B) {
	ln := getListener(b)
	defer ln.Close()
	addr := ln.Addr().String()

	p := NewPool("tcp", addr, &PoolConfig{
		Size:           4,
		ConnectTimeout: time.Second,
	})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := p.NextChannel(context.Background())
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

type breaker interface {
	Fatal(...interface{})
}

func getListener(t breaker) (ln net.Listener) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				if !strings.Contains(err.Error(), "use of closed network connection") {
					t.Fatal(err)
				}
				return
			}
		}
	}()
	return
}

func runServer(t breaker, h Handler) (ln net.Listener, done chan struct{}) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &Server{
		ChannelConfig: &ChannelConfig{
			DisablePing: true,
			Handler:     ParallelHandler(h, 128),
		},
	}
	done = make(chan struct{})
	go func() {
		defer close(done)

		err := srv.Serve(context.Background(), ln)
		if err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				t.Fatal(err)
			}
			return
		}
	}()
	return
}

func runEchoServer(t breaker, stop chan struct{}) (ln net.Listener, done chan struct{}) {
	return runServer(t, HandlerFunc(func(_ context.Context, c Conn, p Packet) {
		select {
		case <-stop:
			return
		default:
			if p.Header.Sync == 0 {
				return
			}
			_ = c.Send(bg, ResponseTo(p, nil))
		}
	}))
}

type addrconn struct {
	net.Conn
	local  net.Addr
	remote net.Addr
}

func (a addrconn) RemoteAddr() net.Addr {
	return a.remote
}

func (a addrconn) LocalAddr() net.Addr {
	return a.local
}

func withAddr(c net.Conn, local, remote string) net.Conn {
	return addrconn{c, strAddr(local), strAddr(remote)}
}
