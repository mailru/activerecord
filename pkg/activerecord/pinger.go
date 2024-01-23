package activerecord

import (
	"context"
)

type ClusterConfigParameters struct {
	Globs         MapGlobParam
	OptionCreator func(ShardInstanceConfig) (OptionInterface, error)
	OptionChecker func(ctx context.Context, instance ShardInstance) (OptionInterface, error)
}

func (c ClusterConfigParameters) Validate() bool {
	return c.OptionCreator != nil && c.OptionChecker != nil && c.Globs.PoolSize > 0
}
