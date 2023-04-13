package octopus

import (
	"context"
	"testing"
	"time"
)

func Test_prepareConnection(t *testing.T) {
	type args struct {
		server string
		opts   []ConnectionOption
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "connectionHash",
			args: args{
				server: "127.0.0.1",
				opts:   []ConnectionOption{},
			},
			want:    "ff9ed3cc",
			wantErr: false,
		},
		{
			name: "same connectionHash",
			args: args{
				server: "127.0.0.1",
				opts:   []ConnectionOption{},
			},
			want:    "ff9ed3cc",
			wantErr: false,
		},
		{
			name: "connectionHash with options",
			args: args{
				server: "127.0.0.1",
				opts: []ConnectionOption{
					WithTimeout(time.Millisecond*50, time.Millisecond*100),
				},
			},
			want:    "f855b29a",
			wantErr: false,
		},
		{
			name: "yes another connectionHash with options",
			args: args{
				server: "127.0.0.1",
				opts: []ConnectionOption{
					WithTimeout(time.Millisecond*50, time.Millisecond*100),
					WithIntervals(time.Second*50, time.Second*50, time.Second*50),
				},
			},
			want:    "fdef5d9b",
			wantErr: false,
		},
		{
			name: "yes another connectionHash with options",
			args: args{
				server: "",
				opts: []ConnectionOption{
					WithTimeout(time.Millisecond*50, time.Millisecond*100),
					WithIntervals(time.Second*50, time.Second*50, time.Second*50),
				},
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewOptions(tt.args.server, ModeMaster, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("prepareConnection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.GetConnectionID() != tt.want {
				t.Errorf("prepareConnection() Hex = %v, want %v", got.GetConnectionID(), tt.want)
			}
		})
	}
}

func TestGetConnection(t *testing.T) {
	type args struct {
		ctx    context.Context
		server string
		opts   []ConnectionOption
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "first connection",
			args: args{
				ctx:    context.Background(),
				server: "127.0.0.1:11211",
				opts:   []ConnectionOption{},
			},
			wantErr: false,
			want:    []string{""},
		},
	}

	oms, err := InitMockServer(WithHost("127.0.0.1", "11211"))
	if err != nil {
		t.Fatalf("error init octopusMock %s", err)
		return
	}

	err = oms.Start()
	if err != nil {
		t.Fatalf("error start octopusMock %s", err)
		return
	}

	defer func() {
		err := oms.Stop()
		if err != nil {
			t.Fatalf("error stop octopusMock %s", err)
		}
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			octopusOpts, err := NewOptions(tt.args.server, ModeMaster, tt.args.opts...)
			if err != nil {
				t.Errorf("can't initialize options: %s", err)
			}
			_, err = GetConnection(tt.args.ctx, octopusOpts)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetConnection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
