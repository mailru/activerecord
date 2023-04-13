package netutil

import (
	"fmt"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"
)

// TestDialerDialLimits expects that Dialer will not reach limits and intervals
// of given configuration.
func TestDialerDialLimits(t *testing.T) {
	t.Skip("Doesn't work on linux ")
	srv := &server{}

	d := &Dialer{
		Network: "tcp",
		Addr:    "localhost:80",
		Logf:    t.Logf,

		NetDial: srv.dial,

		LoopInterval:    time.Millisecond * 25,
		MaxLoopInterval: time.Millisecond * 100,
		LoopTimeout:     time.Millisecond * 500,
	}

	precision := float64(time.Millisecond * 10)

	var (
		expCalls    int
		expInterval []time.Duration
		growTime    time.Duration
		step        time.Duration
	)

	for step < d.MaxLoopInterval {
		expCalls++
		growTime += step
		expInterval = append(expInterval, step)
		step += d.LoopInterval
	}

	expCalls += int((d.LoopTimeout - growTime) / d.MaxLoopInterval)

	_, _ = d.Dial(context.Background())

	if n := len(srv.calls); n != expCalls {
		t.Errorf("unexpected dial calls: %v; want %v", n, expCalls)
	}

	for i, c := range srv.calls {
		var exp time.Duration
		if i < len(expInterval) {
			exp = expInterval[i]
		} else {
			exp = d.MaxLoopInterval
		}
		if act := c.delay; act != exp && math.Abs(float64(act-exp)) > precision {
			t.Errorf("unexpected %dth attempt delay: %s; want %s", i, act, exp)
		}
	}
}

//nolint:unused
type dialCall struct {
	time          time.Time
	delay         time.Duration
	network, addr string
}

//nolint:unused
type server struct {
	mu    sync.Mutex
	calls []dialCall
}

//nolint:unused
func (s *server) dial(ctx context.Context, n, a string) (net.Conn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	var delay time.Duration
	if n := len(s.calls); n > 0 {
		delay = now.Sub(s.calls[n-1].time)
	}
	s.calls = append(s.calls, dialCall{now, delay, n, a})

	return nil, fmt.Errorf("noop")
}
