package octopus

import (
	"reflect"
	"testing"

	"github.com/mailru/activerecord/pkg/iproto/iproto"
)

func TestMockServer_ProcessRequest(t *testing.T) {
	keysPacked := [][][]byte{}

	for _, key := range []string{"key1"} {
		keysField := [][]byte{}
		keysField = append(keysField, PackString([]byte{}, key, iproto.ModeDefault))
		keysPacked = append(keysPacked, keysField)
	}

	type fields struct {
		oft []FixtureType
	}
	type args struct {
		msg uint8
		req []byte
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []byte
		wantEx bool
	}{
		{
			name: "simple check ProcessRequest",
			fields: fields{
				oft: []FixtureType{
					{
						ID:       1,
						Msg:      RequestTypeSelect,
						Request:  []byte("aassdd"),
						Response: []byte("zzxxcc"),
					},
				},
			},
			args: args{
				msg: uint8(RequestTypeSelect),
				req: []byte("aassdd"),
			},
			want:   []byte("zzxxcc"),
			wantEx: true,
		},
		{
			name: "Select message type",
			fields: fields{
				oft: []FixtureType{
					{
						ID:       1,
						Msg:      RequestTypeSelect,
						Request:  PackSelect(1, 0, 0, 10, keysPacked),
						Response: []byte("select ok"),
					},
				},
			},
			args: args{
				msg: uint8(RequestTypeSelect),
				req: PackSelect(1, 0, 0, 10, keysPacked),
			},
			want:   []byte("select ok"),
			wantEx: true,
		},
		{
			name: "Select message type (not found)",
			fields: fields{
				oft: []FixtureType{
					{
						ID:       1,
						Msg:      RequestTypeSelect,
						Request:  PackSelect(1, 0, 0, 10, keysPacked),
						Response: []byte("select ok"),
					},
				},
			},
			args: args{
				msg: uint8(RequestTypeSelect),
				req: PackSelect(1, 0, 0, 1, keysPacked),
			},
			want:   nil,
			wantEx: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oms := &MockServer{
				oft:    tt.fields.oft,
				logger: &DefaultLogger{},
			}

			got, ex := oms.ProcessRequest(tt.args.msg, tt.args.req)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MockServer.ProcessRequest() got = %v, want %v", got, tt.want)
			}

			if ex != tt.wantEx {
				t.Errorf("MockServer.ProcessRequest() exist = %v, want %v", ex, tt.wantEx)
			}
		})
	}
}
