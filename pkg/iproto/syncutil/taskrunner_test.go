package syncutil

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/net/context"
)

func TestTaskRunnerDo(t *testing.T) {
	tr := TaskRunner{}

	var rcvrs []<-chan error

	taskRunTime := time.Duration(10 * time.Millisecond)
	taskWaitTime := time.Duration(12 * time.Millisecond)
	taskSubscribersNumber := 10

	for i := 0; i < taskSubscribersNumber; i++ {
		rcvrs = append(rcvrs, tr.Do(context.Background(), func(ctx context.Context) error {
			time.Sleep(taskRunTime)

			return fmt.Errorf("Some error")
		}))
	}

	for _, rcvr := range rcvrs {
		select {
		case <-rcvr:
		case <-time.After(taskWaitTime):
			t.Fatal("must have already received task result for all receivers")
		}
	}
}

func TestTaskRunnerCancel(t *testing.T) {
	tr := TaskRunner{}

	result := tr.Do(context.Background(), func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	})

	time.Sleep(10 * time.Millisecond)

	tr.Cancel()

	select {
	case <-result:
	case <-time.After(10 * time.Millisecond):
		t.Fatal("wanted task to be canceled")
	}
}

func TestTaskRunnerRecovery(t *testing.T) {
	tr := TaskRunner{}

	var rcvrs []<-chan error

	taskWaitTime := time.Duration(12 * time.Millisecond)
	taskSubscribersNumber := 2

	for i := 0; i < taskSubscribersNumber; i++ {
		rcvrs = append(rcvrs, tr.Do(context.Background(), func(ctx context.Context) error {
			panic("panic")
		}))
	}

	for _, rcvr := range rcvrs {
		select {
		case response := <-rcvr:
			if response != ErrTaskPanic {
				t.Fatal("must have received task panic error for all receivers")
			}
		case <-time.After(taskWaitTime):
			t.Fatal("must have already received task result for all receivers")
		}
	}
}
