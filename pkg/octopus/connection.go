package octopus

import (
	"context"
	"fmt"

	"github.com/mailru/activerecord/pkg/iproto/iproto"
)

var (
	ErrConnection = fmt.Errorf("error dial to box")
)

func GetConnection(ctx context.Context, octopusOpts *ConnectionOptions) (*Connection, error) {
	pool, err := iproto.Dial(ctx, "tcp", octopusOpts.server, octopusOpts.poolCfg)
	if err != nil {
		return nil, fmt.Errorf("%w %s with connect timeout '%d': %s", ErrConnection, octopusOpts.server, octopusOpts.poolCfg.ConnectTimeout, err)
	}

	return &Connection{pool: pool, opts: octopusOpts}, nil
}

type Connection struct {
	pool *iproto.Pool
	opts *ConnectionOptions
}

func (c *Connection) Call(ctx context.Context, rt RequetsTypeType, data []byte) ([]byte, error) {
	if c == nil || c.pool == nil {
		return []byte{}, fmt.Errorf("attempt call from empty connection")
	}

	return c.pool.Call(ctx, uint32(rt), data)
}

func (c *Connection) InstanceMode() any {
	return c.opts.InstanceMode()
}

func (c *Connection) Close() {
	if c == nil || c.pool == nil {
		return
	}

	c.pool.Close()
}

func (c *Connection) Done() <-chan struct{} {
	return c.pool.Done()
}

func (c *Connection) Info() string {
	return fmt.Sprintf("Server: %s, timeout; %d, poolSize: %d", c.opts.server, c.opts.poolCfg.ConnectTimeout, c.opts.poolCfg.Size)
}
