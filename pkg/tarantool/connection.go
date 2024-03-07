package tarantool

import (
	"context"
	"fmt"

	"github.com/mailru/activerecord/pkg/activerecord"
	"github.com/tarantool/go-tarantool"
)

var DefaultConnectionParams = activerecord.MapGlobParam{
	Timeout: DefaultConnectionTimeout,
}

type Connection struct {
	*tarantool.Connection
	opts *ConnectionOptions
}

func GetConnection(_ context.Context, opts *ConnectionOptions) (*Connection, error) {
	conn, err := tarantool.Connect(opts.server, opts.cfg)
	if err != nil {
		return nil, fmt.Errorf("error connect to tarantool %s with connect timeout '%d': %s", opts.server, opts.cfg.Timeout, err)
	}

	return &Connection{
		Connection: conn,
		opts:       opts,
	}, nil
}

func (c *Connection) InstanceMode() any {
	return c.opts.InstanceMode()
}

func (c *Connection) Close() {
	if err := c.Connection.Close(); err != nil {
		panic(err)
	}

}

func (c *Connection) Done() <-chan struct{} {
	return nil
}

func (c *Connection) Info() string {
	return fmt.Sprintf("Server: %s, timeout; %d, user: %s", c.opts.server, c.opts.cfg.Timeout, c.opts.cfg.User)
}
