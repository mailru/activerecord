package syncutil

import (
	"sync"
	"time"
)

// NewThrottle creates new Throttle with given period.
func NewThrottle(p time.Duration) *Throttle {
	return &Throttle{
		period: p,
	}
}

// Throttle helps to run a function only a once per given time period.
type Throttle struct {
	mu     sync.RWMutex
	period time.Duration
	last   time.Time
}

// Do executes fn if Throttle's last execution time is far enough in the past.
func (t *Throttle) Next() (ok bool) {
	now := time.Now()

	t.mu.RLock()

	ok = now.Sub(t.last) >= t.period
	t.mu.RUnlock()

	if !ok {
		return
	}

	t.mu.Lock()

	ok = now.Sub(t.last) >= t.period
	if ok {
		t.last = now
	}

	t.mu.Unlock()

	return
}

// Reset resets the throttle timeout such that next Next() will return true.
func (t *Throttle) Reset() {
	t.mu.Lock()
	t.last = time.Time{}
	t.mu.Unlock()
}

// Set sets throttle point such that Next() will return true only after given
// moment p.
func (t *Throttle) Set(p time.Time) {
	t.mu.Lock()
	t.last = p.Add(-t.period)
	t.mu.Unlock()
}
