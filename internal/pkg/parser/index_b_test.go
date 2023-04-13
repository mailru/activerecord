package parser_test

import (
	"go/ast"
	"reflect"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/internal/pkg/parser"
)

func TestParseIndexPart(t *testing.T) {
	type args struct {
		dst    *ds.RecordPackage
		fields []*ast.Field
	}

	wantRp := ds.NewRecordPacakge()
	wantRp.Fields = []ds.FieldDeclaration{
		{Name: "Field1", Format: "int"},
		{Name: "Field2", Format: "int"},
	}
	wantRp.FieldsMap = map[string]int{"Field1": 0, "Field2": 1}
	wantRp.Indexes = []ds.IndexDeclaration{
		{
			Name:     "Field1Field2",
			Num:      0,
			Selector: "SelectByField1Field2",
			Fields:   []int{0, 1},
			FieldsMap: map[string]ds.IndexField{
				"Field1": {IndField: 0, Order: 0},
				"Field2": {IndField: 1, Order: 0},
			},
			Unique: true,
		},
		{
			Name:     "Field1Part",
			Num:      1,
			Selector: "SelectByField1",
			Fields:   []int{0},
			FieldsMap: map[string]ds.IndexField{
				"Field1": {IndField: 0, Order: 0},
			},
		},
	}
	wantRp.IndexMap = map[string]int{"Field1Field2": 0, "Field1Part": 1}
	wantRp.SelectorMap = map[string]int{"SelectByField1": 1, "SelectByField1Field2": 0}

	rp := ds.NewRecordPacakge()

	err := rp.AddField(ds.FieldDeclaration{
		Name:       "Field1",
		Format:     "int",
		PrimaryKey: false,
	})
	if err != nil {
		t.Errorf("can't prepare test data: %s", err)
		return
	}

	err = rp.AddField(ds.FieldDeclaration{
		Name:       "Field2",
		Format:     "int",
		PrimaryKey: false,
	})
	if err != nil {
		t.Errorf("can't prepare test data: %s", err)
		return
	}

	err = rp.AddIndex(ds.IndexDeclaration{
		Name:      "Field1Field2",
		Num:       0,
		Selector:  "SelectByField1Field2",
		Fields:    []int{0, 1},
		FieldsMap: map[string]ds.IndexField{"Field1": {IndField: 0, Order: ds.IndexOrderAsc}, "Field2": {IndField: 1, Order: ds.IndexOrderAsc}},
		Primary:   false,
		Unique:    true,
		Type:      "",
	})
	if err != nil {
		t.Errorf("can't prepare test data: %s", err)
		return
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    *ds.RecordPackage
	}{
		{
			name: "simple index part",
			args: args{
				dst: rp,
				fields: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "Field1Part"}},
						Type:  &ast.Ident{Name: "bool"},
						Tag:   &ast.BasicLit{Value: "`" + `ar:"index:Field1Field2;fieldnum:1;selector:SelectByField1"` + "`"},
					},
				},
			},
			wantErr: false,
			want:    wantRp,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parser.ParseIndexPart(tt.args.dst, tt.args.fields); (err != nil) != tt.wantErr {
				t.Errorf("ParseIndexPart() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(tt.args.dst, tt.want) {
				t.Errorf("ParseIndexPart() = %+v, want %+v", tt.args.dst, tt.want)
			}
		})
	}
}
