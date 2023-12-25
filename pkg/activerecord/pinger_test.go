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
		pingFns map[string]func(ctx context.Context) error
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
			opts: []OptionPinger{WithPingInterval(time.Microsecond)},
			pingFns: map[string]func(ctx context.Context) error{
				"conf": func(ctx context.Context) error { return nil },
			},
			want: true,
		},
		{
			name: "started panic pinger funcs",
			opts: []OptionPinger{WithPingInterval(time.Microsecond)},
			pingFns: map[string]func(ctx context.Context) error{
				"panicConf": func(ctx context.Context) error { panic("panic pinger") },
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPinger(tt.opts...)

			for c, fn := range tt.pingFns {
				p.Add(context.Background(), c, fn)
			}

			time.Sleep(time.Millisecond)

			require.Equal(t, tt.want, p.isStarted())

			require.NoError(t, p.StopWatch())
			require.False(t, p.isStarted())
		})
	}
}
