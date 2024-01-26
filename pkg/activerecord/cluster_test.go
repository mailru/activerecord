package activerecord

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/mock"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

func TestGetClusterInfoFromCfg(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx           context.Context
		path          string
		globs         MapGlobParam
		optionCreator func(ShardInstanceConfig) (OptionInterface, error)
	}
	tests := []struct {
		name    string
		args    args
		mocks   func(*testing.T, *MockConfig)
		want    *Cluster
		wantErr bool
	}{
		{
			name: "cluster hosts from root path (no master or replica keys)",
			mocks: func(t *testing.T, mockConfig *MockConfig) {
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
				globs: MapGlobParam{},
				optionCreator: func(c ShardInstanceConfig) (OptionInterface, error) {
					return &TestOptions{hash: c.Addr}, nil
				},
			},
			want: &Cluster{
				shards: []Shard{
					{
						Masters: []ShardInstance{
							{
								ParamsID: "host1",
								Config: ShardInstanceConfig{
									Mode: ModeMaster,
									Addr: "host1",
								},
								Options: &TestOptions{hash: "host1"},
							},
							{
								ParamsID: "host2",
								Config: ShardInstanceConfig{
									Mode: ModeMaster,
									Addr: "host2",
								},
								Options: &TestOptions{hash: "host2"},
							},
						},
						Replicas: []ShardInstance{},
					},
				},
			},
		},
		{
			name: "cluster hosts from master and replica keys path",
			mocks: func(t *testing.T, mockConfig *MockConfig) {
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
				globs: MapGlobParam{},
				optionCreator: func(c ShardInstanceConfig) (OptionInterface, error) {
					return &TestOptions{hash: c.Addr}, nil
				},
			},
			want: &Cluster{
				shards: []Shard{
					{
						Masters: []ShardInstance{
							{
								ParamsID: "host2",
								Config: ShardInstanceConfig{
									Mode: ModeMaster,
									Addr: "host2",
								},
								Options: &TestOptions{hash: "host2"},
							},
						},
						Replicas: []ShardInstance{
							{
								ParamsID: "host1",
								Config: ShardInstanceConfig{
									Mode: ModeReplica,
									Addr: "host1",
								},
								Options: &TestOptions{hash: "host1", mode: ModeReplica},
							},
						},
					},
				},
			},
		},
		{
			name: "cluster hosts from root path and replica keys path",
			mocks: func(t *testing.T, mockConfig *MockConfig) {
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
				globs: MapGlobParam{},
				optionCreator: func(c ShardInstanceConfig) (OptionInterface, error) {
					return &TestOptions{hash: c.Addr}, nil
				},
			},
			want: &Cluster{
				shards: []Shard{
					{
						Masters: []ShardInstance{
							{
								ParamsID: "host1",
								Config: ShardInstanceConfig{
									Mode: ModeMaster,
									Addr: "host1",
								},
								Options: &TestOptions{hash: "host1"},
							},
						},
						Replicas: []ShardInstance{
							{
								ParamsID: "host2",
								Config: ShardInstanceConfig{
									Mode: ModeReplica,
									Addr: "host2",
								},
								Options: &TestOptions{hash: "host2", mode: ModeReplica},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig := NewMockConfig(t)

			ReinitActiveRecord(
				WithConfig(mockConfig),
			)

			if tt.mocks != nil {
				tt.mocks(t, mockConfig)
			}

			got, err := GetClusterInfoFromCfg(tt.args.ctx, tt.args.path, tt.args.globs, tt.args.optionCreator)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetClusterInfoFromCfg() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Check(t, cmp.DeepEqual(got.ShardInstances(0), tt.want.ShardInstances(0), cmpopts.IgnoreUnexported(Shard{}, TestOptions{})), "GetClusterInfoFromCfg() got = %v, want %v", got, tt.want)
		})
	}
}

func TestShard_Instances(t *testing.T) {
	tests := []struct {
		name  string
		shard Shard
		want  []ShardInstance
	}{
		{
			name: "unordered sequence",
			shard: Shard{
				Masters: []ShardInstance{
					{
						ParamsID: "Master2",
					},
					{
						ParamsID: "Master1",
					},
				},
				Replicas: []ShardInstance{
					{
						ParamsID: "Replica2",
					},
					{
						ParamsID: "Replica3",
					},
					{
						ParamsID: "Replica1",
					},
				},
			},
			want: []ShardInstance{
				{
					ParamsID: "Master1",
				},
				{
					ParamsID: "Master2",
				},
				{
					ParamsID: "Replica1",
				},
				{
					ParamsID: "Replica2",
				},
				{
					ParamsID: "Replica3",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.shard.Instances()
			assert.Check(t, cmp.DeepEqual(tt.want, got))
		})
	}
}
