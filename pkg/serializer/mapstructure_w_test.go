package serializer

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mailru/activerecord/pkg/serializer/errs"
)

type Services struct {
	Quota uint64
	Flags map[string]bool
	Gift  *Gift
	Other map[string]interface{} `mapstructure:",remain"`
}

type Gift struct {
	GiftInterval string `mapstructure:"gift_interval" json:"gift_interval"`
	GiftQuota    uint64 `mapstructure:"gift_quota" json:"gift_quota"`
	GiftibleID   string `mapstructure:"giftible_id" json:"giftible_id"`
}

func TestMapstructureUnmarshal(t *testing.T) {
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
			name: "simple",
			args: args{val: `{"quota": 2373874}`},
			exec: func(val string) (any, error) {
				var got Services
				err := MapstructureUnmarshal(val, &got)
				return got, err
			},
			want:    Services{Quota: 2373874},
			wantErr: nil,
		},
		{
			name: "with nested map",
			args: args{val: `{"quota": 234321523, "flags": {"UF": true, "OS": true}}`},
			exec: func(val string) (any, error) {
				var got Services
				err := MapstructureUnmarshal(val, &got)
				return got, err
			},
			want:    Services{Quota: 234321523, Flags: map[string]bool{"UF": true, "OS": true}},
			wantErr: nil,
		},
		{
			name: "with nested struct",
			args: args{val: `{"quota": 234321523, "gift": {"giftible_id": "year2020_333_1", "gift_quota": 2343432784}}`},
			exec: func(val string) (any, error) {
				var got Services
				err := MapstructureUnmarshal(val, &got)
				return got, err
			},
			want:    Services{Quota: 234321523, Gift: &Gift{GiftibleID: "year2020_333_1", GiftQuota: 2343432784}},
			wantErr: nil,
		},
		{
			name: "bad input",
			args: args{val: `{"quota": 234321523, "gift": }}}}}}}}}}`},
			exec: func(val string) (any, error) {
				var got Services
				err := MapstructureUnmarshal(val, &got)
				return got, err
			},
			want:    nil,
			wantErr: errs.ErrUnmarshalJSON,
		},
		{
			name: "mapstructure remain",
			args: args{val: `{"quota": 234321523, "unknown_field": "unknown"}`},
			exec: func(val string) (any, error) {
				var got Services
				err := MapstructureUnmarshal(val, &got)
				return got, err
			},
			want:    Services{Quota: 234321523, Other: map[string]interface{}{"unknown_field": "unknown"}},
			wantErr: nil,
		},
		{
			name: "mapstructure err unused",
			args: args{val: `{"quota": 234321523, "unused_field": "unused"}`},
			exec: func(val string) (any, error) {
				// Декодируем в структуру без поля c тегом `mapstructure:",remain"`
				var got struct {
					Quota uint64
				}
				err := MapstructureUnmarshal(val, &got)
				return got, err
			},
			want:    nil,
			wantErr: errs.ErrMapstructureDecode,
		},
		{
			name: "mapstructure err create decoder",
			args: args{val: `{"quota": 2373874}`},
			exec: func(val string) (any, error) {
				var got Services
				// В mapstructurе вторым параметром надо отдавать pointer
				err := MapstructureUnmarshal(val, got)
				return got, err
			},
			want:    nil,
			wantErr: errs.ErrMapstructureNewDecoder,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.exec(tt.args.val)
			if tt.wantErr != err && !errors.Is(err, tt.wantErr) {
				t.Errorf("MapstructureUnmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MapstructureUnmarshal() = %v, want %v", got, tt.want)
			}
		})
	}
}
