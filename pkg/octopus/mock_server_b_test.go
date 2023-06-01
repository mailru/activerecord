package octopus

import (
	"log"
	"reflect"
	"testing"

	"github.com/mailru/activerecord/pkg/iproto/iproto"
)

func TestMockServer_ProcessRequest(t *testing.T) {
	logger := NewMockMockServerLogger(t)

	pk := append([][]byte{}, PackString([]byte{}, "pk", iproto.ModeDefault))

	var keysPacked [][][]byte

	for _, key := range []string{"key1"} {
		var keysField [][]byte
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
		mocks  func(t *testing.T)
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
						ID:      1,
						Msg:     RequestTypeSelect,
						Request: PackSelect(1, 0, 0, 10, keysPacked),
						Response: PackResponse([][]byte{
							[]byte("f1"),
							[]byte("f2")},
						),
					},
				},
			},
			args: args{
				msg: uint8(RequestTypeSelect),
				req: PackSelect(1, 0, 0, 1, keysPacked),
			},
			want:   nil,
			wantEx: false,
			mocks: func(t *testing.T) {
				logger.EXPECT().DebugSelectRequest(uint32(1), uint32(0), uint32(0), uint32(1), keysPacked, SelectMockFixture{
					indexnum:   0,
					offset:     0,
					limit:      10,
					keys:       keysPacked,
					respTuples: []TupleData{{Cnt: uint32(2), Data: [][]byte{[]byte("f1"), []byte("f2")}}},
				})
			},
		},
		{
			name: "Call procedure message type (not found)",
			fields: fields{
				oft: []FixtureType{
					{
						ID:      1,
						Msg:     RequestTypeCall,
						Request: PackLua("foo", "a1", "a2"),
						Response: PackResponse([][]byte{
							[]byte("status"),
							[]byte("data")},
						),
					},
				},
			},
			args: args{
				msg: uint8(RequestTypeCall),
				req: PackLua("foo"),
			},
			want:   nil,
			wantEx: false,
			mocks: func(t *testing.T) {
				logger.EXPECT().DebugCallRequest(
					"foo",
					[][]byte{},
					CallMockFixture{
						procName:   "foo",
						args:       [][]byte{[]byte("a1"), []byte("a2")},
						respTuples: []TupleData{{Cnt: uint32(2), Data: [][]byte{[]byte("status"), []byte("data")}}},
					},
				)
			},
		},
		{
			name: "Insert message type (not found)",
			fields: fields{
				oft: []FixtureType{
					{
						ID:       2,
						Msg:      RequestTypeInsert,
						Request:  PackInsertReplace(2, InsertModeInserOrReplace, [][]byte{[]byte("i1")}),
						Response: nil,
					},
				},
			},
			args: args{
				msg: uint8(RequestTypeInsert),
				req: PackInsertReplace(2, InsertModeInserOrReplace, [][]byte{[]byte("i2")}),
			},
			wantEx: false,
			mocks: func(t *testing.T) {
				logger.EXPECT().DebugInsertRequest(uint32(2), true, InsertModeInserOrReplace, TupleData{Cnt: uint32(1), Data: [][]byte{[]byte("i2")}}, InsertMockFixture{
					needRetVal: true,
					insertMode: InsertModeInserOrReplace,
					tuple:      TupleData{Cnt: uint32(1), Data: [][]byte{[]byte("i1")}},
				})
			},
		},
		{
			name: "Update message type (not found)",
			fields: fields{
				oft: []FixtureType{
					{
						ID:  1,
						Msg: RequestTypeUpdate,
						Request: PackUpdate(42, pk, []Ops{
							{
								Field: 1,
								Op:    OpSet,
								Value: []byte("u1"),
							},
							{
								Field: 2,
								Op:    OpSet,
								Value: []byte("u2"),
							},
						}),
						Response: PackResponse([][]byte{}),
					},
				},
			},
			args: args{
				msg: uint8(RequestTypeUpdate),
				req: PackUpdate(42, pk, []Ops{
					{
						Field: 1,
						Op:    OpSet,
						Value: []byte("u1"),
					},
				}),
			},
			wantEx: false,
			mocks: func(t *testing.T) {
				logger.EXPECT().DebugUpdateRequest(
					uint32(42),
					pk,
					[]Ops{
						{
							Field: 1,
							Op:    OpSet,
							Value: []byte("u1"),
						},
					},
					UpdateMockFixture{
						primaryKey: pk,
						updateOps: []Ops{
							{
								Field: 1,
								Op:    OpSet,
								Value: []byte("u1"),
							},
							{
								Field: 2,
								Op:    OpSet,
								Value: []byte("u2"),
							},
						},
					})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mocks != nil {
				tt.mocks(t)
			}

			oms := &MockServer{
				oft:    tt.fields.oft,
				logger: logger,
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

func PackResponse(data [][]byte) []byte {
	var tuples [][][]byte

	tuples = append(tuples, data)

	resp, err := PackResopnseStatus(RcOK, tuples)
	if err != nil {
		log.Fatal(err)
	}

	return resp
}
