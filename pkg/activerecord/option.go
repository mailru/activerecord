package activerecord

type Option interface {
	apply(*ActiveRecord)
}

type optionFunc func(*ActiveRecord)

func (o optionFunc) apply(c *ActiveRecord) {
	o(c)
}

func WithLogger(logger LoggerInterface) Option {
	return optionFunc(func(a *ActiveRecord) {
		a.logger = logger
	})
}

func WithConfig(config ConfigInterface) Option {
	return optionFunc(func(a *ActiveRecord) {
		a.config = config
	})
}

func WithMetrics(metric MetricInterface) Option {
	return optionFunc(func(a *ActiveRecord) {
		a.metric = metric
	})
}

type clusterOption interface {
	apply(*Cluster)
}

type clusterOptionFunc func(*Cluster)

func (o clusterOptionFunc) apply(c *Cluster) {
	o(c)
}

func WithShard(masters []OptionInterface, replicas []OptionInterface) clusterOption {
	return clusterOptionFunc(func(c *Cluster) {
		newShard := Shard{}

		for _, opt := range masters {
			newShard.Masters = append(newShard.Masters, ShardInstance{
				ParamsID: opt.GetConnectionID(),
				Config:   ShardInstanceConfig{Addr: "static"},
				Options:  opt,
			})
		}

		*c = append(*c, newShard)
	})
}
