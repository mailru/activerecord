package pool

import (
	"container/list"
	"errors"
	"runtime"
	"sync"
	"time"

	ptime "github.com/mailru/activerecord/pkg/iproto/util/time"
	"golang.org/x/net/context"
)

const (
	DefaultExtraWorkerTTL = time.Second
)

var (
	ErrConfigMalformed        = errors.New("malformed pool config: max number of workers is lower than number of unstoppable workers")
	ErrConfigQueueUnstoppable = errors.New("malformed pool config: queue is buffered, but unstoppable workers set to 0")
	ErrUnavailable            = errors.New("pool: temporarily unavailable")
	ErrPoolClosed             = errors.New("pool: closed")
)

// Task represents task that need to be executed by some worker.
type Task interface {
	Run()
}

type TaskFunc func()

func (fn TaskFunc) Run() { fn() }

type Config struct {
	// Number of workers that are always running.
	UnstoppableWorkers int

	// Maximum number of workers that could be spawned.
	// It includes UnstoppableWorkers count.
	// If MaxWorkers is <= 0 then it interpreted as extra workers are unlimited.
	// If MaxWorkers is in range [1, UnstoppableWorkers) then config is malformed.
	MaxWorkers int

	// When work flow becomes too much for unstoppable workers,
	// Pool could spawn extra worker if it fits max workers limit.
	// Spawned worker will be alive for this amount of time.
	ExtraWorkerTTL time.Duration

	// This parameter manages the size of work queue.
	// The smaller this value, the greater the probability of
	// spawning the extra worker. And vice versa.
	WorkQueueSize int

	// QueueTiming is used to calculate duration from task receiving
	// to begining of task execution.
	// QueueTiming stat.SimpleTiming

	// ExecTiming is used to calculate duration of task execution.
	// ExecTiming stat.SimpleTiming

	// IdleTiming is used to calculate idle time of each worker.
	// IdleTiming stat.SimpleTiming

	// OnTaskIn, OnTaskOut are used to signal whan task is scheduled/pulled from queue.
	OnTaskIn, OnTaskOut func()

	// OnWorkerStart, OnWorkerStop are called on extra worker spawned/stopped.
	OnWorkerStart, OnWorkerStop func()
}

func dummyStatPush() {}

func (c *Config) withDefaults() (config Config) {
	if c != nil {
		config = *c
	}

	if config.ExtraWorkerTTL == 0 {
		config.ExtraWorkerTTL = DefaultExtraWorkerTTL
	}

	// if config.QueueTiming == nil {
	// 	config.QueueTiming = stat.DummyTiming
	// }
	// if config.ExecTiming == nil {
	// 	config.ExecTiming = stat.DummyTiming
	// }
	// if config.IdleTiming == nil {
	// 	config.IdleTiming = stat.DummyTiming
	// }

	if config.OnTaskIn == nil {
		config.OnTaskIn = dummyStatPush
	}

	if config.OnTaskOut == nil {
		config.OnTaskOut = dummyStatPush
	}

	if config.OnWorkerStart == nil {
		config.OnWorkerStart = dummyStatPush
	}

	if config.OnWorkerStop == nil {
		config.OnWorkerStop = dummyStatPush
	}

	return
}

func (c Config) check() error {
	if c.MaxWorkers > 0 && c.MaxWorkers < c.UnstoppableWorkers {
		return ErrConfigMalformed
	}

	// If work queue set to be buffered, but unstoppable workers are set to 0, then
	// pool will not process tasks until work queue will be full.
	if c.UnstoppableWorkers == 0 && c.WorkQueueSize > 0 {
		return ErrConfigQueueUnstoppable
	}

	return nil
}

type Pool struct {
	config Config

	work   chan Task
	sem    chan struct{}
	kill   chan struct{}
	done   chan struct{}
	wg     sync.WaitGroup
	noCork bool

	mu   sync.RWMutex
	list *list.List
}

func New(c *Config) (*Pool, error) {
	config := c.withDefaults()

	if err := config.check(); err != nil {
		return nil, err
	}

	p := &Pool{
		config: config,
		work:   make(chan Task, config.WorkQueueSize),
		kill:   make(chan struct{}),
		done:   make(chan struct{}),
		list:   list.New(),
	}

	if config.MaxWorkers > 0 {
		p.sem = make(chan struct{}, config.MaxWorkers)
	}

	for i := 0; i < config.UnstoppableWorkers; i++ {
		if p.sem != nil {
			p.sem <- struct{}{}
		}

		_ = p.spawn(nil, false)
	}

	return p, nil
}

