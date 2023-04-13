package activerecord

import (
	"testing"
)

func TestInitActiveRecord(t *testing.T) {
	type args struct {
		opts []Option
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "empty opts",
			args: args{
				opts: []Option{},
			},
		},
		{
			name: "with logger",
			args: args{
				opts: []Option{
					WithLogger(NewLogger()),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			InitActiveRecord(tt.args.opts...)
		})

		instance = nil
	}
}
