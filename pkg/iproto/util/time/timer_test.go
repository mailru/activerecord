package time

import (
	"runtime"
	"testing"
	"time"
)

func TestTimerPool(t *testing.T) {
	for i := 0; i < 1000000; i++ {
		if i%2 == 0 {
			tm := AcquireTimer(0)
			ReleaseTimer(tm)
			continue
		}

		tm := AcquireTimer(time.Second)
		select {
		case <-tm.C:
			t.Fatalf("unexpected timer event after %d iterations!", i)
		default:
			ReleaseTimer(tm)
		}
	}
}

func BenchmarkTimerPool(b *testing.B) {
	b.SetParallelism(1024)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tm := AcquireTimer(0)
			runtime.Gosched()
			ReleaseTimer(tm)
		}
	})
}