// Must is a helper that wraps a call to a function returning (*Pool, error)
// and panics if the error is non-nil.
// It is intended for use in variable initializations with New() function.
func Must(p *Pool, err error) *Pool {
	if err != nil {
		panic(err)
	}

	return p
}

func (p *Pool) SetNoCork(v bool) {
	p.noCork = v
}

// Close terminates all spawned goroutines.
func (p *Pool) Close() error {
	p.mu.Lock()
	select {
	case <-p.kill:
		// Close() was already called.
		// Wait for done to be closed.
		p.mu.Unlock()
		<-p.done

	default:
		// Close the kill channel under the mutex to accomplish two things.
		// First is to get rid from concurrent wg.Add() calls while waiting for
		// wg.Done() below.
		// Second is to avoid panics on concurrent Close() calls.
		close(p.kill)
		p.mu.Unlock()

		// Wait for all workers exit.
		p.wg.Wait()

		// No more alive workers at this point. So we can safely fulfill work
		// queue to avoid concurrent Schedule*() tasks get stucked in the work
		// queue without progress.
		if !p.noCork {
			cork(p.work)
		}

		// Signal that pool is completely closed.
		close(p.done)
	}

	return nil
}

// Done returns channel which closure means that pool is done with all its work.
func (p *Pool) Done() <-chan struct{} {
	return p.done
}

// Schedule makes task to be scheduled over pool's workers.
// It returns non-nil only if pool become closed and task can not be executed
// over its workers.
func (p *Pool) Schedule(t Task) error {
	return p.schedule(t, nil, nil)
}

// immediate indicates that task scheduling must complete or fail right now.
var immediate = make(chan time.Time)

// ScheduleImmediate makes task to be scheduled without waiting for free
// workers. That is, if all workers are busy and pool is not rubber, then
// ErrUnavailable is returned immediately.
func (p *Pool) ScheduleImmediate(t Task) error {
	return p.schedule(t, immediate, nil)
}

// ScheduleTimeout makes task to be scheduled over pool's workers.
//
// Non-nil error only available when tm is not 0. That is, if no workers are
// available during timeout, it returns error.
//
// Zero timeout means that there are no timeout for awaiting for available
// worker.
func (p *Pool) ScheduleTimeout(tm time.Duration, t Task) error {
	var timeout <-chan time.Time

	if tm != 0 {
		timer := ptime.AcquireTimer(tm)
		defer ptime.ReleaseTimer(timer)
		timeout = timer.C
	}

	return p.schedule(t, timeout, nil)
}

// ScheduleContext makes task to be scheduled over pool's workers.
//
// Non-nil error only available when ctx is done. That is, if no workers are
// available during lifetime of context, it returns ctx.Err() call result.
func (p *Pool) ScheduleContext(ctx context.Context, t Task) error {
	if err := p.schedule(t, nil, ctx.Done()); err != nil {
		return ctx.Err()
	}

	return nil
}

// ScheduleCustom makes task to be scheduled over pool's workers.
//
// Non-nil error only available when cancel closed. That is, if no workers are
// available until closure of cancel chan, it returns error.
//
// This method useful for implementing custom cancellation logic by a caller.
func (p *Pool) ScheduleCustom(cancel <-chan struct{}, t Task) error {
	return p.schedule(t, nil, cancel)
}

// Barrier blocks until workers complete their current task.
func (p *Pool) Barrier() {
	_ = p.barrier(nil, nil)
}

// BarrierTimeout blocks until workers complete their current task or until
// timeout is expired.
func (p *Pool) BarrierTimeout(tm time.Duration) error {
	var timeout <-chan time.Time

	if tm != 0 {
		timer := ptime.AcquireTimer(tm)
		defer ptime.ReleaseTimer(timer)
		timeout = timer.C
	}

	return p.barrier(timeout, nil)
}

// BarrierContext blocks until workers complete their current task or until
// context is done. In case when err is non-nil err is always a result of
// ctx.Err() call.
func (p *Pool) BarrierContext(ctx context.Context) error {
	if err := p.barrier(nil, ctx.Done()); err != nil {
		return ctx.Err()
	}

	return nil
}

