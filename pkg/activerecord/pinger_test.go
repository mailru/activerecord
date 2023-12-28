package activerecord

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestNewPinger(t *testing.T) {
	tests := []struct {
		name    string
		opts    []OptionPinger
		pingFns map[string]func(ctx context.Context, instance ShardInstance) (ServerModeType, error)
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
			opts: []OptionPinger{WithPingInterval(time.Microsecond), WithConfigCache(newConfigCacher())},
			pingFns: map[string]func(ctx context.Context, instance ShardInstance) (ServerModeType, error){
				"conf": func(ctx context.Context, instance ShardInstance) (ServerModeType, error) { return 0, nil },
			},
			want: true,
		},
		{
			name: "started panic pinger funcs",
			opts: []OptionPinger{WithPingInterval(time.Microsecond), WithConfigCache(newConfigCacher())},
			pingFns: map[string]func(ctx context.Context, instance ShardInstance) (ServerModeType, error){
				"panicConf": func(ctx context.Context, instance ShardInstance) (ServerModeType, error) { panic("panic pinger") },
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPinger(tt.opts...)

			for c, fn := range tt.pingFns {
				_, err := p.SchedulePingIfNotExists(context.Background(), c, fn)
				require.NoError(t, err)
			}

			time.Sleep(time.Millisecond)

			require.Equal(t, tt.want, p.isStarted())

			require.NoError(t, p.StopWatch())
			require.False(t, p.isStarted())
		})
	}
}
