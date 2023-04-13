package iproto

import (
	"context"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	network = "tcp"
	address = "localhost:0"
)

func echoServer(done chan struct{}, b *testing.B, l net.Listener, size int64) {
	conn, err := l.Accept()
	if err != nil {
		return
	}

	defer func() {
		close(done)
		conn.Close()
	}()
	if _, err := io.CopyN(conn, conn, size); err != nil {
		b.Fatalf("CopyN() error: %s", err)
	}
}

func devNullServer(done chan struct{}, b *testing.B, l net.Listener, size int64) {
	conn, err := l.Accept()
	if err != nil {
		return
	}
	defer func() {
		close(done)
		conn.Close()
	}()
	if _, err := io.CopyN(io.Discard, conn, size); err != nil {
		b.Fatalf("CopyN() error: %s", err)
	}
}

func startServer(b *testing.B, size int64, server func(chan struct{}, *testing.B, net.Listener, int64)) (chan struct{}, net.Listener) {
	l, err := net.Listen(network, address)
	if err != nil {
		b.Fatalf("Listen() error: %v", err)
	}

	done := make(chan struct{})
	go server(done, b, l, size)
	return done, l
}

func benchmarkCall(b *testing.B, parallelism, size int) {
	b.StopTimer()
	log.SetOutput(io.Discard)

	data := []byte(strings.Repeat("x", size))
	length := len(data) + headerLen

	_, ln := startServer(b, int64(length*b.N), echoServer)
	defer ln.Close()

	c, err := Dial(context.Background(), network, ln.Addr().String(), &PoolConfig{
		DialTimeout: time.Minute,
		ChannelConfig: &ChannelConfig{
			RequestTimeout: time.Minute,
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	defer c.Close()

	b.SetBytes(int64(length) * 2)
	b.ReportAllocs()
	b.ResetTimer()

	var (
		work = make(chan struct{})
		do   = struct{}{}
	)
	var wg sync.WaitGroup
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		//nolint:staticcheck,govet
		go func() {
			defer wg.Done()
			for range work {
				_, err := c.Call(context.Background(), 42, data)
				if err != nil {
					//nolint:staticcheck,govet
					b.Fatal(err)
				}
			}
		}()
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		work <- do
	}

	close(work)
	wg.Wait()
}

func BenchmarkCall_128_50b(b *testing.B)  { benchmarkCall(b, 128, 50) }
func BenchmarkCall_128_250b(b *testing.B) { benchmarkCall(b, 128, 250) }
func BenchmarkCall_1_50b(b *testing.B)    { benchmarkCall(b, 1, 50) }
func BenchmarkCall_1_250b(b *testing.B)   { benchmarkCall(b, 1, 250) }

func BenchmarkNotify(b *testing.B) {
	for _, size := range []int{
		50,
		100,
		200,
		500,
	} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			b.StopTimer()
			log.SetOutput(io.Discard)

			data := []byte(strings.Repeat("x", size))
			length := len(data) + headerLen

			done, ln := startServer(b, int64(length*b.N), devNullServer)
			defer ln.Close()

			c, err := Dial(context.Background(), network, ln.Addr().String(), &PoolConfig{
				DialTimeout: time.Minute,
				ChannelConfig: &ChannelConfig{
					RequestTimeout: time.Minute,
				},
			})
			if err != nil {
				b.Fatal(err)
			}
			defer c.Close()

			b.SetBytes(int64(length))
			b.ReportAllocs()
			b.StartTimer()

			for i := 0; i < b.N; i++ {
				err := c.Notify(context.Background(), 42, data)
				if err != nil {
					b.Fatal(err)
				}
			}
			<-done
		})
	}
}
