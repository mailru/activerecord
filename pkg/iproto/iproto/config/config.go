package config

import (
	"time"

	"github.com/mailru/activerecord/pkg/iproto/iproto"
	"github.com/mailru/activerecord/pkg/iproto/util/pool"

	"golang.org/x/time/rate"
)

// Config describes an object that is capable to create configuration variables
// which could be changed from outside somehow.
type Config interface {
	Duration(string, time.Duration, string, ...func(time.Duration) error) *time.Duration
	Int(string, int, string, ...func(int) error) *int
	Uint32(string, uint32, string, ...func(uint32) error) *uint32
	Bool(string, bool, string, ...func(bool) error) *bool
	Float64(string, float64, string, ...func(float64) error) *float64
}

// ExportPoolConfig exports flags with given prefix for iproto pool configuration.
// It returns factory of iproto.PoolConfig which will be filled with flags values.
func ExportPoolConfig(config Config, prefix string) func() *iproto.PoolConfig {
	prefix = sanitize(prefix)

	var (
		connectTimeout = config.Duration(
			prefix+"iproto.connect_timeout", iproto.DefaultPoolConnectTimeout,
			"connect timeout",
		)
		dialTimeout = config.Duration(
			prefix+"iproto.dial_timeout", 0,
			"single dial timeout",
		)
		poolSize = config.Int(
			prefix+"iproto.pool_size", iproto.DefaultPoolSize,
			"connection pool size",
		)
		disablePredial = config.Bool(
			prefix+"iproto.disable_pre_dial", false,
			"if true, pool will not try to establish a new connection right after one is closed",
		)
		redialInterval = config.Duration(
			prefix+"iproto.redial_interval", iproto.DefaultPoolRedialInterval,
			"time to wait before redialing",
		)
		redialTimeout = config.Duration(
			prefix+"iproto.redial_timeout", iproto.DefaultPoolRedialTimeout,
			"timeout for redialing routine",
		)
		redialForever = config.Bool(
			prefix+"iproto.redial_forever", false,
			"do not use any timeout for redialing routine",
		)
		maxRedialInterval = config.Duration(
			prefix+"iproto.max_redial_interval", 0,
			"max time to wait before redialing",
		)
		failOnCutoff = config.Bool(
			prefix+"iproto.fail_on_cutoff", false,
			"fail fast on calls when there are no online connections in the pool",
		)
		dialThrottle = config.Duration(
			prefix+"iproto.dial_throttle", 0,
			"dial attempts throttling period",
		)
		rateLimit = config.Float64(
			prefix+"iproto.rate_limit", float64(rate.Inf),
			"per-peer rate limit in requests per second",
		)
		rateBurst = config.Int(
			prefix+"iproto.rate_burst", 0,
			"per-peer allowed request burst size",
		)
		rateWait = config.Bool(
			prefix+"iproto.rate_wait", false,
			"enabled request rate shaping instead of policing",
		)
	)

	channelConfig := ExportChannelConfig(config, prefix)

	return func() *iproto.PoolConfig {
		return &iproto.PoolConfig{
			Size:              *poolSize,
			DialTimeout:       *dialTimeout,
			ConnectTimeout:    *connectTimeout,
			DisablePreDial:    *disablePredial,
			RedialInterval:    *redialInterval,
			RedialTimeout:     *redialTimeout,
			RedialForever:     *redialForever,
			MaxRedialInterval: *maxRedialInterval,
			FailOnCutoff:      *failOnCutoff,
			DialThrottle:      *dialThrottle,
			RateLimit:         rate.Limit(*rateLimit),
			RateBurst:         *rateBurst,
			RateWait:          *rateWait,
			ChannelConfig:     channelConfig(),
		}
	}
}

