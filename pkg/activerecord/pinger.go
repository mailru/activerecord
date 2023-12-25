package activerecord

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type Pinger struct {
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	pingers  map[string]func(ctx context.Context) error
	eg       *errgroup.Group
	started  bool
	interval time.Duration
	ticker   *time.Ticker
}

func NewPinger(interval time.Duration, start bool) *Pinger {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Pinger{
		ctx:      ctx,
		cancel:   cancel,
		mu:       sync.Mutex{},
		pingers:  map[string]func(ctx context.Context) error{},
		eg:       &errgroup.Group{},
		started:  false,
		interval: interval,
	}

	if start {
		p.StartWatch(p.ctx)
	}

	return p
}

func (p *Pinger) isStarted() bool {
	return p.started
}

func (p *Pinger) StartWatch(ctx context.Context) {
	if p.isStarted() {
		return
	}

	t := time.NewTicker(p.interval)

	p.eg.Go(func() error {
		var err error

		for err == nil {
			select {
			case <-ctx.Done():
				err = ctx.Err()
				continue
			case <-t.C:
				for _, ping := range p.pingers {
					err = ping(ctx)
				}
			}
		}

		return err
	})

	p.ticker = t
	p.started = true
}

func (p *Pinger) StopWatch() error {
	if !p.isStarted() {
		return nil
	}

	defer func() {
		p.started = false
	}()

	p.cancel()
	p.ticker.Stop()

	return p.eg.Wait()
}

func (p *Pinger) Add(ctx context.Context, path string, ping func(ctx context.Context) error) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ok := p.pingers[path]
	if !ok {
		p.pingers[path] = ping

		p.StartWatch(p.ctx)

		return true
	}

	return false
}
