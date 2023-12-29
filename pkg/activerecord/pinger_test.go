package activerecord

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestNewPinger(t *testing.T) {
	defaultConfigCacher := NewDefaultConfigCacher(MapGlobParam{Timeout: time.Millisecond, PoolSize: 1}, func(config ShardInstanceConfig) (OptionInterface, error) {
		return &TestOptions{hash: "testopt1"}, nil
	})

	tests := []struct {
		name    string
		opts    []OptionPinger
		pingFns map[string]func(ctx context.Context, instance ShardInstance) (ServerModeType, error)
		mocks   func(*testing.T, *MockConfig)
		want    bool
	}{
		{
			name: "not started",
			want: false,
		},
		{
			name: "started without pinger funcs",
			opts: []OptionPinger{WithPingInterval(time.Microsecond), WithStart()},
			want: true,
		},
		{
			name: "started pinger funcs",
			opts: []OptionPinger{WithConfigCache(defaultConfigCacher)},
			mocks: func(t *testing.T, mockConfig *MockConfig) {
				mockConfig.EXPECT().GetIntIfExists(mock.Anything, mock.Anything).Return(0, false)
				mockConfig.EXPECT().GetDuration(mock.Anything, mock.Anything, mock.Anything).Return(0)
				mockConfig.EXPECT().GetInt(mock.Anything, mock.Anything, mock.Anything).Return(0)
				mockConfig.EXPECT().GetDurationIfExists(mock.Anything, mock.Anything).Return(0, false)
				mockConfig.EXPECT().GetStringIfExists(mock.Anything, "conf/master").Return("", false)
				mockConfig.EXPECT().GetStringIfExists(mock.Anything, "conf").Return("host1,host2", true)
				mockConfig.EXPECT().GetStringIfExists(mock.Anything, "conf/replica").Return("", false)
			},
			pingFns: map[string]func(ctx context.Context, instance ShardInstance) (ServerModeType, error){
				"conf": func(ctx context.Context, instance ShardInstance) (ServerModeType, error) { return 0, nil },
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig := NewMockConfig(t)

			ReinitActiveRecord(
				WithConfig(mockConfig),
				WithConfigCacher(defaultConfigCacher),
			)

			if tt.mocks != nil {
				tt.mocks(t, mockConfig)
			}
			p := NewPinger(tt.opts...)

			for c, fn := range tt.pingFns {
				_, err := p.AddClusterChecker(context.Background(), c, fn)
				require.NoError(t, err)
			}

			time.Sleep(time.Millisecond)

			require.Equal(t, tt.want, p.isStarted())

			require.NoError(t, p.StopWatch())
			require.False(t, p.isStarted())
		})
	}
}
