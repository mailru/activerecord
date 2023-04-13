package serializer

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mailru/activerecord/pkg/serializer/errs"
)

func TestJSONUnmarshal(t *testing.T) {
	type args struct {
		val string
	}
	tests := []struct {
		name    string
		args    args
		exec    func(string) (any, error)
		want    any
		wantErr error
	}{
		{
			name: "simple map",
			args: args{val: `{"key": "value"}`},
			exec: func(val string) (any, error) {
				var got map[string]interface{}
				err := JSONUnmarshal(val, &got)
				return got, err
			},
			want:    map[string]interface{}{"key": "value"},
			wantErr: nil,
		},
		{
			name: "nested map",
			args: args{val: `{"key": {"nestedkey": "value"}}`},
			exec: func(val string) (any, error) {
				var got map[string]interface{}
				err := JSONUnmarshal(val, &got)
				return got, err
			},
			want:    map[string]interface{}{"key": map[string]interface{}{"nestedkey": "value"}},
			wantErr: nil,
		},
		{
			name: "err map unmarshal",
			args: args{val: `{"key": {"nestedkey": "value}}`},
			exec: func(val string) (any, error) {
				var got map[string]interface{}
				err := JSONUnmarshal(val, &got)
				return got, err
			},
			want:    nil,
			wantErr: errs.ErrUnmarshalJSON,
		},
		{
			name: "simple custom type ",
			args: args{val: `{"quota": 2373874}`},
			exec: func(val string) (any, error) {
				var got Services
				err := JSONUnmarshal(val, &got)
				return got, err
			},
			want:    Services{Quota: 2373874},
			wantErr: nil,
		},
		{
			name: "nested custom type",
			args: args{val: `{"quota": 234321523, "gift": {"giftible_id": "year2020_333_1", "gift_quota": 2343432784}}`},
			exec: func(val string) (any, error) {
				var got Services
				err := JSONUnmarshal(val, &got)
				return got, err
			},
			want:    Services{Quota: 234321523, Gift: &Gift{GiftibleID: "year2020_333_1", GiftQuota: 2343432784}},
			wantErr: nil,
		},
		{
			name: "err custom type",
			args: args{val: `{"quota": 234321523, "gift": }}}}}}}}}}`},
			exec: func(val string) (any, error) {
				var got Services
				err := JSONUnmarshal(val, &got)
				return got, err
			},
			want:    nil,
			wantErr: errs.ErrUnmarshalJSON,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.exec(tt.args.val)
			if tt.wantErr != err && !errors.Is(err, tt.wantErr) {
				t.Errorf("JSONUnmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("JSONUnmarshal() = %v, want %v", got, tt.want)
			}
		})
	}
}
