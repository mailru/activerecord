package octopus

import (
	"reflect"
	"testing"
)

func TestPackSelect(t *testing.T) {
	namespace := []byte{0x02, 0x00, 0x00, 0x00}
	indexNum := []byte{0x01, 0x00, 0x00, 0x00}
	offset := []byte{0x00, 0x00, 0x00, 0x00}
	limit := []byte{0x0A, 0x00, 0x00, 0x00}
	tuples := []byte{0x02, 0x00, 0x00, 0x00}
	tuple1 := []byte{0x02, 0x00, 0x00, 0x00, 0x03, 97, 97, 97, 0x02, 0x10, 0x00}
	tuple2 := []byte{0x02, 0x00, 0x00, 0x00, 0x03, 98, 98, 98, 0x02, 0x20, 0x00}

	selectReq := append(namespace, indexNum...)
	selectReq = append(selectReq, offset...)
	selectReq = append(selectReq, limit...)
	selectReq = append(selectReq, tuples...)
	selectReq = append(selectReq, tuple1...)
	selectReq = append(selectReq, tuple2...)

	type args struct {
		ns       uint32
		indexnum uint32
		offset   uint32
		limit    uint32
		keys     [][][]byte
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "select",
			args: args{
				ns:       2,
				indexnum: 1,
				offset:   0,
				limit:    10,
				keys: [][][]byte{
					{
						[]byte("aaa"),
						{0x10, 0x00},
					},
					{
						[]byte("bbb"),
						{0x20, 0x00},
					},
				},
			},
			want: selectReq,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PackSelect(tt.args.ns, tt.args.indexnum, tt.args.offset, tt.args.limit, tt.args.keys); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PackSelect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPackInsertReplace(t *testing.T) {
	fieldValue := []byte{0x0A, 0x00, 0x00, 0x00}
	namespace := []byte{0x02, 0x00, 0x00, 0x00}
	insertreplaceFlags := []byte{0x01, 0x00, 0x00, 0x00}
	insertFlags := []byte{0x03, 0x00, 0x00, 0x00}
	replaceFlags := []byte{0x05, 0x00, 0x00, 0x00}
	insertTupleCardinality := []byte{0x02, 0x00, 0x00, 0x00}
	insertTupleFields := append([]byte{0x04}, fieldValue...) //len + Field1
	insertTupleFields = append(insertTupleFields, []byte{0x00}...)

	insertReq := append(namespace, insertFlags...)
	insertReq = append(insertReq, insertTupleCardinality...)
	insertReq = append(insertReq, insertTupleFields...)

	replaceReq := append(namespace, replaceFlags...)
	replaceReq = append(replaceReq, insertTupleCardinality...)
	replaceReq = append(replaceReq, insertTupleFields...)

	insertreplaceReq := append(namespace, insertreplaceFlags...)
	insertreplaceReq = append(insertreplaceReq, insertTupleCardinality...)
	insertreplaceReq = append(insertreplaceReq, insertTupleFields...)

	type args struct {
		ns         uint32
		insertMode InsertMode
		tuple      [][]byte
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "insert",
			args: args{
				ns:         2,
				insertMode: 1,
				tuple: [][]byte{
					fieldValue,
					{},
				},
			},
			want: insertReq,
		},
		{
			name: "replace",
			args: args{
				ns:         2,
				insertMode: 2,
				tuple: [][]byte{
					fieldValue,
					{},
				},
			},
			want: replaceReq,
		},
		{
			name: "insertreplace",
			args: args{
				ns:         2,
				insertMode: 0,
				tuple: [][]byte{
					fieldValue,
					{},
				},
			},
			want: insertreplaceReq,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PackInsertReplace(tt.args.ns, tt.args.insertMode, tt.args.tuple); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PackInsertReplace() = %v, want %v", got, tt.want)
			}
		})
	}
}
