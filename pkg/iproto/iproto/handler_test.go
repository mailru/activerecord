package iproto_test

import (
	"testing"
	"time"

	"github.com/mailru/activerecord/pkg/iproto/iproto"
	"github.com/mailru/activerecord/pkg/iproto/iproto/internal/testutil"
	"github.com/mailru/activerecord/pkg/iproto/util/pool"
	"golang.org/x/net/context"
)

func handler(ctx context.Context, rw iproto.Conn, pkt iproto.Packet) {
	_ = rw.Send(ctx, iproto.ResponseTo(pkt, nil))
}

func benchmarkHandler(b *testing.B, h iproto.Handler) {
	var p iproto.Packet
	rw := testutil.NewFakeResponseWriter()
	rw.DoSend = func(context.Context, iproto.Packet) error {
		// emulate some work
		time.Sleep(time.Microsecond)
		return nil
	}
	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		h.ServeIProto(ctx, rw, p)
	}
}

func BenchmarkHandlerPlain(b *testing.B) {
	benchmarkHandler(b, iproto.HandlerFunc(handler))
}

func BenchmarkHandlerParallel(b *testing.B) {
	benchmarkHandler(b, iproto.ParallelHandler(iproto.HandlerFunc(handler), 128))
}

func BenchmarkHandlerPool(b *testing.B) {
	p := pool.Must(pool.New(&pool.Config{
		UnstoppableWorkers: 128,
		MaxWorkers:         128,
		WorkQueueSize:      100,
	}))
	benchmarkHandler(b, iproto.PoolHandler(iproto.HandlerFunc(handler), p))
}
