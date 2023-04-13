package iproto

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"
)

func TestListenDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan struct{})

	//nolint:staticcheck
	go func() {
		srv := &Server{ChannelConfig: &ChannelConfig{
			Handler: HandlerFunc(func(ctx context.Context, c Conn, p Packet) {
				var in uint32
				err = UnpackUint32(bytes.NewReader(p.Data), &in, 0)
				if err != nil {
					//nolint:govet
					t.Fatal(err)
					return
				}

				_ = c.Send(bg, ResponseTo(p, PackUint32(nil, in*2, 0)))
			}),
		}}

		err = srv.Serve(context.Background(), ln)

		select {
		case <-done:
			// test is complete it is okay
		default:
			//nolint:govet
			t.Fatal(err)
		}
	}()

	pool, err := Dial(context.Background(), "tcp", ln.Addr().String(), &PoolConfig{
		Size:           4,
		RedialInterval: time.Second * 1000,
		ConnectTimeout: time.Second * 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)

		//nolint:staticcheck
		go func() {
			defer wg.Done()
			var i uint32
			for i = 0; i < 1024; i++ {
				resp, err := pool.Call(context.Background(), uint32(i), PackUint32(nil, i, 0))
				if err != nil {
					//nolint:govet
					t.Fatal(err)
				}

				var r uint32
				err = UnpackUint32(bytes.NewReader(resp), &r, 0)
				if err != nil {
					//nolint:govet
					t.Fatal(err)
				}

				if r != i*2 {
					//nolint:govet
					t.Fatalf("pool.Call(%v) = %v; want %v", i, r, i*2)
				}
			}
		}()
	}
	wg.Wait()
	close(done)
}
