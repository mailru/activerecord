package time

import "time"

type MonotonicTimestamp timestamp

func (m MonotonicTimestamp) Add(d time.Duration) MonotonicTimestamp {
	return MonotonicTimestamp(timestamp(m).add(d))
}

func (m MonotonicTimestamp) Sub(u MonotonicTimestamp) time.Duration {
	return timestamp(m).sub(timestamp(u))
}

func (m MonotonicTimestamp) Equal(u MonotonicTimestamp) bool {
	return timestamp(m).equal(timestamp(u))
}

func (m MonotonicTimestamp) Before(u MonotonicTimestamp) bool {
	return timestamp(m).before(timestamp(u))
}

func (m MonotonicTimestamp) After(u MonotonicTimestamp) bool {
	return timestamp(m).after(timestamp(u))
}
