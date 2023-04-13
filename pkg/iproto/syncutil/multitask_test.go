package syncutil

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mailru/activerecord/pkg/iproto/util/pool"
	"golang.org/x/net/context"
)

func TestMultitask(t *testing.T) {
	for _, test := range []struct {
		label       string
		goer        func(context.Context, func()) error
		n           int
		cancelOnErr bool
		partial     bool
		delay       time.Duration
		exp         map[error]int
	}{
		{
			label: "simple",
			n:     8,
			exp: map[error]int{
				nil: 8,
			},
		},
		{
			// This case tests possible deadlock situation, when number of
			// workers is less than number of parallel requests.
			label: "pool",
			goer:  getPoolGoer(1),
			n:     8,
			delay: 100 * time.Millisecond,
			exp: map[error]int{
				nil: 8,
			},
		},
		{
			label:   "cancelation",
			n:       8,
			delay:   100 * time.Millisecond,
			partial: true,

			goer: getCancelGoer(4), // This goer will return error after 4 calls.
			exp: map[error]int{
				nil:             4,
				ErrGoerCanceled: 4,
			},
		},
		{
			label:       "cancelation",
			n:           8,
			cancelOnErr: true,
			delay:       500 * time.Millisecond,

			goer: getCancelGoer(4), // This goer will return error after 4 calls.
			exp: map[error]int{
				context.Canceled: 4,
				ErrGoerCanceled:  4,
			},
		},
	} {
		t.Run(test.label, func(t *testing.T) {
			actors := make(map[int]func(context.Context) error, test.n)
			for i := 0; i < test.n; i++ {
				actors[i] = func(ctx context.Context) error {
					select {
					case <-time.After(test.delay):
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			}

			m := Multitask{
				Goer:            test.goer,
				ContinueOnError: test.partial,
			}

			var mu sync.Mutex
			act := map[error]int{}
			rem := test.n

			err := m.Do(context.Background(), test.n, func(ctx context.Context, i int) bool {
				err := actors[i](ctx)
				mu.Lock()
				act[err]++
				rem--
				mu.Unlock()

				if test.cancelOnErr && err != nil {
					return false
				}

				return true
			})

			if err != nil {
				act[err] = rem
			}

			//nolint:deepequalerrors
			if exp := test.exp; !reflect.DeepEqual(act, exp) {
				t.Fatalf("unexpected errors count: %v; want %v", act, exp)
			}
		})
	}
}

func BenchmarkMultitask(b *testing.B) {
	m := Multitask{
		Goer: getPoolGoer(1024),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.Do(context.Background(), 1, func(_ context.Context, i int) bool {
			return true
		})
	}
}

func getPoolGoer(n int) func(context.Context, func()) error {
	p, err := pool.New(&pool.Config{
		UnstoppableWorkers: n,
		MaxWorkers:         n,
	})
	if err != nil {
		panic(err)
	}
	return func(ctx context.Context, task func()) error {
		return p.ScheduleContext(ctx, pool.TaskFunc(task))
	}
}

var ErrGoerCanceled = fmt.Errorf("goer could not process task: limit exceeded")

func getCancelGoer(after int) func(context.Context, func()) error {
	n := new(int32)
	return func(_ context.Context, task func()) error {
		if count := atomic.AddInt32(n, 1); int(count) > after {
			return ErrGoerCanceled
		}
		go task()
		return nil
	}
}
