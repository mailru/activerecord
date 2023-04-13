package config

import (
	"runtime"
	"time"

	"github.com/mailru/activerecord/pkg/iproto/util/pool"
)

// Config describes an object that is capable to create configuration variables
// which could be changed from outside somehow.
type Config interface {
	Int(string, int, string, ...func(int) error) *int
	Duration(string, time.Duration, string, ...func(time.Duration) error) *time.Duration
}

// Stat describes an object that is the same as Config but with stat
// additional methods.
type Stat interface {
	Config
	Float64Slice(string, []float64, string, ...func([]float64) error) *[]float64
}

func Export(config Config, prefix string) func() *pool.Config {
	prefix = sanitize(prefix)

	var (
		unstoppableWorkers = config.Int(
			prefix+"pool.unstoppable_workers", 1,
			"number of always running workers",
		)
		maxWorkers = config.Int(
			prefix+"pool.max_workers", runtime.NumCPU(),
			"total number of workers that could be spawned",
		)
		extraWorkerTTL = config.Duration(
			prefix+"pool.extra_worker_ttl", pool.DefaultExtraWorkerTTL,
			"time to live for extra spawnd workers",
		)
	)

	workQueueSize := config.Int(
		prefix+"pool.work_queue_size", 0,
		"work queue size",
	)

	return func() *pool.Config {
		return &pool.Config{
			UnstoppableWorkers: *unstoppableWorkers,
			MaxWorkers:         *maxWorkers,
			ExtraWorkerTTL:     *extraWorkerTTL,
			WorkQueueSize:      *workQueueSize,
		}
	}
}

func sanitize(p string) string {
	if n := len(p); n != 0 {
		if p[n-1] != '.' {
			return p + "."
		}
	}

	return p
}