// BarrierCustom blocks until workers complete their current task or until
// given cancelation channel is non-empty.
func (p *Pool) BarrierCustom(cancel <-chan struct{}) error {
	return p.barrier(nil, cancel)
}

// Wait blocks until all previously scheduled tasks are executed.
func (p *Pool) Wait() {
	_ = p.wait(nil, nil)
}

// WaitTimeout blocks until all previously scheduled tasks are executed or until
// timeout is expired.
func (p *Pool) WaitTimeout(tm time.Duration) error {
	var timeout <-chan time.Time

	if tm != 0 {
		timer := ptime.AcquireTimer(tm)
		defer ptime.ReleaseTimer(timer)
		timeout = timer.C
	}

	return p.wait(timeout, nil)
}

// WaitContext blocks until all previously scheduled tasks are executed or
// until context is done. In case when err is non-nil err is always a result of
// ctx.Err() call.
func (p *Pool) WaitContext(ctx context.Context) error {
	if err := p.wait(nil, ctx.Done()); err != nil {
		return ctx.Err()
	}

	return nil
}

// WaitCustom blocks until all previously scheduled tasks are executed or until
// given cancelation channel is non-empty.
func (p *Pool) WaitCustom(cancel <-chan struct{}) error {
	return p.wait(nil, cancel)
}

// workers returns current alive workers list.
func (p *Pool) workers() []*worker {
	p.mu.RLock()
	ws := make([]*worker, 0, p.list.Len())

	for el := p.list.Front(); el != nil; el = el.Next() {
		ws = append(ws, el.Value.(*worker))
	}

	p.mu.RUnlock()

	return ws
}

// barrier spreads a technical task over all running workers and blocks until
// every worker execute it. It returns earlier with non-nil error if timeout or
// done channels are non-empty.
func (p *Pool) barrier(timeout <-chan time.Time, done <-chan struct{}) error {
	snap := make(chan struct{}, 1)
	n, err := p.multicast(TaskFunc(func() {
		select {
		case snap <- struct{}{}:
		case <-timeout:
		case <-done:
		}
	}), timeout, done)

	if err != nil {
		return err
	}

	for i := 0; i < n; i++ {
		select {
		case <-snap:
		case <-timeout:
			return ErrUnavailable
		case <-done:
			return ErrUnavailable
		}
	}

	return nil
}

// multicast makes task to be executed on every running worker.
// It returns number of workers received the task and an error that could arise
// only when timeout or done channels are non-nil and non-empty.
//
// Note that if pool is closed it does not returns ErrPoolClosed error.
//
// If some worker can not receive incoming task until timeout or done chanel it
// stops trying to send the task to remaining workers.
func (p *Pool) multicast(task Task, timeout <-chan time.Time, done <-chan struct{}) (n int, err error) {
	// NOTE: if for some reasons it is neccessary to check that pool is closed
	// and return ErrPoolClosed, then you MUST edit the barrier() method such
	// that it will check the ErrPoolClosed case and make p.Done() waiting or
	// its timeout or done channels closure.
	// You will also need to edit the worker run() method, which is also rely
	// on this behaviour iniside kill case.
	for _, w := range p.workers() {
		callMaybe(p.config.OnTaskIn)
		t := p.wrapTask(task)

		switch {
		case timeout == immediate:
			select {
			case w.direct <- t:
				n++
			case <-w.done:
				// Worker killed.
			default:
				return n, ErrUnavailable
			}
		default:
			select {
			case w.direct <- t:
				n++
			case <-w.done:
				// Worker killed.
			case <-timeout:
				return n, ErrUnavailable
			case <-done:
				return n, ErrUnavailable
			}
		}
	}

	return n, nil
}

func (p *Pool) schedule(task Task, timeout <-chan time.Time, done <-chan struct{}) (err error) {
	callMaybe(p.config.OnTaskIn)
	w := p.wrapTask(task)

	// First, try to put task into the work queue in optimistic manner.
	select {
	case <-p.kill:
		// Drop it after Close() call
		callMaybe(p.config.OnTaskOut)
		return ErrPoolClosed
	case p.work <- w:
		return nil
	default:
	}

	switch {
	case p.sem == nil:
		// If pool is configured to be "rubber" we just spawn worker
		// without limits.
	case timeout == immediate:
		// Immediate timeout means that caller dont want to wait for
		// scheduling. In that case our only option here is to try to spawn
		// goroutine in non-blocking mode.
		select {
		case p.sem <- struct{}{}:
		default:
			err = ErrUnavailable
		}
	default:
		// If pool is not "rubber", try to enqueue task into the work queue or
		// spawn new worker until the timeout or done channel are filled.
		select {
		case p.work <- w:
			return nil
		case p.sem <- struct{}{}:
		case <-timeout:
			err = ErrUnavailable
		case <-done:
			err = ErrUnavailable
		case <-p.kill:
			err = ErrPoolClosed
		}
	}

	if err == nil {
		// We get here only if p.sem is nil or write to p.sem succeed.
		err = p.spawn(w, true)
	}

	if err != nil {
		callMaybe(p.config.OnTaskOut)
	}

	return err
}

