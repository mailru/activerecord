package pool

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	_ "unsafe" // for go:linkname

	"golang.org/x/net/context"
)

func TestPoolScheduleAfterClose(t *testing.T) {
	for _, test := range []struct {
		name   string
		config *Config
	}{
		{
			config: &Config{
				UnstoppableWorkers: 5,
				MaxWorkers:         10,
				WorkQueueSize:      5,
			},
		},
		{
			config: &Config{
				UnstoppableWorkers: 0,
				MaxWorkers:         0,
			},
		},
		{
			config: &Config{
				UnstoppableWorkers: 1,
				MaxWorkers:         0,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := Must(New(test.config))
			p.Close()
			if err := p.Schedule(TaskFunc(nil)); err != ErrPoolClosed {
				t.Errorf("error is %v; want %v", err, ErrPoolClosed)
			}
		})
	}
}

func TestPoolWait(t *testing.T) {
	for _, test := range []struct {
		name     string
		config   *Config
		schedule int
		delay    time.Duration
		err      error
		timeout  time.Duration
	}{
		{
			schedule: 15,
			config: &Config{
				UnstoppableWorkers: 5,
				MaxWorkers:         10,
				ExtraWorkerTTL:     time.Millisecond,
				WorkQueueSize:      5,
			},
		},
		{
			schedule: 3,
			config: &Config{
				UnstoppableWorkers: 5,
				MaxWorkers:         10,
				ExtraWorkerTTL:     time.Millisecond,
				WorkQueueSize:      5,
			},
		},
		{
			schedule: 5,
			config: &Config{
				UnstoppableWorkers: 1,
				MaxWorkers:         1,
				ExtraWorkerTTL:     time.Millisecond,
				WorkQueueSize:      5,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := Must(New(test.config))

			n := test.schedule
			m := new(int32)

			release := make(chan struct{})
			for i := 0; i < n; i++ {
				err := p.ScheduleTimeout(time.Second, TaskFunc(func() {
					<-release
					atomic.AddInt32(m, 1)
				}))
				if err != nil {
					t.Fatal(err)
				}
			}
			time.AfterFunc(test.delay, func() {
				close(release)
			})

			var timeout <-chan time.Time
			if tm := test.timeout; tm != 0 {
				timeout = time.After(tm)
			}
			err := p.wait(timeout, nil)
			if err != test.err {
				t.Errorf("unexpected error: %v; want %v", err, test.err)
			}
			if m := int(atomic.LoadInt32(m)); m != n {
				t.Errorf("wait returned before all queued tasks completed: %d(%d)", m, n)
			}
		})
	}
}

func TestPoolScheduleOnClosedPool(t *testing.T) {
	for _, test := range []struct {
		name   string
		config *Config
	}{
		{
			config: &Config{
				MaxWorkers:         0,
				UnstoppableWorkers: 1,
				WorkQueueSize:      0,
			},
		},
		{
			config: &Config{
				MaxWorkers:         1,
				UnstoppableWorkers: 1,
				WorkQueueSize:      1,
			},
		},
		{
			config: &Config{
				MaxWorkers:         50,
				UnstoppableWorkers: 10,
				WorkQueueSize:      10,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			release := make(chan struct{})
			test.config.OnTaskIn = func() {
				<-release
			}

			p := Must(New(test.config))
			time.AfterFunc(time.Millisecond, func() {
				go p.Close()
				close(release)
			})

			scheduled := make(chan error, (test.config.WorkQueueSize+1)*5)
			for i := 0; i < cap(scheduled); i++ {
				go func() {
					err := p.Schedule(TaskFunc(func() {
						scheduled <- nil
					}))
					if err != nil {
						scheduled <- err
					}
				}()
			}
			timeout := time.After(time.Second)
			for i := 0; i < cap(scheduled); i++ {
				select {
				case <-scheduled:
					//t.Logf("schedule error: %v", err)
				case <-timeout:
					t.Errorf("task was not scheduled during 1s")
				}
			}

			<-p.Done()
		})
	}
}

