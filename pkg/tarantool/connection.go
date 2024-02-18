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
}

func GetConnection(_ context.Context, opts *ConnectionOptions) (*Connection, error) {
	conn, err := tarantool.Connect(opts.server, opts.cfg)
	if err != nil {
		return nil, fmt.Errorf("error connect to tarantool %s with connect timeout '%d': %s", opts.server, opts.cfg.Timeout, err)
	}

	return &Connection{conn}, nil
}

func (c *Connection) Close() {
	if err := c.Connection.Close(); err != nil {
		panic(err)
	}

}

func (c *Connection) Done() <-chan struct{} {
	return nil
}
