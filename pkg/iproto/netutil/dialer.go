package netutil

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/mailru/activerecord/pkg/iproto/syncutil"
	egotime "github.com/mailru/activerecord/pkg/iproto/util/time"
	"golang.org/x/net/context"
)

const DefaultLoopInterval = time.Millisecond * 50

var (
	ErrClosed = errors.New("dialer owner has been gone")
)

// BackgroundDialer is a wrapper around Dialer that contains logic of glueing
// and cancellation of dial requests.
type BackgroundDialer struct {
	mu       sync.Mutex
	timer    *time.Timer
	deadline time.Time

	Dialer     *Dialer
	TaskGroup  *syncutil.TaskGroup
	TaskRunner *syncutil.TaskRunner
}

// Dial begins dial routine if no one was started yet. It returns channel
// that signals about routine is done. If some routine was started before and
// not done yet, it returns done channel of that goroutine.
// It returns non-nil error only if dial routine was not started.
//
// Started routine could be cancelled by calling Cancel method.
//
// Note that cb is called only once. That is, if caller A calls Dial and caller
// B calls Dial immediately after, both of them will receive the same done
// channel, but only A's callback will be called in the end.
func (d *BackgroundDialer) Dial(ctx context.Context, cb func(net.Conn, error)) <-chan error {
	ps := d.TaskRunner.Do(ctx, func(ctx context.Context) error {
		conn, err := d.Dialer.Dial(ctx)
		cb(conn, err)
		return err
	})

	return ps
}

// Cancel stops current background dial routine.
func (d *BackgroundDialer) Cancel() {
	d.TaskRunner.Cancel()
}

// SetDeadline sets the dial deadline.
//
// A deadline is an absolute time after which all dial routines fail.
// The deadline applies to all future and pending dials, not just the
// immediately following call to Dial.
// Cancelling some routine by calling Cancel method will not affect deadline.
// After a deadline has been exceeded, the dialer can be refreshed by setting a
// deadline in the future.
//
// A zero value for t means dial routines will not time out.
func (d *BackgroundDialer) SetDeadline(t time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.setDeadline(t)
}

// SetDeadlineAtLeast sets the dial deadline if current deadline is zero or
// less than t. The other behaviour is the same as in SetDeadline.
//
// A zero value for t is ignored.
//
// It returns actual deadline value.
func (d *BackgroundDialer) SetDeadlineAtLeast(t time.Time) time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.deadline.Before(t) {
		d.setDeadline(t)
	}

	return d.deadline
}

// Mutex must be held.
func (d *BackgroundDialer) setDeadline(t time.Time) {
	d.deadline = t

	if t.IsZero() {
		if d.timer != nil {
			d.timer.Stop()
		}

		return
	}

	//nolint:gosimple
	tm := t.Sub(time.Now())
	if tm < 0 {
		tm = 0
	}

	if d.timer == nil {
		d.timer = time.AfterFunc(tm, d.Cancel)
	} else {
		// We do not check d.timer.Stop() here cause it is not a problem, if
		// deadline has been reached and some dialing routine was cancelled.
		d.timer.Reset(tm)
	}
}

// Dialer contains options for connecting to an address.
type Dialer struct {
	// Network and Addr are destination credentials.
	Network, Addr string

	// Timeout is the maximum amount of time a dial will wait for a single
	// connect to complete.
	Timeout time.Duration

	// LoopTimeout is the maximum amount of time a dial loop will wait for a
	// successful established connection. It may fail earlier if Closed option
	// is set or Cancel method is called.
	LoopTimeout time.Duration

	// LoopInterval is used to delay dial attepmts between each other.
	LoopInterval time.Duration

	// MaxLoopInterval is the maximum delay before next attempt to connect is
	// prepared. Note that LoopInterval is used as initial delay, and could be
	// increased by every dial attempt up to MaxLoopInterval.
	MaxLoopInterval time.Duration

	// Closed signals that Dialer owner is closed forever and will never want
	// to dial again.
	Closed chan struct{}

	// OnAttempt will be called with every dial attempt error. Nil error means
	// that dial succeed.
	OnAttempt func(error)

	// NetDial could be set to override dial function. By default net.Dial is
	// used.
	NetDial func(ctx context.Context, network, addr string) (net.Conn, error)

	// Logf could be set to receive log messages from Dialer.
	Logf   func(string, ...interface{})
	Debugf func(string, ...interface{})

	// DisableLogAddr removes addr part in log message prefix.
	DisableLogAddr bool
}

// Dial tries to connect until some of events occur:
// - successful connect;
// – ctx is cancelled;
// – dialer owner is closed;
// – loop timeout exceeded (if set);
func (d *Dialer) Dial(ctx context.Context) (conn net.Conn, err error) {
	var (
		maxInterval = d.MaxLoopInterval
		step        = d.LoopInterval
	)

	if step == 0 {
		step = DefaultLoopInterval
	}

	interval := step
	if maxInterval < interval {
		maxInterval = interval
	}

	loopTimer := egotime.AcquireTimer(interval)
	defer egotime.ReleaseTimer(loopTimer)

	if tm := d.LoopTimeout; tm != 0 {
		ctx, _ = context.WithTimeout(ctx, tm)
	}

	var attempts int

	for {
		d.debugf("dialing (%d)", attempts)
		attempts++

		conn, err = d.dial(ctx)
		if cb := d.OnAttempt; cb != nil {
			cb(err)
		}

		if err == nil {
			d.debugf("dial ok: local addr is %s", conn.LocalAddr().String())
			return
		}

		if ctx.Err() == nil {
			d.logf("dial error: %v; delaying next attempt for %s", err, interval)
		} else {
			d.logf("dial error: %v;", err)
		}

		select {
		case <-loopTimer.C:
			//
		case <-ctx.Done():
			err = ctx.Err()
			return
		case <-d.Closed:
			err = ErrClosed
			return
		}

		interval += step
		if interval > maxInterval {
			interval = maxInterval
		}

		loopTimer.Reset(interval)
	}
}

func (d *Dialer) dial(ctx context.Context) (conn net.Conn, err error) {
	if tm := d.Timeout; tm != 0 {
		ctx, _ = context.WithTimeout(ctx, tm)
	}

	netDial := d.NetDial
	if netDial == nil {
		netDial = defaultNetDial
	}

	return netDial(ctx, d.Network, d.Addr)
}

func (d *Dialer) getLogPrefix() string {
	if d.DisableLogAddr {
		return "dialer: "
	}

	return `dialer to "` + d.Network + `:` + d.Addr + `": `
}

func (d *Dialer) logf(fmt string, args ...interface{}) {
	prefix := d.getLogPrefix()

	if logf := d.Logf; logf != nil {
		logf(prefix+fmt, args...)
	}
}

func (d *Dialer) debugf(fmt string, args ...interface{}) {
	prefix := d.getLogPrefix()

	if debugf := d.Debugf; debugf != nil {
		debugf(prefix+fmt, args...)
	}
}

var emptyDialer net.Dialer

func defaultNetDial(ctx context.Context, network, addr string) (net.Conn, error) {
	return emptyDialer.DialContext(ctx, network, addr)
}