func TestPoolBarrierOnClosedPool(t *testing.T) {
	p := Must(New(&Config{
		MaxWorkers:         1,
		UnstoppableWorkers: 1,
	}))

	release := make(chan struct{})
	_ = p.Schedule(TaskFunc(func() {
		<-release
	}))
	// After closing release channel, we expect that Done() will be fulfilled,
	// and Barrier() will return somewhen close to this.
	time.AfterFunc(10*time.Millisecond, func() {
		close(release)
	})

	// Do not wait when Close() returns.
	go p.Close()

	timeline := make(chan time.Time, 2)
	go func() {
		p.Barrier()
		timeline <- time.Now()
	}()
	go func() {
		<-p.Done()
		timeline <- time.Now()
	}()

	a := <-timeline
	b := <-timeline

	diff := a.Sub(b).Nanoseconds()
	if act, max := time.Duration(int64abs(diff)), 500*time.Microsecond; act > max {
		t.Errorf(
			"difference between Done() and Barrier() is %v; want at most %v",
			act, max,
		)
	}
}

func TestPoolWaitOnClosedPool(t *testing.T) {
	t.Skip("Unstable test")
	p := Must(New(&Config{
		MaxWorkers:         1,
		UnstoppableWorkers: 1,
	}))

	release := make(chan struct{})
	_ = p.Schedule(TaskFunc(func() {
		<-release
	}))
	// After closing release channel, we expect that Done() will be fulfilled,
	// and Wait() will return somewhen close to this.
	time.AfterFunc(10*time.Millisecond, func() {
		close(release)
	})

	// Do not wait when Close() returns.
	go p.Close()

	timeline := make(chan time.Time, 2)
	go func() {
		p.Wait()
		timeline <- time.Now()
	}()
	go func() {
		<-p.Done()
		timeline <- time.Now()
	}()

	a := <-timeline
	b := <-timeline

	diff := a.Sub(b).Nanoseconds()
	if act, max := time.Duration(int64abs(diff)), 500*time.Microsecond; act > max {
		t.Errorf(
			"difference between Done() and Wait() is %v; want at most %v",
			act, max,
		)
	}
}

func TestPoolTwiceClose(t *testing.T) {
	p := Must(New(&Config{
		MaxWorkers:         1,
		UnstoppableWorkers: 1,
	}))

	release := make(chan struct{})

	_ = p.Schedule(TaskFunc(func() {
		<-release
	}))

	time.AfterFunc(10*time.Millisecond, func() {
		close(release)
	})

	timeline := make(chan time.Time, 2)
	go func() {
		p.Close()
		timeline <- time.Now()
	}()
	go func() {
		p.Close()
		timeline <- time.Now()
	}()

	a := <-timeline
	b := <-timeline

	diff := a.Sub(b).Nanoseconds()
	if act, max := time.Duration(int64abs(diff)), 100*time.Microsecond; act > max {
		t.Errorf(
			"difference between Close() return is %v; want at most %v",
			act, max,
		)
	}
}

func int64abs(v int64) int64 {
	m := v >> 63 // Get 111 for negative and 000 for positive.
	v ^= m       // Get (NOT v) for negative and v for positive.
	v -= m       // Subtract -1 (add one) for negative and 0 for positive.
	return v
}

func TestPoolMulticast(t *testing.T) {
	p := Must(New(&Config{
		MaxWorkers:         2,
		UnstoppableWorkers: 2,
		WorkQueueSize:      0,
	}))

	var (
		seq     = make(chan string)
		release = make(chan struct{})
	)

	// Lock first worker.
	_ = p.Schedule(TaskFunc(func() {
		<-release
		seq <- "unicast"
	}))

	// Send multicast task for every worker. We expect that second worker runs
	// it immediately.
	n, err := p.multicast(TaskFunc(func() {
		seq <- "multicast"
	}), nil, nil)

	if err != nil {
		t.Fatal(err)
	}

	if n != 2 {
		t.Fatalf("multicast task sent to %d workers; want %d", n, 2)
	}

	// Prepare store the order of execution.
	var tasks [3]string

	// Note that because worker's direct channel is buffered, scheduler will
	// not run worker goroutine immediately in most of times. That's why we
	// need to force scheduling by locking on seq channel.
	select {
	case tasks[0] = <-seq:
	case <-time.After(time.Second):
		t.Fatalf("no action during second")
	}

	// Release first worker. We expect that after task done first worker will
	// run multicast task.
	close(release)
	tasks[1] = <-seq
	tasks[2] = <-seq

	if tasks != [3]string{"multicast", "unicast", "multicast"} {
		t.Fatalf("unexpected order of task execution: %v", tasks)
	}
}

