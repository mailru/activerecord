package serializer

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mailru/activerecord/pkg/serializer/errs"
)

func TestPrintfUnmarshal(t *testing.T) {
	type args struct {
		val string
	}
	tests := []struct {
		name    string
		args    args
		want    float64
		wantErr error
	}{
		{
			name:    "simple",
			args:    args{val: `1.223`},
			want:    1.223,
			wantErr: nil,
		},
		{
			name:    "err",
			args:    args{val: `{"key": {"nestedkey": "value}}`},
			want:    0,
			wantErr: errs.ErrPrintfParse,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got float64
			err := PrintfUnmarshal("", tt.args.val, &got)
			if tt.wantErr != err && !errors.Is(err, tt.wantErr) {
				t.Errorf("PrintfUnmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PrintfUnmarshal() = %v, want %v", got, tt.want)
			}
		})
	}
}
