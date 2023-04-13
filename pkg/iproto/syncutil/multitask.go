package syncutil

import (
	"sync"

	"golang.org/x/net/context"
)

// Multitask helps to run N tasks in parallel.
type Multitask struct {
	// ContinueOnError disables cancellation of sub context passed to
	// action callbackwhen it is not possible to start all N goroutines to
	// prepare an action.
	ContinueOnError bool

	// Goer starts a goroutine which executes a given task.
	// It is useful when client using some pool of goroutines.
	//
	// If nil, then default `go` is used and context is ignored.
	//
	// Non-nil error from Goer means that some resources are temporarily
	// unavailable and given task will not be executed.
	Goer GoerFn
}

// Do executes actor function N times probably in parallel.
// It blocks until all actions are done or become canceled.
func (m Multitask) Do(ctx context.Context, n int, actor func(context.Context, int) bool) (err error) {
	// Prepare sub context to get the ability of cancelation remaining actions
	// when user decide to stop.
	subctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Prapre wait group counter.
	var wg sync.WaitGroup

	wg.Add(n)

	for i := 0; i < n; i++ {
		// Remember index of i.
		index := i
		// NOTE: We must spawn a goroutine with exactly root context, and call
		// actor() with exactly sub context to prevent goer() falsy errors.
		err = goer(ctx, m.Goer, func() {
			if !actor(subctx, index) && subctx.Err() == nil {
				cancel()
			}
			wg.Done()
		})
		if err != nil {
			// Reduce wait group counter to zero because we do not want to
			// proceed.
			for j := i; j < n; j++ {
				wg.Done()
			}

			if !m.ContinueOnError {
				cancel()
			}

			// We are pessimistic here. If Goer could not prepare our request,
			// we assume that other requests will fail too.
			//
			// It is also works on case when Goer is relies only on context –
			// if context is canceled no more requests can be processed.
			break
		}
	}

	// Wait for the sent requests.
	wg.Wait()

	return err
}

// Every starts n goroutines and runs actor inside each. If some actor returns
// error it stops processing and cancel other actions by canceling their
// context argument. It returns first error occured.
func Every(ctx context.Context, n int, actor func(context.Context, int) error) error {
	m := Multitask{
		ContinueOnError: false,
	}

	var (
		mu   sync.Mutex
		fail error
	)

	_ = m.Do(ctx, n, func(ctx context.Context, i int) bool {
		if err := actor(ctx, i); err != nil {
			mu.Lock()
			if fail == nil {
				fail = err
			}
			mu.Unlock()

			return false
		}

		return true
	})

	return fail
}

// Each starts n goroutines and runs actor inside it. It returns when all
// actors return.
func Each(ctx context.Context, n int, actor func(context.Context, int)) {
	m := Multitask{
		ContinueOnError: true,
	}

	_ = m.Do(ctx, n, func(ctx context.Context, i int) bool {
		actor(ctx, i)
		return true
	})
}

// GoerFn represents function that starts a goroutine which executes a given
// task.
type GoerFn func(context.Context, func()) error

func goer(ctx context.Context, g GoerFn, task func()) error {
	if g != nil {
		return g(ctx, task)
	}

	go task()

	return nil
}