func TestPoolBarrier(t *testing.T) {
	p := Must(New(&Config{
		MaxWorkers:         32,
		UnstoppableWorkers: 32,
		WorkQueueSize:      32,
	}))

	// Prepare two counters, which will be used before and after Barrier().
	a := new(int32)
	b := new(int32)

	var counter atomic.Value
	counter.Store(a)

	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			runtime.LockOSThread()
			var err error
			for err == nil {
				err = p.ScheduleCustom(nil, TaskFunc(func() {
					n := counter.Load().(*int32)
					// Add some delay to imitate some real work.
					time.Sleep(time.Microsecond)
					atomic.AddInt32(n, 1)
				}))
			}
		}()
	}

	// Let the workers to increment A counter.
	time.Sleep(time.Millisecond * 50)

	// Swap counter. Note that after this, workers could increment both
	// counters.
	counter.Store(b)

	// Barrier workers. After that all workers MUST increment only B counter.
	p.Barrier()

	// Load last value of A counter.
	x := atomic.LoadInt32(a)

	// Let the workers to increment B counter.
	time.Sleep(time.Millisecond * 50)

	// Stop the pool.
	p.Close()

	if n := atomic.LoadInt32(a); n != x {
		t.Fatalf("counter has been changed after Barrier(); %d != %d", n, x)
	}
}

func TestPoolNew(t *testing.T) {
	for i, test := range []struct {
		config *Config
		spawn  int
		err    bool
	}{
		{
			config: &Config{MaxWorkers: 1, UnstoppableWorkers: 2},
			err:    true,
		},
		{
			config: &Config{MaxWorkers: 0},
			err:    false,
		},
		{
			config: &Config{MaxWorkers: 1, UnstoppableWorkers: 2},
			err:    true,
		},
		{
			config: &Config{MaxWorkers: 1, UnstoppableWorkers: 0, WorkQueueSize: 1},
			err:    true,
		},
		{
			config: &Config{MaxWorkers: 1, UnstoppableWorkers: 0},
			spawn:  0,
		},
		{
			config: &Config{MaxWorkers: 1, UnstoppableWorkers: 1},
			spawn:  1,
		},
		{
			config: &Config{MaxWorkers: 8, UnstoppableWorkers: 4},
			spawn:  4,
		},
	} {
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			p, err := New(test.config)
			if test.err && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !test.err && err != nil {
				t.Errorf("unexpected error: %s", err)
			}
			if err != nil {
				return
			}
			if n := len(p.sem); n != test.spawn {
				t.Errorf("spawned %d goroutines; want %d", n, test.spawn)
			}
		})
	}
}