func (p *Pool) wait(timeout <-chan time.Time, done <-chan struct{}) error {
	crossed := make(chan struct{})
	// Execution of this task means that all previous tasks from the work queue
	// were received by worker(s). After that, we only need to wait them to
	// finish their current labor.
	bound := TaskFunc(func() {
		close(crossed)
	})
	// Put the task manually to the queue without using schedule() method. This
	// is done to avoid side-effects of schedule() like spawning extra worker
	// which will execute this task out of queue order.
	select {
	case p.work <- bound:
		// OK.
	case <-p.done:
		// No need to do additional work – pool closed and workers are done.
		return nil

	case <-timeout:
		return ErrUnavailable
	case <-done:
		return ErrUnavailable
	}
	select {
	case <-crossed:
		return p.barrier(timeout, done)

	case <-timeout:
		return ErrUnavailable
	case <-done:
		return ErrUnavailable
	}
}

func (p *Pool) spawn(t Task, extra bool) (closed error) {
	// Prepare worker before taking the mutex.
	// That is we act here in optimistic manner that pool is not closed.
	w := &worker{
		direct: make(chan Task, 1),
		done:   make(chan struct{}),
		work:   p.work,
		kill:   p.kill,
		noCork: p.noCork,
		// idleTiming: p.config.IdleTiming,
	}
	if extra {
		w.ttl = p.config.ExtraWorkerTTL
	}

	// We must synchronize workers wait group to avoid panic "WaitGroup is
	// reused before previous Wait has returned". That is, there could be race
	// when we spawning worker while pool is closing; in that case inside
	// Close(): wg.Done() is not yet returned but its counter could reach the
	// zero due to all previous workers are done; here inside spawn(): we can
	// call wg.Add(1), which will produce the panic inside wg.Wait().
	p.mu.Lock()
	{
		select {
		case <-p.kill:
			closed = ErrPoolClosed
		default:
			p.wg.Add(1)
			w.elem = p.list.PushBack(w)
		}
	}
	p.mu.Unlock()

	if closed != nil {
		if p.sem != nil {
			// We can release token from the workers semaphore not under the
			// mutex.
			<-p.sem
		}

		return closed
	}

	w.onStop = func() {
		if p.sem != nil {
			<-p.sem
		}

		p.wg.Done()
		callMaybe(p.config.OnWorkerStop)

		p.mu.Lock()
		p.list.Remove(w.elem)
		p.mu.Unlock()
	}
	w.onStart = func() {
		callMaybe(p.config.OnWorkerStart)
	}

	go w.run(t)

	return nil
}

func (p *Pool) wrapTask(task Task) *taskWrapper {
	w := getTaskWrapper()

	w.task = task

	// w.queueTimer = p.config.QueueTiming.Timer()
	// w.execTimer = p.config.ExecTiming.Timer()

	// w.queueTimer.Start()
	w.onDequeue = func() {
		// w.queueTimer.Stop()
		p.config.OnTaskOut()
	}

	return w
}

// Schedule is a helper that returns function which purpose is to schedule
// execution of next given function over p.
func Schedule(p *Pool) func(func()) {
	return func(fn func()) {
		_ = p.Schedule(TaskFunc(fn))
	}
}

// ScheduleTimeout is a helper that returns function which purpose is to
// schedule execution of next given function over p with timeout.
func ScheduleTimeout(p *Pool) func(time.Duration, func()) error {
	return func(t time.Duration, fn func()) error {
		return p.ScheduleTimeout(t, TaskFunc(fn))
	}
}

