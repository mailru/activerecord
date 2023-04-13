package iproto

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/context"
)

// TestChannelShutdownTwice tests that second call to Shutdown() will return
// the same moment when first returns.
func TestChannelShutdownTwice(t *testing.T) {
	server, client := net.Pipe()
	go func() {
		_, _ = io.Copy(io.Discard, server)
	}()

	ch := NewChannel(client, &ChannelConfig{
		ShutdownTimeout: time.Second,
	})
	_ = ch.Init()

	timeline := make(chan time.Time, 2)
	go func() {
		ch.Shutdown()
		timeline <- time.Now()
	}()
	go func() {
		ch.Shutdown()
		timeline <- time.Now()
	}()

	a := <-timeline
	b := <-timeline

	diff := a.Sub(b).Nanoseconds()
	if act, max := time.Duration(int64abs(diff)), 100*time.Microsecond; act > max {
		t.Errorf(
			"difference between calls is %v; want at most %v",
			act, max,
		)
	}
}

func int64abs(v int64) int64 {
	m := v >> 63 // Get all 111..111 for negative or 000..000 for positive;
	v = v ^ m    // (NOT v) for negative and the same for positive;
	v -= m       // +1 is for negative (hence m is all 111..111, which is -1 bit pattern), -0 for positive;
	return v
}

