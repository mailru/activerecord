package activerecord_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/mailru/activerecord/pkg/activerecord"
	"github.com/mailru/activerecord/pkg/iproto/iproto"
	"github.com/mailru/activerecord/pkg/octopus"
	"github.com/stretchr/testify/mock"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

func TestGetClusterInfoFromCfg(t *testing.T) {
	ctx := context.Background()

	_ = &iproto.PoolConfig{
		Size:              octopus.DefaultPoolSize,
		ConnectTimeout:    octopus.DefaultConnectionTimeout,
		DialTimeout:       octopus.DefaultConnectionTimeout,
		RedialInterval:    octopus.DefaultRedialInterval,
		MaxRedialInterval: octopus.DefaultRedialInterval,
		ChannelConfig: &iproto.ChannelConfig{
			WriteTimeout:   octopus.DefaultTimeout,
			RequestTimeout: octopus.DefaultTimeout,
			PingInterval:   octopus.DefaultPingInterval,
		},
	}

	type args struct {
		ctx           context.Context
		path          string
		globs         activerecord.MapGlobParam
		optionCreator func(activerecord.ShardInstanceConfig) (activerecord.OptionInterface, error)
	}
	tests := []struct {
		name    string
		args    args
		mocks   func(*testing.T, *activerecord.MockConfig)
		want    activerecord.Cluster
		wantErr bool
	}{
		{
			name: "cluster hosts from root path (no master or replica keys)",
			mocks: func(t *testing.T, mockConfig *activerecord.MockConfig) {
				mockConfig.EXPECT().GetIntIfExists(mock.Anything, mock.Anything).Return(0, false)
				mockConfig.EXPECT().GetDuration(mock.Anything, mock.Anything, mock.Anything).Return(0)
				mockConfig.EXPECT().GetInt(mock.Anything, mock.Anything, mock.Anything).Return(0)
				mockConfig.EXPECT().GetDurationIfExists(mock.Anything, mock.Anything).Return(0, false)
				mockConfig.EXPECT().GetStringIfExists(mock.Anything, "testconfig/master").Return("", false)
				mockConfig.EXPECT().GetStringIfExists(mock.Anything, "testconfig").Return("host1,host2", true)
				mockConfig.EXPECT().GetStringIfExists(mock.Anything, "testconfig/replica").Return("", false)
			},
			args: args{
				ctx:   ctx,
				path:  "testconfig",
				globs: activerecord.MapGlobParam{},
				optionCreator: func(c activerecord.ShardInstanceConfig) (activerecord.OptionInterface, error) {
					return octopus.NewOptions(c.Addr, octopus.ServerModeType(c.Mode))
				},
			},
			want: activerecord.Cluster{
				{
					Masters: []activerecord.ShardInstance{
						{
							Config: activerecord.ShardInstanceConfig{
								Mode: activerecord.ModeMaster,
								Addr: "host1",
							},
							Options: &octopus.ConnectionOptions{
								Mode: octopus.ModeMaster,
							},
						},
						{
							Config: activerecord.ShardInstanceConfig{
								Mode: activerecord.ModeMaster,
								Addr: "host2",
							},
							Options: &octopus.ConnectionOptions{
								Mode: octopus.ModeMaster,
							},
						},
					},
					Replicas: []activerecord.ShardInstance{},
				},
			},
		},
		{
			name: "cluster hosts from master and replica keys path",
			mocks: func(t *testing.T, mockConfig *activerecord.MockConfig) {
				mockConfig.EXPECT().GetIntIfExists(mock.Anything, mock.Anything).Return(0, false)
				mockConfig.EXPECT().GetDuration(mock.Anything, mock.Anything, mock.Anything).Return(0)
				mockConfig.EXPECT().GetInt(mock.Anything, mock.Anything, mock.Anything).Return(0)
				mockConfig.EXPECT().GetDurationIfExists(mock.Anything, mock.Anything).Return(0, false)
				mockConfig.EXPECT().GetStringIfExists(mock.Anything, "testconfig/master").Return("host2", true)
				mockConfig.EXPECT().GetStringIfExists(mock.Anything, "testconfig/replica").Return("host1", true)
			},
			args: args{
				ctx:   ctx,
				path:  "testconfig",
				globs: activerecord.MapGlobParam{},
				optionCreator: func(c activerecord.ShardInstanceConfig) (activerecord.OptionInterface, error) {
					return octopus.NewOptions(c.Addr, octopus.ServerModeType(c.Mode))
				},
			},
			want: activerecord.Cluster{
				{
					Masters: []activerecord.ShardInstance{
						{
							Config: activerecord.ShardInstanceConfig{
								Mode: activerecord.ModeMaster,
								Addr: "host2",
							},
							Options: &octopus.ConnectionOptions{
								Mode: octopus.ModeMaster,
							},
						},
					},
					Replicas: []activerecord.ShardInstance{
						{
							Config: activerecord.ShardInstanceConfig{
								Mode: activerecord.ModeReplica,
								Addr: "host1",
							},
							Options: &octopus.ConnectionOptions{
								Mode: octopus.ModeReplica,
							},
						},
					},
				},
			},
		},
		{
			name: "cluster hosts from root path and replica keys path",
			mocks: func(t *testing.T, mockConfig *activerecord.MockConfig) {
				mockConfig.EXPECT().GetIntIfExists(mock.Anything, mock.Anything).Return(0, false)
				mockConfig.EXPECT().GetDuration(mock.Anything, mock.Anything, mock.Anything).Return(0)
				mockConfig.EXPECT().GetInt(mock.Anything, mock.Anything, mock.Anything).Return(0)
				mockConfig.EXPECT().GetDurationIfExists(mock.Anything, mock.Anything).Return(0, false)
				mockConfig.EXPECT().GetStringIfExists(mock.Anything, "testconfig/master").Return("", false)
				mockConfig.EXPECT().GetStringIfExists(mock.Anything, "testconfig").Return("host1", true)
				mockConfig.EXPECT().GetStringIfExists(mock.Anything, "testconfig/replica").Return("host2", true)
			},
			args: args{
				ctx:   ctx,
				path:  "testconfig",
				globs: activerecord.MapGlobParam{},
				optionCreator: func(c activerecord.ShardInstanceConfig) (activerecord.OptionInterface, error) {
					return octopus.NewOptions(c.Addr, octopus.ServerModeType(c.Mode))
				},
			},
			want: activerecord.Cluster{
				{
					Masters: []activerecord.ShardInstance{
						{
							Config: activerecord.ShardInstanceConfig{
								Mode: activerecord.ModeMaster,
								Addr: "host1",
							},
							Options: &octopus.ConnectionOptions{
								Mode: octopus.ModeMaster,
							},
						},
					},
					Replicas: []activerecord.ShardInstance{
						{
							Config: activerecord.ShardInstanceConfig{
								Mode: activerecord.ModeReplica,
								Addr: "host2",
							},
							Options: &octopus.ConnectionOptions{
								Mode: octopus.ModeReplica,
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig := activerecord.NewMockConfig(t)

			activerecord.ReinitActiveRecord(
				activerecord.WithConfig(mockConfig),
			)

			if tt.mocks != nil {
				tt.mocks(t, mockConfig)
			}

			got, err := activerecord.GetClusterInfoFromCfg(tt.args.ctx, tt.args.path, tt.args.globs, tt.args.optionCreator)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetClusterInfoFromCfg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Check(t, cmp.DeepEqual(got, tt.want, cmpopts.IgnoreFields(activerecord.ShardInstance{}, "ParamsID"), cmpopts.IgnoreUnexported(activerecord.Shard{}, octopus.ConnectionOptions{})), "GetClusterInfoFromCfg() got = %v, want %v", got, tt.want)
		})
	}
}
