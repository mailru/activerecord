// Package time contains tools for time manipulation.
package time

import "time"

const (
	minDuration time.Duration = -1 << 63
	maxDuration time.Duration = 1<<63 - 1
)

type timestamp struct {
	// sec gives the number of seconds elapsed since some time point
	// regarding the type of timestamp.
	sec int64

	// nsec specifies a non-negative nanosecond offset within the seconds.
	// It must be in the range [0, 999999999].
	nsec int32
}

// add returns the timestamp t+d.
func (t timestamp) add(d time.Duration) timestamp {
	t.sec += int64(d / 1e9)

	nsec := t.nsec + int32(d%1e9)
	if nsec >= 1e9 {
		t.sec++

		nsec -= 1e9
	} else if nsec < 0 {
		t.sec--

		nsec += 1e9
	}

	t.nsec = nsec

	return t
}

// sub return duration t-u.
func (t timestamp) sub(u timestamp) time.Duration {
	d := time.Duration(t.sec-u.sec)*time.Second + time.Duration(int32(t.nsec)-int32(u.nsec))
	// Check for overflow or underflow.
	switch {
	case u.add(d).equal(t):
		return d // d is correct
	case t.before(u):
		return minDuration // t - u is negative out of range
	default:
		return maxDuration // t - u is positive out of range
	}
}

// equal reports whether the t is equal to u.
func (t timestamp) equal(u timestamp) bool {
	return t.sec == u.sec && t.nsec == u.nsec
}

// equal reports whether the t is before u.
func (t timestamp) before(u timestamp) bool {
	return t.sec < u.sec || t.sec == u.sec && t.nsec < u.nsec
}

// after reports whether the t is after u.
func (t timestamp) after(u timestamp) bool {
	return t.sec > u.sec || t.sec == u.sec && t.nsec > u.nsec
}