func TestPoolSchedule(t *testing.T) {
	for i, test := range []struct {
		config      *Config
		tasks       int
		spawnBefore int
		spawnAfter  int
		kill        int
		cbDelay     time.Duration
		sleep       time.Duration
	}{
		{
			config: &Config{
				MaxWorkers:     4,
				WorkQueueSize:  0,
				ExtraWorkerTTL: time.Millisecond * 100,
			},
			tasks:       4,
			spawnBefore: 0,
			spawnAfter:  4,
			kill:        4,

			cbDelay: time.Millisecond * 2,
			sleep:   time.Millisecond * 500,
		},
		{
			config: &Config{
				UnstoppableWorkers: 2,
				MaxWorkers:         4,
				WorkQueueSize:      4,
				ExtraWorkerTTL:     time.Millisecond,
			},
			tasks:       4,
			spawnBefore: 2,
			spawnAfter:  0,
			kill:        0,

			cbDelay: time.Millisecond * 2,
			sleep:   time.Millisecond * 4,
		},
		{
			config: &Config{
				UnstoppableWorkers: 0,
				MaxWorkers:         4,
				WorkQueueSize:      0,
				ExtraWorkerTTL:     time.Millisecond,
			},
			tasks:       16,
			spawnBefore: 0,
			spawnAfter:  4,
			kill:        4,

			cbDelay: time.Millisecond * 2,
			sleep:   time.Millisecond * 16,
		},
		{
			config: &Config{
				UnstoppableWorkers: 2,
				MaxWorkers:         0, // no limit
				WorkQueueSize:      0,
				ExtraWorkerTTL:     time.Millisecond,
			},
			tasks:       8,
			spawnBefore: 2,
			spawnAfter:  6,
			kill:        6,

			cbDelay: time.Millisecond * 2,
			sleep:   time.Millisecond * 16,
		},
	} {
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			pool, err := New(test.config)
			if err != nil {
				t.Fatal(err)
			}

			if test.config.UnstoppableWorkers > 0 {
				// Let workers to be spawned.
				time.Sleep(time.Millisecond * 100)
			}

			n1 := wgCount(&pool.wg)
			if n1 != test.spawnBefore {
				t.Errorf("spawned %d goroutines before tasks; want %d", n1, test.spawnBefore)
			}

			var callbacks sync.WaitGroup
			callbacks.Add(test.tasks)
			for i := 0; i < test.tasks; i++ {
				err := pool.Schedule(TaskFunc(func() {
					time.Sleep(test.cbDelay)
					callbacks.Done()
				}))
				if err != nil {
					t.Fatal(err)
				}
			}

			runtime.Gosched()

			n2 := wgCount(&pool.wg)
			if n := n2 - n1; n != test.spawnAfter {
				t.Errorf("spawned %d goroutines after tasks; want %d", n, test.spawnAfter)
			}

			callbacks.Wait()
			time.Sleep(test.sleep)

			n3 := wgCount(&pool.wg)
			if n := n2 - n3; n != test.kill {
				t.Errorf("killed %d goroutines after sleep; want %d", n, test.kill)
			}
		})
	}
}

