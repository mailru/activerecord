package time

import "time"

// Monotonic returns nanoseconds passed from some point in past.
// Note that returned value is not persistent. That is, returned value is no longer actual after system restart.
func Monotonic() (MonotonicTimestamp, error) {
	now := time.Now()
	sec := now.Unix()
	nsec := int32(now.UnixNano() - sec*1e9)
	return MonotonicTimestamp{
		sec:  sec,
		nsec: nsec,
	}, nil
}