func TestChannelOnCloseAfterRun(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:")
	if err != nil {
		t.Fatal(err)
	}

	closed := make(chan struct{}, 1)
	srv := &Server{
		ChannelConfig: &ChannelConfig{
			Init: func(ctx context.Context, ch *Channel) error {
				// Emulate some runtime delay before setting OnClose callback. This
				// lets channel to be closed before callback registered.
				time.Sleep(time.Millisecond)

				ch.OnClose(func() {
					closed <- struct{}{}
				})

				return nil
			},
		},
	}

	//nolint:errcheck
	go srv.Serve(context.Background(), ln)

	conn, err := net.Dial(ln.Addr().Network(), ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()

	select {
	case <-closed:
	case <-time.After(time.Millisecond * 100):
		t.Errorf("OnClose was not called after conn is closed")
	}
}

func TestChannelCall(t *testing.T) {
	type testCase struct {
		method uint32
		resp   []byte
		err    bool
	}
	var wg sync.WaitGroup
	for i, test := range []testCase{
		{
			method: 42,
			resp:   []byte("OK"),
		},
	} {
		wg.Add(1)
		//nolint:staticcheck
		//nolint:govet
		go func(i int, test testCase) {
			client, server, err := getClientServerConns()
			if err != nil {
				//nolint:staticcheck,govet
				t.Fatal(err)
			}

			defer wg.Done()
			defer client.Close()
			defer server.Close()

			//nolint:errcheck
			go RunChannel(server, &ChannelConfig{
				Handler: HandlerFunc(func(ctx context.Context, w Conn, p Packet) {
					if err = w.Send(bg, ResponseTo(p, test.resp)); err != nil {
						//nolint:govet
						t.Fatal(err)
					}
				}),
				Logger: devnullLogger{},
			})

			channel, err := RunChannel(client, &ChannelConfig{
				WriteQueueSize:  DefaultWriteQueueSize,
				ShutdownTimeout: time.Millisecond,
				SizeLimit:       DefaultSizeLimit,
				PingInterval:    0,
				Logger:          devnullLogger{},
			})
			if err != nil {
				//nolint:govet
				t.Fatal(err)
			}

			data, err := channel.Call(context.Background(), test.method, nil)
			if err != nil && !test.err {
				t.Errorf("[%d] unexpected error: %s", i, err)
			}
			if err == nil && test.err {
				t.Errorf("[%d] want error got nil", i)
			}
			if !reflect.DeepEqual(data, test.resp) {
				t.Errorf("[%d] Call() = %v; want %v", i, data, test.resp)
			}
		}(i, test)
	}
	wg.Wait()
}

func TestChannelClose(t *testing.T) {
	client, server, err := getClientServerConns()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer server.Close()

	channel, err := RunChannel(client, &ChannelConfig{
		WriteQueueSize:  DefaultWriteQueueSize,
		ShutdownTimeout: time.Millisecond,
		SizeLimit:       DefaultSizeLimit,
		PingInterval:    0,
		Logger:          devnullLogger{},
	})
	if err != nil {
		t.Fatal(err)
	}

	channel.Close()

	_, err = channel.Call(context.Background(), 42, nil)
	if err != ErrStopped {
		t.Errorf("_, err := Call(); err is %v; want %v", err, io.EOF)
	}
}

func TestChannelShutdown(t *testing.T) {
	client, server, err := getClientServerConns()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer server.Close()

	// read all input messages to prevent socket locks
	go devnull(server)

	channel, err := RunChannel(client, &ChannelConfig{
		WriteQueueSize:  DefaultWriteQueueSize,
		ShutdownTimeout: time.Millisecond,
		SizeLimit:       DefaultSizeLimit,
		PingInterval:    0,
		Logger:          devnullLogger{},
	})
	if err != nil {
		t.Fatal(err)
	}

	var (
		received int32
		sent     int32
		wgEnter  sync.WaitGroup
		wgLeave  sync.WaitGroup
	)

	for sent = 0; sent < 50000; sent++ {
		wgEnter.Add(1)
		wgLeave.Add(1)

		go func() {
			defer func() {
				if err := recover(); err != nil {
					//nolint:govet
					t.Fatal(err)
				}
			}()
			wgEnter.Done()
			{
				_, _ = channel.Call(context.Background(), 42, []byte("Привет, Мир!"))
				atomic.AddInt32(&received, 1)
			}
			wgLeave.Done()
		}()
	}

	wgEnter.Wait()
	channel.Shutdown()

	wgLeave.Wait()
	<-channel.Done()

	if s, r := atomic.LoadInt32(&sent), atomic.LoadInt32(&received); s != r {
		t.Errorf("shutdown is not so graceful: sent %d; received %d", s, r)
	}
}

func TestChannelShutdownCloseMessage(t *testing.T) {
	for _, test := range []struct {
		call bool
		resp bool
		exp  int32
	}{
		{false, false, 1},
		{true, false, 0},
		{true, true, 1},
	} {
		t.Run("", func(t *testing.T) {
			c, s, err := getClientServerConns()
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			defer s.Close()

			client, err := RunChannel(c, &ChannelConfig{
				Logger:          DefaultLogger{"client: "},
				RequestTimeout:  time.Hour,
				ShutdownTimeout: 100 * time.Millisecond,
			})
			if err != nil {
				t.Fatal(err)
			}
			server, err := RunChannel(s, &ChannelConfig{
				Logger: DefaultLogger{"server: "},
			})
			if err != nil {
				t.Fatal(err)
			}

			order := make(chan string, 2)
			called := new(int32)
			server.OnShutdown(func() {
				order <- "server"
				atomic.AddInt32(called, 1)
			})
			client.OnShutdown(func() {
				order <- "client"
			})

			if test.call {
				//nolint:errcheck
				go client.Call(bg, 42, nil)
				// Let call be queued.
				time.Sleep(5 * time.Millisecond)
			}
			if test.resp {
				err := server.Send(bg, ResponseTo(Packet{Header{Msg: 42, Sync: 1}, nil}, nil))
				if err != nil {
					t.Fatal(err)
				}
			}

			go client.Shutdown()
			<-server.Done()
			<-client.Done()

			if n := atomic.LoadInt32(called); n != test.exp {
				t.Fatalf("OnShutdown called %d times; want %d", n, test.exp)
			}
			if test.exp == 0 {
				return
			}
			if a, b := <-order, <-order; a != "client" || b != "server" {
				t.Errorf("OnShutdown called in wrong order: %q then %q", a, b)
			}
		})
	}
}

func TestChannelPing(t *testing.T) {
	client, server, err := getClientServerConns()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer server.Close()

	s, err := RunChannel(server, &ChannelConfig{
		PingInterval: time.Millisecond * 500,
		IdleTimeout:  time.Second,
		Logger:       devnullLogger{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	c, err := RunChannel(client, &ChannelConfig{
		PingInterval: time.Millisecond * 500,
		IdleTimeout:  time.Second,
		Logger:       devnullLogger{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	select {
	case <-c.Done():
		t.Fatalf("client channel done: %s", c.Error())
	case <-s.Done():
		t.Fatalf("server channel done: %s", s.Error())
	case <-time.After(time.Second * 5):
	}
}

func TestChannelIdleTimeout(t *testing.T) {
	client, server, err := getClientServerConns()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer server.Close()

	now := time.Now()
	c, _ := RunChannel(client, &ChannelConfig{
		DisablePing: true,
		IdleTimeout: 10 * time.Millisecond,
	})
	defer c.Close()

	select {
	case <-c.Done():
		expErr := fmt.Sprintf("inactivity for %s", c.config.IdleTimeout)
		if !strings.Contains(c.Error().Error(), expErr) {
			t.Fatalf("channel closed with %q cause; want %q", c.Error(), expErr)
		}
	case tm := <-time.After(time.Millisecond * 100):
		t.Fatalf("idle timer did not closed the channel after %s (idle timeout is %s)", tm.Sub(now), c.config.IdleTimeout)
	}
}

func TestChannelCallTimeout(t *testing.T) {
	client, server, err := getClientServerConns()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer server.Close()

	c, err := RunChannel(client, &ChannelConfig{
		DisablePing:    true,
		IdleTimeout:    time.Minute,
		RequestTimeout: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	result := make(chan error)
	now := time.Now()
	go func() {
		_, err := c.Call(context.Background(), 42, nil)
		result <- err
	}()

	select {
	case err := <-result:
		if err != ErrTimeout {
			t.Fatalf("Call() error is %v; want %v", err, ErrTimeout)
		}
	case tm := <-time.After(time.Millisecond * 10):
		t.Fatalf("Call() did not failed after %s (request timeout is %s)", tm.Sub(now), c.config.RequestTimeout)
	}
}

func TestChannelCallContextDone(t *testing.T) {
	client, server, err := getClientServerConns()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer server.Close()

	c, err := RunChannel(client, &ChannelConfig{
		DisablePing:    true,
		IdleTimeout:    time.Minute,
		RequestTimeout: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	result := make(chan error)
	ctx, _ := context.WithTimeout(context.Background(), time.Millisecond)
	now := time.Now()
	go func() {
		_, err := c.Call(ctx, 42, nil)
		result <- err
	}()

	select {
	case err := <-result:
		if err != context.DeadlineExceeded {
			t.Fatalf("Call() error is %v; want %v", err, context.DeadlineExceeded)
		}
	case tm := <-time.After(time.Millisecond * 10):
		t.Fatalf("Call() did not failed after %s (request timeout is %s)", tm.Sub(now), c.config.RequestTimeout)
	}
}

func TestChannelReaderEOF(t *testing.T) {
	client, server, err := getClientServerConns()
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer server.Close()

	c, err := RunChannel(client, &ChannelConfig{
		DisablePing:    true,
		IdleTimeout:    time.Minute,
		RequestTimeout: time.Minute,
		Logger:         devnullLogger{},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	err = server.(*net.TCPConn).CloseWrite()
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.Call(context.Background(), 42, nil)
	if err == nil {
		t.Fatalf("want error, got nil")
	}
}

func TestChannelFinalizer(t *testing.T) {
	_, c := net.Pipe()
	ch, err := RunChannel(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	ch.OnClose(func() {
		log.Printf("onClose called: %p", ch)
	})
	ch.OnShutdown(func() {
		t.Errorf("onShutdown called: %p", ch)
	})

	collected := make(chan struct{})
	runtime.SetFinalizer(ch, func(ch *Channel) {
		close(collected)
	})

	ch.Close()

	timeout := time.After(time.Second)
	for {
		select {
		case <-collected:
			return
		case <-timeout:
			t.Fatalf("finalizer was not called")
		default:
			runtime.GC()
			runtime.Gosched()
		}
	}
}

func TestChannelFinalizerShutdown(t *testing.T) {
	s, c := net.Pipe()
	ch, err := RunChannel(c, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Emulate some references to ch.
	mp := map[*Channel]bool{ch: true}

	collected := make(chan struct{})
	runtime.SetFinalizer(ch, func(ch *Channel) {
		log.Printf("Finalizer called: %p", ch)
		close(collected)
	})
	ch.OnShutdown(func() {
		log.Printf("OnShutdown called: %p", ch)
		delete(mp, ch)
	})

	if err := WritePacket(s, shutdownPacket); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPacket(s); err != nil {
		t.Fatal(err)
	}
	s.Close()

	timeout := time.After(time.Second * 60)
	for {
		select {
		case <-collected:
			return
		case <-timeout:
			t.Fatalf("OnShutdown() does not return")
		default:
			runtime.GC()
			runtime.Gosched()
		}
	}
}

type delayReader struct {
	net.Conn

	delay time.Duration
	times int32
}

func (d *delayReader) Read(p []byte) (int, error) {
	if atomic.AddInt32(&d.times, -1) >= 0 {
		time.Sleep(d.delay)
	}
	return d.Conn.Read(p)
}

// TestChannelDelayedPingInfiniteLoop tests that channel handles pings correct
// when one peer busy.
//
// That is, this test expect channel prevents infinite ping "loop" between peers:
// Say you have two peers A and B, A do not send pings to B, B pings
// A for every 1s. If A freeze for 2.1s, it will receive 2 pings, and
// could send response pings immediately. B will receive two pings:
// first will be interpreted as an "ping-ack", but second – as a ping
// from A. Then, B will immediately send its "ping-ack" to A, and A
// will immediately respond on it and so on.
func TestChannelDelayedPingInfiniteLoop(t *testing.T) {
	client, server, err := getClientServerConns()
	if err != nil {
		t.Fatal(err)
	}

	// Prepare reader, that receives 2 first ping acks glued.
	r := &delayReader{client, 21 * time.Millisecond, 1}
	c, err := RunChannel(r, &ChannelConfig{
		PingInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Prepare server that do not send pings.
	s, err := RunChannel(server, &ChannelConfig{
		DisablePing: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)
	c.Close()
	s.Close()

	if n := c.Stats().NoticesCount; n > 5 {
		t.Errorf("client sent %d notices; want at most 5", n)
	}
}

func TestChannelRunCallbackSerialization(t *testing.T) {
	const (
		cbInit = iota
		cbClose
	)

	client, server, err := getClientServerConns()
	if err != nil {
		t.Fatal(err)
	}

	q := make(chan int, 2)

	ch := NewChannel(client, &ChannelConfig{
		Init: func(ctx context.Context, ch *Channel) error {
			// Give channel's reader goroutine ability to read EOF from closed
			// connection and call OnClose callbacks.
			time.Sleep(time.Millisecond)

			q <- cbInit

			return nil
		},
	})
	ch.OnClose(func() {
		q <- cbClose
	})

	client.Close()
	server.Close()

	if err := ch.Init(); err != ErrStopped {
		t.Fatalf("want ErrStopped; got %v", err)
	}

	a, b := <-q, <-q
	if a != cbInit || b != cbClose {
		t.Errorf(
			"expected first action to be inside Init() call (%v), and second OnClose() callback (%v); got %v, %v",
			cbInit, cbClose, a, b,
		)
	}
}

func TestChannelUDP(t *testing.T) {
	serv, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer serv.Close()

	recv := make(chan bytesWithAddr, 3)
	go func() {
		buf := make([]byte, 4096)

		for {
			n, addr, errRead := serv.ReadFrom(buf)
			if errRead != nil {
				return
			}
			t.Logf(
				"server received %d bytes from %q: %v (err is %v)",
				n, addr, buf[:n], errRead,
			)
			recv <- bytesWithAddr{
				append(([]byte)(nil), buf[:n]...), addr,
			}
		}
	}()

	pool, err := Dial(bg, "udp", serv.LocalAddr().String(), &PoolConfig{
		ChannelConfig: &ChannelConfig{
			WriteBufferSize: 18, // 12 bytes for 1.5 empty packets; need to test packet split.
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	ch, err := pool.NextChannel(bg)
	if err != nil {
		t.Fatal(err)
	}

	// First, send two packets for check split cases.
	_ = ch.Notify(bg, 1, nil)
	_ = ch.Notify(bg, 1, nil)
	_ = ch.Notify(bg, 1, nil)

	time.Sleep(time.Millisecond)
	// Second, send some packet with payload.
	_ = ch.Notify(bg, 42, []byte("ping"))

	for {
		select {
		case ba := <-recv:
			if act, exp := ba.addr.String(), ch.LocalAddr().String(); act != exp {
				t.Fatalf("received from unexpected addr: %q; want %q", act, exp)
			}
			r := bytes.NewReader(ba.bytes)
			for r.Len() > 0 {
				p, err := ReadPacket(r)
				if err != nil {
					t.Fatalf("can not parse packet: %v", err)
				}
				if p.Header.Msg == 42 {
					if act, exp := string(p.Data), "ping"; act != exp {
						t.Fatalf("unexpected data: %q; want %q", act, exp)
					}
					return
				}
			}
		case <-time.After(time.Second):
			t.Fatalf("udp server did not receive any packet after 1s")
		}
	}
}

func getClientServerConns() (client, server net.Conn, err error) {
	var ln net.Listener
	ln, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return
	}

	srv := make(chan struct{})
	go func() {
		defer ln.Close()
		server, _ = ln.Accept()
		close(srv)
	}()

	client, err = net.Dial("tcp", ln.Addr().String())
	<-srv

	return
}

func devnull(c net.Conn) {
	for {
		_, err := io.Copy(io.Discard, c)
		if err != nil {
			break
		}
	}
}

type devnullLogger struct{}

func (devnullLogger) Printf(context.Context, string, ...interface{}) {}
func (devnullLogger) Debugf(context.Context, string, ...interface{}) {}