func TestPoolScheduleImmediate(t *testing.T) {
	// We set GOMAXPROCS to 1 here to avoid races on first locking task
	// completion and worker locking on task queue. When GOMAXPROCS > 1 we
	// could get flaky errors here, such that locker is finished, but worker
	// not yet locked on task queue, but our goroutine already calls
	// ScheduleImmediate and fails with ErrUnavailable.
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))

	pool := Must(New(&Config{
		MaxWorkers:    1,
		WorkQueueSize: 0,
	}))

	var (
		err error

		lock = make(chan struct{})
		done = make(chan struct{})
		noop = TaskFunc(func() {})
	)
	locker := TaskFunc(func() {
		<-lock
		close(done)
	})

	if err = pool.ScheduleImmediate(locker); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err = pool.ScheduleImmediate(noop); err == nil {
		t.Fatalf("expected error got nil")
	}

	// Complete the first task.
	close(lock)
	<-done

	// Let the worker to lock on reading from pool.work queue.
	runtime.Gosched()

	if err = pool.ScheduleImmediate(noop); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPoolScheduleTimeout(t *testing.T) {
	pool, err := New(&Config{
		MaxWorkers:    1,
		WorkQueueSize: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	// First, create task that will block other tasks execution until we
	// close done channel.
	done := make(chan struct{})
	err = pool.ScheduleTimeout(10*time.Millisecond, TaskFunc(func() {
		<-done
	}))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Next, try to schedule task and expect the ErrUnavailable.
	err = pool.ScheduleTimeout(10*time.Millisecond, TaskFunc(func() {
		t.Errorf("unexpected task execution")
	}))
	if err != ErrUnavailable {
		t.Errorf("unexpected error: %s; want %s", err, ErrUnavailable)
	}

	// Finally, release the pool and try to schedule another task,
	// expecting that it well be okay.
	close(done)
	ok := make(chan struct{})
	err = pool.ScheduleTimeout(10*time.Millisecond, TaskFunc(func() {
		close(ok)
	}))
	if err != nil {
		t.Errorf("unexecpted error: %s", err)
	}

	<-ok
}

func TestScheduleContext(t *testing.T) {
	pool, err := New(&Config{
		MaxWorkers:    1,
		WorkQueueSize: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	// First, create task that will block other tasks execution until we
	// close done channel.
	done := make(chan struct{})
	err = pool.ScheduleTimeout(time.Millisecond, TaskFunc(func() {
		<-done
	}))
	if err != nil {
		t.Errorf("unexecpted error: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Millisecond, cancel)

	// Next, try to schedule task and expect the context.Canceled error.
	err = pool.ScheduleContext(ctx, TaskFunc(func() {
		t.Errorf("unexpected task execution")
	}))
	if err != context.Canceled {
		t.Errorf("unexpected error: %s; want %s", err, context.Canceled)
	}

	// Finally, release the pool and try to schedule another task,
	// expecting that it well be okay.
	close(done)
	ok := make(chan struct{})
	err = pool.ScheduleContext(context.Background(), TaskFunc(func() {
		close(ok)
	}))
	if err != nil {
		t.Errorf("unexecpted error: %s", err)
	}

	<-ok
}

func TestPoolClose(t *testing.T) {
	pool, err := New(&Config{
		UnstoppableWorkers: 1,
		MaxWorkers:         1,
		WorkQueueSize:      1,
	})
	if err != nil {
		t.Fatal(err)
	}

	task := TaskFunc(func() {})
	err = pool.ScheduleImmediate(task)
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()

	if err = pool.ScheduleImmediate(task); err != ErrPoolClosed {
		t.Fatal(err)
	}
}

func TestPoolScheduleStat(t *testing.T) {
	var tasks int32

	p := Must(New(&Config{
		UnstoppableWorkers: 1,
		MaxWorkers:         1,
		WorkQueueSize:      0,

		OnTaskIn: func() {
			atomic.AddInt32(&tasks, 1)
		},
		OnTaskOut: func() {
			atomic.AddInt32(&tasks, -1)
		},
	}))

	// First lock the pool.
	done := make(chan struct{})
	_ = p.Schedule(TaskFunc(func() {
		<-done
	}))

	// Prepare canceled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Make multiple Schedule* calls, all of which will fail.
	_ = p.ScheduleImmediate(nil)
	_ = p.ScheduleTimeout(time.Nanosecond, nil)
	_ = p.ScheduleContext(ctx, nil)
	_ = p.ScheduleCustom(ctx.Done(), nil)

	close(done)
	// Let pool become unlocked.
	runtime.Gosched()

	if n := atomic.LoadInt32(&tasks); n != 0 {
		t.Fatalf("eventually got %d enqueued tasks; want 0", n)
	}
}

func BenchmarkSchedule(b *testing.B) {
	for _, test := range []struct {
		config *Config
	}{
		{&Config{UnstoppableWorkers: 0, MaxWorkers: 1, WorkQueueSize: 0}},
		{&Config{UnstoppableWorkers: 8, MaxWorkers: 8, WorkQueueSize: 0}},
		{&Config{UnstoppableWorkers: 8, MaxWorkers: 8, WorkQueueSize: 8}},
		{&Config{UnstoppableWorkers: 1, MaxWorkers: 8, WorkQueueSize: 8}},
		{&Config{UnstoppableWorkers: 1, MaxWorkers: 0, WorkQueueSize: 8}},
		{&Config{UnstoppableWorkers: 1, MaxWorkers: 0, WorkQueueSize: 0}},
	} {

		b.Run(test.config.String(), func(b *testing.B) {
			var wg sync.WaitGroup
			wg.Add(b.N)
			task := TaskFunc(wg.Done)

			pool := Must(New(test.config))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = pool.schedule(task, nil, nil)
			}

			wg.Wait()
		})
	}
}

func (c *Config) String() string {
	return fmt.Sprintf(
		"unstpb:%d max:%d queue:%d",
		c.UnstoppableWorkers,
		c.MaxWorkers,
		c.WorkQueueSize,
	)
}

func wgCount(wg *sync.WaitGroup) int {
	statep := wgState(wg)
	v := atomic.LoadUint64(statep)
	return int(v >> 32)
}

//go:linkname wgState sync.(*WaitGroup).state
func wgState(*sync.WaitGroup) *uint64
