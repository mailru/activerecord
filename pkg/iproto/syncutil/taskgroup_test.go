package syncutil

import (
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/context"
)

var bg = context.Background()

func TestTaskGroupDoDeadlock(t *testing.T) {
	sem := make(chan struct{}, 1)
	tg := TaskGroup{
		N: 2,
		Goer: GoerFn(func(ctx context.Context, task func()) error {
			sem <- struct{}{}
			go func() {
				defer func() { <-sem }()
				task()
			}()
			return nil
		}),
	}

	var (
		done = make(chan struct{})
		task = make(chan int, 2)
	)
	go func() {
		defer close(done)
		tg.Do(bg, 2, func(ctx context.Context, i int) error {
			task <- i
			return nil
		})
	}()
	select {
	case <-done:
		t.Errorf("Do() returned; want deadlock")
	case <-time.After(time.Second):
	}
	if n := len(task); n != 1 {
		t.Fatalf("want only one task to be executed; got %d", n)
	}
	if i := <-task; i != 0 {
		t.Fatalf("want task #%d be executed; got #%d", 0, i)
	}
}

func TestTaskGroupDo(t *testing.T) {
	const N = 8
	s := TaskGroup{
		N: N,
	}

	sleep := make(chan struct{})
	ret := s.Do(bg, N, func(ctx context.Context, i int) error {
		<-sleep
		return nil
	})

	for i := 0; i < 100; i++ {
		time.Sleep(time.Millisecond)
		s.Do(bg, N, func(_ context.Context, _ int) error {
			panic("must not be called")
		})
	}

	close(sleep)
	if err := WaitPending(bg, ret); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTaskGroupCancel(t *testing.T) {
	s := TaskGroup{
		N: 1,
	}

	time.AfterFunc(50*time.Millisecond, func() {
		s.Cancel()
	})
	a := DoOne(bg, &s, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	b := DoOne(bg, &s, func(ctx context.Context) error {
		panic("must not be called")
	})

	if b != a {
		t.Fatalf("unexpected exec")
	}

	if err, want := <-a, context.Canceled; err != want {
		t.Errorf("got %v; want %v", err, want)
	}

	c := DoOne(bg, &s, func(ctx context.Context) error {
		return nil
	})

	if err := <-c; err != nil {
		t.Fatal(err)
	}
}

func TestTaskGroupSplit(t *testing.T) {
	s := TaskGroup{
		N: 2,
	}
	var (
		a = make(chan struct{}, 1)
		b = make(chan struct{}, 1)
		n = new(int32)
	)
	s.Do(bg, 2, func(ctx context.Context, i int) error {
		atomic.AddInt32(n, 1)
		switch i {
		case 0:
			<-a
		case 1:
			<-b
		}
		return nil
	})

	// Release first task.
	a <- struct{}{}
	time.Sleep(10 * time.Millisecond)
	s.Do(bg, 2, func(ctx context.Context, i int) error {
		atomic.AddInt32(n, 1)
		if i != 0 {
			t.Fatalf("unexpected index: %d", i)
		}
		<-a
		return nil
	})

	// Release second task.
	b <- struct{}{}
	time.Sleep(10 * time.Millisecond)
	s.Do(bg, 2, func(ctx context.Context, i int) error {
		atomic.AddInt32(n, 1)
		if i != 1 {
			t.Fatalf("unexpected index: %d", i)
		}
		<-b
		return nil
	})

	time.Sleep(10 * time.Millisecond)
	if m := atomic.LoadInt32(n); m != 4 {
		t.Fatalf("unexpected number of executed tasks: %d", m)
	}
}

func DoOne(ctx context.Context, s *TaskGroup, cb func(context.Context) error) <-chan error {
	ps := s.Do(ctx, 1, func(ctx context.Context, _ int) error {
		return cb(ctx)
	})
	return ps[0]
}

func WaitPending(ctx context.Context, chs []<-chan error) error {
	var fail error
	for _, ch := range chs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-ch:
			if err != nil && fail == nil {
				fail = err
			}
		}
	}
	return fail
}
