package syncutil

import (
	"sync"

	"golang.org/x/net/context"
)

// TaskGroup helps to control execution flow of repeatable tasks.
// It is intended to execute at most N tasks at one time.
type TaskGroup struct {
	// N is a maximum number of tasks TaskGroup can allow to execute.
	// If N is zero then TaskGroup with value 1 is used by default.
	N int

	// Goer starts a goroutine which executes a given task.
	// It is useful when client using some pool of goroutines.
	//
	// If nil, then default `go` is used and context is ignored.
	//
	// Non-nil error from Goer means that some resources are temporarily
	// unavailable and given task will not be executed.
	//
	// Note that for goroutine pool implementations it is required for pool to
	// have at least capacity of N goroutines. In other way deadlock may occur.
	Goer GoerFn

	mu      sync.Mutex
	once    sync.Once
	n       int
	pending []chan error
	cancel  func()
}

func (t *TaskGroup) init() {
	t.once.Do(func() {
		if t.N == 0 {
			t.N = 1
		}

		t.pending = make([]chan error, t.N)
	})
}

// Do executes given function task in separate goroutine n minus <currently
// running tasks number> times. It returns slice of n channels which
// fulfillment means the end of appropriate task execution.
//
// That is, for m already running tasks Do(n, n < m) will return n channels
// referring to a previously spawned task goroutines.
//
// All currenlty executing tasks can be signaled to cancel by calling
// TaskGroup's Cancel() method.
//
// nolint:gocognit
func (t *TaskGroup) Do(ctx context.Context, n int, task func(context.Context, int) error) []<-chan error {
	t.init()

	if n > t.N {
		n = t.N
	}

	ret := make([]<-chan error, 0, n)

	t.mu.Lock()
	defer t.mu.Unlock()

	if exec := n - t.n; exec > 0 {
		// Start remaining tasks.
		subctx, cancel := context.WithCancel(ctx)
		// Append current call context to previous.
		prev := t.cancel
		t.cancel = func() {
			if prev != nil {
				prev()
			}

			cancel()
		}

		for i := 0; i < exec; i++ {
			var j int

			for ; j < len(t.pending); j++ {
				if t.pending[j] != nil {
					// Filter out already active "promises".
					continue
				}

				break
			}

			done := make(chan error, 1)
			err := goer(ctx, t.Goer, func() {
				done <- task(subctx, j)

				t.mu.Lock()
				defer t.mu.Unlock()

				exec--
				if exec == 0 {
					// Cancel current sub context.
					cancel()
				}
				if t.pending[j] == done {
					// Current activity was not canceled.
					t.pending[j] = nil
					t.n--
					if t.n == 0 {
						t.cancel = nil
					}
				}
			})

			if err != nil {
				// Spawn goroutine error. Fulfill channel immediately.
				done <- err
			} else {
				t.pending[j] = done
				t.n++
			}
		}
	}

	for i := 0; i < len(t.pending) && len(ret) < n; i++ {
		if t.pending[i] == nil {
			continue
		}

		ret = append(ret, t.pending[i])
	}

	return ret
}

// Cancel cancels context of all currently running tasks. Further Do() calls
// will not be blocked on waiting for exit of previous tasks.
func (t *TaskGroup) Cancel() {
	t.init()

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}

	for i := range t.pending {
		// NOTE: Do not close the pending channel.
		// It will be closed by a task runner.
		//
		// Set to nil to prevent memory leaks.
		t.pending[i] = nil
	}

	t.n = 0
}
