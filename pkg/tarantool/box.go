package tarantool

import (
	"context"
	"fmt"

	"github.com/mailru/activerecord/pkg/activerecord"
)

var DefaultOptionCreator = func(sic activerecord.ShardInstanceConfig) (activerecord.OptionInterface, error) {
	return NewOptions(sic.Addr, sic.Mode, WithTimeout(sic.Timeout), WithCredential(sic.User, sic.Password))
}

func Box(ctx context.Context, shard int, instType activerecord.ShardInstanceType, configPath string, optionCreator func(activerecord.ShardInstanceConfig) (activerecord.OptionInterface, error)) (*Connection, error) {
	if optionCreator == nil {
		optionCreator = DefaultOptionCreator
	}

	clusterInfo, err := activerecord.ConfigCacher().Get(
		ctx,
		configPath,
		DefaultConnectionParams,
		optionCreator,
	)
	if err != nil {
		return nil, fmt.Errorf("can't get cluster %s info: %w", configPath, err)
	}

	if clusterInfo.Shards() < shard {
		return nil, fmt.Errorf("invalid shard num %d, max = %d", shard, clusterInfo.Shards())
	}

	var (
		configBox activerecord.ShardInstance
		ok        bool
	)

	switch instType {
	case activerecord.ReplicaInstanceType:
		configBox, ok = clusterInfo.NextReplica(shard)
		if !ok {
			return nil, fmt.Errorf("replicas not set")
		}
	case activerecord.ReplicaOrMasterInstanceType:
		configBox, ok = clusterInfo.NextReplica(shard)
		if ok {
			break
		}

		fallthrough
	case activerecord.MasterInstanceType:
		configBox = clusterInfo.NextMaster(shard)
	}

	conn, err := activerecord.ConnectionCacher().GetOrAdd(configBox, func(options interface{}) (activerecord.ConnectionInterface, error) {
		octopusOpt, ok := options.(*ConnectionOptions)
		if !ok {
			return nil, fmt.Errorf("invalit type of options %T, want Options", options)
		}

		return GetConnection(ctx, octopusOpt)
	})
	if err != nil {
		return nil, fmt.Errorf("error from connectionCacher: %w", err)
	}

	box, ex := conn.(*Connection)
	if !ex {
		return nil, fmt.Errorf("invalid connection type %T, want *tarantool.Connection", conn)
	}

	return box, nil
}

func CheckShardInstance(ctx context.Context, instance activerecord.ShardInstance) (activerecord.OptionInterface, error) {
	opts, ok := instance.Options.(*ConnectionOptions)
	if !ok {
		return nil, fmt.Errorf("invalit type of options %T, want Options", instance.Options)
	}

	var err error
	c := activerecord.ConnectionCacher().Get(instance)
	if c == nil {
		c, err = GetConnection(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("error from connectionCacher: %w", err)
		}
	}

	conn, ok := c.(*Connection)
	if !ok {
		return nil, fmt.Errorf("invalid connection type %T, want *tarantool.Connection", conn)
	}

	var res []bool

	if err = conn.Call17Typed("dostring", []interface{}{"return box.info.ro"}, &res); err != nil {
		return nil, fmt.Errorf("can't get instance status: %w", err)
	}

	if len(res) == 1 {
		ret := res[0]
		switch ret {
		case false:
			return NewOptions(opts.server, activerecord.ModeMaster)
		default:
			return NewOptions(opts.server, activerecord.ModeReplica)
		}
	}

	return nil, fmt.Errorf("can't parse instance status: %w", err)
}
