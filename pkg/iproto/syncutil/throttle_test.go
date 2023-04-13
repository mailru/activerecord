package syncutil

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestThrottleDo(t *testing.T) {
	t.Skip("Fails on centos")
	th := NewThrottle(time.Millisecond * 50)

	var (
		wg sync.WaitGroup
		n  int32
	)
	for i := 0; i < runtime.NumCPU(); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tick := time.Tick(time.Millisecond * 5)
			for j := 0; j < 100; j++ {
				<-tick
				if th.Next() {
					atomic.AddInt32(&n, 1)
				}
			}
		}()
	}
	wg.Wait()

	if act, exp := int(atomic.LoadInt32(&n)), 10; act != exp {
		t.Errorf("got %d truly Next(); want %d", act, exp)
	}
}

func BenchmarkThrottleDo(b *testing.B) {
	th := NewThrottle(time.Millisecond * 10)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			th.Next()
		}
	})
}
