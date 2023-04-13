package syncutil

import (
	"fmt"
	"log"
	"sync"

	"golang.org/x/net/context"
)

var ErrTaskPanic = fmt.Errorf("task panic occurred")

// TaskRunner runs only one task. Caller can
// subscribe to current task or in case if no task
// is running initiate a new one via Do method.
//
// Check MAILX-1585 for details.
type TaskRunner struct {
	mu sync.RWMutex

	rcvrs []chan error

	cancel func()
}

// Do returns channel from which the result of the current task will
// be returned.
//
// In case if task is not running, it creates one.
func (t *TaskRunner) Do(ctx context.Context, task func(context.Context) error) <-chan error {
	result := make(chan error, 1)

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.rcvrs == nil {
		t.initTask(ctx, task)
	}

	t.rcvrs = append(t.rcvrs, result)

	return result
}

func (t *TaskRunner) Cancel() {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.cancel != nil {
		t.cancel()
	}
}

func (t *TaskRunner) initTask(ctx context.Context, task func(context.Context) error) {
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	go func() {
		defer func() {
			t.makeRecover(recover())
			cancel()
		}()

		err := task(ctx)

		t.broadcastErr(err)
	}()
}

func (t *TaskRunner) broadcastErr(err error) {
	t.mu.Lock()
	rcvrs := t.rcvrs
	t.rcvrs = nil
	t.mu.Unlock()

	if rcvrs == nil {
		return
	}

	for _, subscriber := range rcvrs {
		subscriber <- err
	}
}

func (t *TaskRunner) makeRecover(rec interface{}) {
	if rec != nil {
		log.Printf("[internal_error] panic occurred in TaskRunner: %v", rec)
		t.broadcastErr(ErrTaskPanic)
	}
}
