package tarantool

import (
	"fmt"
	"hash/crc32"
	"time"

	"github.com/mailru/activerecord/pkg/activerecord"
	"github.com/tarantool/go-tarantool"
)

const DefaultConnectionTimeout = 20 * time.Millisecond

type ConnectionOptions struct {
	*activerecord.GroupHash
	cfg    tarantool.Opts
	server string
	Mode   activerecord.ServerModeType
}

type ConnectionOption interface {
	apply(*ConnectionOptions) error
}

type optionConnectionFunc func(*ConnectionOptions) error

func (o optionConnectionFunc) apply(c *ConnectionOptions) error {
	return o(c)
}

// WithTimeout - опция для изменений таймаутов
func WithTimeout(request time.Duration) ConnectionOption {
	return optionConnectionFunc(func(opts *ConnectionOptions) error {
		opts.cfg.Timeout = request

		return opts.UpdateHash("T", request)
	})
}

// WithCredential - опция авторизации
func WithCredential(user, pass string) ConnectionOption {
	return optionConnectionFunc(func(opts *ConnectionOptions) error {
		opts.cfg.User = user
		opts.cfg.Pass = pass

		return opts.UpdateHash("L", user, pass)
	})
}

func NewOptions(server string, mode activerecord.ServerModeType, opts ...ConnectionOption) (*ConnectionOptions, error) {
	if server == "" {
		return nil, fmt.Errorf("invalid param: server is empty")
	}

	connectionOpts := &ConnectionOptions{
		cfg: tarantool.Opts{
			Timeout: DefaultConnectionTimeout,
		},
		server: server,
		Mode:   mode,
	}

	connectionOpts.GroupHash = activerecord.NewGroupHash(crc32.NewIEEE())

	for _, opt := range opts {
		if err := opt.apply(connectionOpts); err != nil {
			return nil, fmt.Errorf("error apply options: %w", err)
		}
	}

	err := connectionOpts.UpdateHash("S", server)
	if err != nil {
		return nil, fmt.Errorf("can't get pool: %w", err)
	}

	return connectionOpts, nil
}

func (c *ConnectionOptions) GetConnectionID() string {
	return c.GetHash()
}

func (c *ConnectionOptions) InstanceMode() activerecord.ServerModeType {
	return c.Mode
}
