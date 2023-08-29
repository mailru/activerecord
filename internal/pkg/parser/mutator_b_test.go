package parser_test

import (
	"go/ast"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/internal/pkg/parser"
)

type Bar struct {
}

//nolint:unused
func _TestParseMutator(t *testing.T) {
	dst := ds.NewRecordPackage()

	if _, err := dst.AddImport("github.com/mailru/activerecord/internal/pkg/parser", "parser_test"); err != nil {
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
			name: "simple mutator",
			args: args{
				dst: dst,
				fields: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "Foo"}},
						Tag:   &ast.BasicLit{Value: "`ar:\"pkg:github.com/mailru/activerecord/internal/pkg/parser\"`"},
						Type: &ast.StarExpr{
							X: &ast.SelectorExpr{
								X:   &ast.Ident{Name: "parser_test"},
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
			if err := parser.ParseMutators(tt.args.dst, tt.args.fields); (err != nil) != tt.wantErr {
				t.Errorf("ParseMutators() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
