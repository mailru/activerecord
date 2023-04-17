package parser

import (
	"go/ast"
	"reflect"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/pkg/octopus"
)

func TestParseFieldsObject(t *testing.T) {
	rp := ds.NewRecordPackage()

	err := rp.AddField(ds.FieldDeclaration{
		Name:       "BarID",
		Format:     octopus.Int,
		PrimaryKey: false,
		Mutators:   []ds.FieldMutator{},
		Size:       0,
		Serializer: []string{},
		ObjectLink: "",
	})
	if err != nil {
		t.Errorf("can't prepare test data: %s", err)
		return
	}

	wantRp := ds.NewRecordPackage()
	wantRp.FieldsMap["BarID"] = len(wantRp.Fields)
	wantRp.Fields = append(wantRp.Fields, ds.FieldDeclaration{
		Name:       "BarID",
		Format:     octopus.Int,
		PrimaryKey: false,
		Mutators:   []ds.FieldMutator{},
		Size:       0,
		Serializer: []string{},
		ObjectLink: "Bar",
	})
	wantRp.FieldsObjectMap["Bar"] = ds.FieldObject{
		Name:       "Bar",
		Key:        "ID",
		ObjectName: "bar",
		Field:      "BarID",
		Unique:     true,
	}

	type args struct {
		dst          *ds.RecordPackage
		fieldsobject []*ast.Field
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    *ds.RecordPackage
	}{
		{
			name: "simple field object",
			args: args{
				dst: rp,
				fieldsobject: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "Bar"}},
						Type:  &ast.Ident{Name: "bool"},
						Tag:   &ast.BasicLit{Value: "`" + `ar:"key:ID;object:bar;field:BarID"` + "`"},
					},
				},
			},
			wantErr: false,
			want:    wantRp,
		},
		{
			name: "invalid ident type",
			args: args{
				dst: rp,
				fieldsobject: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "Bar"}},
						Type:  &ast.Ident{Name: "map[string]bool"},
						Tag:   &ast.BasicLit{Value: "`" + `ar:"key:ID;object:bar;field:BarID"` + "`"},
					},
				},
			},
			wantErr: true,   // Ожидаем ошибку
			want:    wantRp, // Состояние с прошлого прогона не должно поменяться
		},
		{
			name: "invalid not slice type",
			args: args{
				dst: rp,
				fieldsobject: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "Bar"}},
						Type:  &ast.ArrayType{Len: &ast.Ident{}, Elt: &ast.Ident{Name: "int"}},
						Tag:   &ast.BasicLit{Value: "`" + `ar:"key:ID;object:bar;field:BarID"` + "`"},
					},
				},
			},
			wantErr: true,   // Ожидаем ошибку
			want:    wantRp, // Состояние с прошлого прогона не должно поменяться
		},
		{
			name: "invalid slice type",
			args: args{
				dst: rp,
				fieldsobject: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "Bar"}},
						Type:  &ast.ArrayType{Elt: &ast.Ident{Name: "int"}},
						Tag:   &ast.BasicLit{Value: "`" + `ar:"key:ID;object:bar;field:BarID"` + "`"},
					},
				},
			},
			wantErr: true,   // Ожидаем ошибку
			want:    wantRp, // Состояние с прошлого прогона не должно поменяться
		},
		{
			name: "invalid type",
			args: args{
				dst: rp,
				fieldsobject: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "Bar"}},
						Type:  &ast.MapType{},
						Tag:   &ast.BasicLit{Value: "`" + `ar:"key:ID;object:bar;field:BarID"` + "`"},
					},
				},
			},
			wantErr: true,   // Ожидаем ошибку
			want:    wantRp, // Состояние с прошлого прогона не должно поменяться
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ParseFieldsObject(tt.args.dst, tt.args.fieldsobject); (err != nil) != tt.wantErr {
				t.Errorf("ParseFieldsObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(tt.args.dst, tt.want) {
				t.Errorf("ParseFieldsObject() Fields = %+v, wantFields %+v", tt.args.dst, tt.want)
			}
		})
	}
}
