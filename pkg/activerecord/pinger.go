package activerecord

import (
	"context"
	"errors"
	"runtime/debug"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type OptionPingerFunc func(*Pinger)

type OptionPinger interface {
	apply(pinger *Pinger)
}

func (o OptionPingerFunc) apply(pinger *Pinger) {
	o(pinger)
}

func WithPingInterval(interval time.Duration) OptionPinger {
	return OptionPingerFunc(func(p *Pinger) {
		p.interval = interval
	})
}

func WithStart() OptionPinger {
	return OptionPingerFunc(func(p *Pinger) {
		p.StartWatch(p.ctx)
	})
}

type Pinger struct {
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	pingers  map[string]func(ctx context.Context) error
	eg       *errgroup.Group
	started  bool
	interval time.Duration
	ticker   *time.Ticker
	logger   LoggerInterface
}

func NewPinger(opts ...OptionPinger) *Pinger {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Pinger{
		ctx:      ctx,
		cancel:   cancel,
		mu:       sync.Mutex{},
		pingers:  map[string]func(ctx context.Context) error{},
		eg:       &errgroup.Group{},
		interval: time.Second,
		logger:   NewLogger(),
	}

	for _, opt := range opts {
		opt.apply(p)
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

		defer func() {
			r := recover()
			if r != nil {
				p.logger.Error(p.ctx, "unexpected pinger watch panic:", string(debug.Stack()))
			}
		}()

		for err == nil {
			select {
			case <-ctx.Done():
				err = ctx.Err()
				continue
			case <-t.C:
				for _, ping := range p.pingers {
					if warn := ping(ctx); warn != nil {
						p.logger.Warn(p.ctx, warn)
					}
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

	err := p.eg.Wait()
	if errors.Is(err, context.Canceled) {
		return nil
	}

	return err
}

func (p *Pinger) Add(ctx context.Context, path string, ping func(ctx context.Context) error) bool {
	if p == nil {
		return false
	}

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
