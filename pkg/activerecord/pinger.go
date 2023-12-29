package activerecord

import (
	"context"
	"errors"
	"runtime/debug"
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

func WithConfigCache(configCache ConfigCacherInterface) OptionPinger {
	return OptionPingerFunc(func(p *Pinger) {
		p.configCache = configCache
	})
}

type Pinger struct {
	ctx         context.Context
	cancel      context.CancelFunc
	pingers     map[string]func(ctx context.Context, instance ShardInstance) (ServerModeType, error)
	eg          *errgroup.Group
	started     bool
	interval    time.Duration
	ticker      *time.Ticker
	logger      LoggerInterface
	configCache ConfigCacherInterface
}

func NewPinger(opts ...OptionPinger) *Pinger {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Pinger{
		ctx:      ctx,
		cancel:   cancel,
		pingers:  map[string]func(ctx context.Context, instance ShardInstance) (ServerModeType, error){},
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
				start := time.Now().UnixMilli()
				p.logger.Warn(ctx, "starting ping")

				for cfgPath, ping := range p.pingers {
					clusterConf, warn := p.clusterConfig().Actualize(ctx, cfgPath, ping)
					if warn != nil {
						p.logger.Warn(p.ctx, warn)
					}

					p.log(ctx, clusterConf)
				}
				p.logger.Info(ctx, "ping finished after ", time.Now().UnixMilli()-start, " ms")
			}
		}

		return err
	})
	p.logger.Info(ctx, "start instance watcher")
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

// AddClusterChecker добавляет в локальный пингер конфигурации кластера, актуализируя типы и доступность узлов
func (p *Pinger) AddClusterChecker(ctx context.Context, path string, ping func(ctx context.Context, instance ShardInstance) (ServerModeType, error)) (*Cluster, error) {
	_, ok := p.pingers[path]

	if !ok {
		p.pingers[path] = ping

		// если пингер для конфигурации кластер не зарегистрировался ранее (конфигурация загружена впервые)
		// актуализируем конфигурацию кластера
		clusterConf, err := p.clusterConfig().Actualize(ctx, path, ping)
		if err != nil {
			return nil, err
		}

		p.log(ctx, clusterConf)

		p.StartWatch(p.ctx)

		return clusterConf, nil
	}

	return nil, nil
}

func (p *Pinger) clusterConfig() ConfigCacherInterface {
	if p.configCache != nil {
		return p.configCache
	}

	return ConfigCacher()
}

func (p *Pinger) log(ctx context.Context, clusterConf *Cluster) {
	for i := 0; i < clusterConf.Len(); i++ {
		shard := clusterConf.Shard(i)
		for _, shardInstance := range append(shard.Masters, shard.Replicas...) {
			if !shardInstance.Offline {
				continue
			}

			switch shardInstance.Config.Mode {
			case ModeMaster:
				p.logger.Warn(ctx, "master:", shardInstance.Config.Addr, "is unavailable")
			case ModeReplica:
				p.logger.Warn(ctx, "replica:", shardInstance.Config.Addr, "is unavailable")
			}

		}
	}
}