// ScheduleTimeout is a helper that returns function which purpose is to
// schedule execution of next given function over p with context.
func ScheduleContext(p *Pool) func(context.Context, func()) error {
	return func(ctx context.Context, fn func()) error {
		return p.ScheduleContext(ctx, TaskFunc(fn))
	}
}

// ScheduleTimeout is a helper that returns function which purpose is to
// schedule execution of next given function over p with cancellation chan.
func ScheduleCustom(p *Pool) func(chan struct{}, func()) error {
	return func(ch chan struct{}, fn func()) error {
		return p.ScheduleCustom(ch, TaskFunc(fn))
	}
}

type worker struct {
	elem *list.Element

	work <-chan Task
	kill <-chan struct{}

	direct chan Task
	done   chan struct{}

	ttl time.Duration
	// idleTiming stat.SimpleTiming
	noCork bool

	onStop  func()
	onStart func()
}

func (w *worker) run(t Task) {
	defer func() {
		close(w.done)
		w.onStop()
	}()
	w.onStart()

	if t != nil {
		t.Run()
	}

	var timeout <-chan time.Time

	if w.ttl != 0 {
		tm := ptime.AcquireTimer(w.ttl)
		defer ptime.ReleaseTimer(tm)
		timeout = tm.C
	}

	// idle := w.idleTiming.Timer()
	// idle.Start()
	/// XXX: wrap Stop with func, because of receiver is defined on moment of evaluation
	// defer func() { idle.Stop() }()

	run := func(t Task) {
		// idle.Stop()
		// XXX: spawn new timer, because realtime timer should be used only once
		// idle = w.idleTiming.Timer()
		// idle.Start()
		t.Run()
	}

	for {
		select {
		case t := <-w.direct:
			run(t)
		case t := <-w.work:
			run(t)
		case <-timeout:
			// Cork direct work queue because it could contain buffered tasks,
			// which execution can block the user.
			cork(w.direct)

			return
		case <-w.kill:
			// Receving from w.kill means that pool is closing and no more
			// tasks will be queued soon.
			if w.noCork {
				// Drop everything
				return
			}

			// We can give a last chance to read some tasks from work queue and
			// reduce the pressure on Close() calling goroutine by reducing
			// amount of possible tasks executed during cork(p.work) there.
			runtime.Gosched()

			for {
				// We do not handle here w.direct due no pool closure checking
				// iniside multicast(). That is, multicast() tries to send
				// direct message for every worker listed in p.list. Reading
				// messages from w.direct inside this loop could produce
				// infinite living worker if client calls multicast() in a
				// loop. Thats why we just cork(w.direct) below.
				select {
				case t := <-w.work:
					// No idle timer usage here.
					t.Run()
				default:
					cork(w.direct)
					return
				}
			}
		}
	}
}

type taskWrapper struct {
	task Task
	// queueTimer stat.Timer
	// execTimer  stat.Timer
	onDequeue func()
}

var taskWrapperPool sync.Pool

func getTaskWrapper() *taskWrapper {
	if w, _ := taskWrapperPool.Get().(*taskWrapper); w != nil {
		return w
	}

	return &taskWrapper{}
}

func putTaskWrapper(w *taskWrapper) {
	*w = taskWrapper{}
	taskWrapperPool.Put(w)
}

func (w *taskWrapper) Run() {
	w.onDequeue()

	// w.execTimer.Start()
	w.task.Run()
	// w.execTimer.Stop()

	putTaskWrapper(w)
}

func callMaybe(fn func()) {
	if fn != nil {
		fn()
	}
}

// cork fulfills given work channel with stub tasks. It executes every task
// found in queue during corking. It returns when all tasks in queue are stubs.
//
// Note that no one goroutine must own the work channel while calling cork.
// In other case it could lead to spinlock.
func cork(work chan Task) {
	n := cap(work)
	for i := 0; i != n; {
	corking:
		for i = 0; i < n; i++ {
			select {
			case work <- stubTask:
			default:
				// Blocked to write due to some lost task in the queue.
				break corking
			}
		}
		// Drain the work queue and execute non stub tasks that stucked in the
		// queue after Close() call.
		//
		// NOTE: it runs only when we does not fulfilled work queue with stubs
		//       in loop from above (i != n).
		for j := 0; i != n && j < n; j++ {
			w := <-work
			if w != stubTask {
				w.Run()
			}
		}
	}
}

type panicTask struct{}

func (s panicTask) Run() {
	panic("pool: this task must never be executed")
}

var stubTask = panicTask{}
