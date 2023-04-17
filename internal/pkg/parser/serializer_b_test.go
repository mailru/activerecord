package parser_test

import (
	"go/ast"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/internal/pkg/parser"
)

func TestParseSerializer(t *testing.T) {
	dst := ds.NewRecordPackage()

	if _, err := dst.AddImport("github.com/mailru/activerecord/notexistsfolder/dictionary"); err != nil {
		t.Errorf("can't prepare test data: %s", err)
		return
	}

	type args struct {
		dst    *ds.RecordPackage
		fields []*ast.Field
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "simple serializer",
			args: args{
				dst: dst,
				fields: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "Foo"}},
						Tag:   &ast.BasicLit{Value: "`ar:\"pkg:github.com/mailru/activerecord/notexistsfolder/serializer\"`"},
						Type: &ast.StarExpr{
							X: &ast.SelectorExpr{
								X:   &ast.Ident{Name: "dictionary"},
								Sel: &ast.Ident{Name: "Bar"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "not imported package for serializer type",
			args: args{
				dst: dst,
				fields: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "Foo"}},
						Tag:   &ast.BasicLit{Value: "`ar:\"pkg:github.com/mailru/activerecord/notexistsfolder/serializer\"`"},
						Type: &ast.StarExpr{
							X: &ast.SelectorExpr{
								X:   &ast.Ident{Name: "notimportedpackage"},
								Sel: &ast.Ident{Name: "Bar"},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parser.ParseSerializer(tt.args.dst, tt.args.fields); (err != nil) != tt.wantErr {
				t.Errorf("ParseSerializer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseTypeSerializer(t *testing.T) {
	dst := ds.NewRecordPackage()
	if _, err := dst.AddImport("github.com/mailru/activerecord/notexistsfolder/dictionary"); err != nil {
		t.Errorf("can't prepare test data: %s", err)
		return
	}

	type args struct {
		dst            *ds.RecordPackage
		serializerName string
		t              interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "simple type",
			args: args{
				dst:            dst,
				serializerName: "Foo",
				t: &ast.StarExpr{
					X: &ast.SelectorExpr{
						X:   &ast.Ident{Name: "dictionary"},
						Sel: &ast.Ident{Name: "Bar"},
					},
				},
			},
			want:    "*dictionary.Bar",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.ParseTypeSerializer(tt.args.dst, tt.args.serializerName, tt.args.t)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTypeSerializer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseTypeSerializer() = %v, want %v", got, tt.want)
			}
		})
	}
}