// ExportChannelConfig exports flags with given prefix for iproto channel configuration.
// It returns factory of iproto.ChannelConfig which will be filled with flags values.
func ExportChannelConfig(config Config, prefix string) func() *iproto.ChannelConfig {
	prefix = sanitize(prefix)

	var (
		packetSizeLimit = config.Uint32(
			prefix+"iproto.packet_size_limit", iproto.DefaultSizeLimit,
			"maximum packet size in bytes",
		)
		writeQueueSize = config.Int(
			prefix+"iproto.write_queue_size", iproto.DefaultWriteQueueSize,
			"maximum number of buffered packets to write",
		)
		writeBufferSize = config.Int(
			prefix+"iproto.write_buffer_size", iproto.DefaultWriteBufferSize,
			"write buffer size in bytes",
		)
		writeTimeout = config.Duration(
			prefix+"iproto.write_timeout", iproto.DefaultWriteTimeout,
			"write timeout",
		)
		readBufferSize = config.Int(
			prefix+"iproto.read_buffer_size", iproto.DefaultReadBufferSize,
			"read buffer size in bytes",
		)
		disablePing = config.Bool(
			prefix+"iproto.disable_ping", false,
			"ping disable",
		)
		requestTimeout = config.Duration(
			prefix+"iproto.request_timeout", iproto.DefaultRequestTimeout,
			"request timeout",
		)
		noticeTimeout = config.Duration(
			prefix+"iproto.notice_timeout", iproto.DefaultNoticeTimeout,
			"notice timeout",
		)
		idleTimeout = config.Duration(
			prefix+"iproto.idle_timeout", 0,
			"connection idle timeout (0 for infinity)",
		)
		pingInterval = config.Duration(
			prefix+"iproto.ping_interval", iproto.DefaultPingInterval,
			"ping interval",
		)
		shutdownTimeout = config.Duration(
			prefix+"iproto.shutdown_timeout", iproto.DefaultShutdownTimeout,
			"shutdown timeout",
		)
	)

	return func() *iproto.ChannelConfig {
		return &iproto.ChannelConfig{
			WriteQueueSize:  *writeQueueSize,
			WriteBufferSize: *writeBufferSize,
			ReadBufferSize:  *readBufferSize,
			WriteTimeout:    *writeTimeout,
			ShutdownTimeout: *shutdownTimeout,
			RequestTimeout:  *requestTimeout,
			NoticeTimeout:   *noticeTimeout,
			SizeLimit:       *packetSizeLimit,
			IdleTimeout:     *idleTimeout,
			PingInterval:    *pingInterval,
			DisablePing:     *disablePing,
		}
	}
}

func ExportPacketServerConfig(config Config, prefix string) func() *iproto.PacketServerConfig {
	prefix = sanitize(prefix)

	var (
		maxTransmissionUnit = config.Int(
			prefix+"iproto.dgram_max_transmission_unit", iproto.DefaultMaxTransmissionUnit,
			"maximum transmission unit",
		)
		writeQueueSize = config.Int(
			prefix+"iproto.dgram_write_queue_size", iproto.DefaultWriteQueueSize,
			"maximum number of buffered packets to write",
		)
		writeTimeout = config.Duration(
			prefix+"iproto.dgram_write_timeout", iproto.DefaultWriteTimeout,
			"write timeout",
		)
		readBufferSize = config.Int(
			prefix+"iproto.dgram_read_buffer_size", iproto.DefaultReadBufferSize,
			"read buffer size in bytes",
		)
		packetSizeLimit = config.Uint32(
			prefix+"iproto.dgram_packet_size_limit", iproto.DefaultSizeLimit,
			"maximum packet size in bytes",
		)
	)

	return func() *iproto.PacketServerConfig {
		return &iproto.PacketServerConfig{
			MaxTransmissionUnit: *maxTransmissionUnit,
			WriteQueueSize:      *writeQueueSize,
			WriteTimeout:        *writeTimeout,
			ReadBufferSize:      *readBufferSize,
			SizeLimit:           *packetSizeLimit,
		}
	}
}

func ExportParallelHandler(config Config, prefix string) func(iproto.Handler) iproto.Handler {
	prefix = sanitize(prefix)

	var (
		safe = configHandlerRecover(config, prefix)
		n    = config.Int(
			prefix+"iproto.handler.max_parallel_requests", 128,
			"maximum amount of packets being processed in parallel",
		)
	)

	return func(h iproto.Handler) iproto.Handler {
		if *safe {
			h = iproto.RecoverHandler(h)
		}

		if *n == 0 {
			return h
		}

		return iproto.ParallelHandler(h, *n)
	}
}

func ExportPoolHandler(config Config, prefix string, poolConfig func() *pool.Config) func(iproto.Handler) iproto.Handler {
	prefix = sanitize(prefix)

	var (
		safe = configHandlerRecover(config, prefix)
	)

	return func(h iproto.Handler) iproto.Handler {
		p := pool.Must(pool.New(poolConfig()))

		if *safe {
			h = iproto.RecoverHandler(h)
		}

		return iproto.PoolHandler(h, p)
	}
}

func ExportChannelConfigWithHandler(config Config, prefix string, wrapper func(iproto.Handler) iproto.Handler) func(iproto.Handler) *iproto.ChannelConfig {
	prefix = sanitize(prefix)

	var (
		channel = ExportChannelConfig(config, prefix)
	)

	return func(h iproto.Handler) *iproto.ChannelConfig {
		c := channel()
		c.Handler = wrapper(h)

		return c
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

func configHandlerRecover(config Config, prefix string) *bool {
	return config.Bool(
		prefix+"iproto.handler.recover", false,
		"recover after packet handling",
	)
}
