package parser_test

import (
	"fmt"
	"go/ast"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"

	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/internal/pkg/parser"
)

func NewRecordPackage(t *testing.T) (*ds.RecordPackage, error) {
	dst := ds.NewRecordPackage()
	dst.Namespace.ModuleName = "github.com/mailru/activerecord/internal/pkg/parser"

	if _, err := dst.AddImport("github.com/mailru/activerecord/internal/pkg/parser/testdata/foo"); err != nil {
		return nil, fmt.Errorf("can't create test package: %w", err)
	}

	return dst, nil
}

func TestParseMutator(t *testing.T) {
	type args struct {
		fields []*ast.Field
	}
	tests := []struct {
		name    string
		args    args
		want    *ds.RecordPackage
		wantErr bool
	}{
		{
			name: "parse mutator decl",
			args: args{
				fields: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "FooMutatorField"}},
						Tag: &ast.BasicLit{
							Value: "`ar:\"update:updateFunc,param1,param2;replace:replaceFunc;pkg:github.com/mailru/activerecord/internal/pkg/conv\"`",
						},
						Type: &ast.StarExpr{
							X: &ast.SelectorExpr{
								X:   &ast.Ident{Name: "foo"},
								Sel: &ast.Ident{Name: "Foo"},
							},
						},
					},
					{
						Names: []*ast.Ident{{Name: "SimpleTypeMutatorField"}},
						Tag: &ast.BasicLit{
							Value: "`ar:\"update:updateSimpleTypeFunc\"`",
						},
						Type: &ast.Ident{Name: "int"},
					},
				},
			},
			want: &ds.RecordPackage{
				Server: ds.ServerDeclaration{},
				Namespace: ds.NamespaceDeclaration{
					ModuleName: "github.com/mailru/activerecord/internal/pkg/parser",
				},
				Fields:          []ds.FieldDeclaration{},
				FieldsMap:       map[string]int{},
				FieldsObjectMap: map[string]ds.FieldObject{},
				Indexes:         []ds.IndexDeclaration{},
				IndexMap:        map[string]int{},
				SelectorMap:     map[string]int{},
				Backends:        []string{},
				SerializerMap:   map[string]ds.SerializerDeclaration{},
				MutatorMap: map[string]ds.MutatorDeclaration{
					"FooMutatorField": {
						Name:       "FooMutatorField",
						Pkg:        "github.com/mailru/activerecord/internal/pkg/conv",
						Type:       "*foo.Foo",
						ImportName: "mutatorFooMutatorField",
						Update:     "updateFunc,param1,param2",
						Replace:    "replaceFunc",
						PartialFields: []ds.PartialFieldDeclaration{
							{Name: "Key", Type: "string", MappingKeyName: "Key"},
							{Name: "Bar", Type: "ds.AppInfo", MappingKeyName: "Bar"},
							{Name: "BeerData", Type: "[]foo.Beer", MappingKeyName: "beer_data"},
							{Name: "MapData", Type: "map[string]any", MappingKeyName: "MapData"},
						},
					},
					"SimpleTypeMutatorField": {
						Name:       "SimpleTypeMutatorField",
						Type:       "int",
						ImportName: "mutatorSimpleTypeMutatorField",
						Update:     "updateSimpleTypeFunc",
					},
				},
				ImportPackage: ds.ImportPackage{
					Imports: []ds.ImportDeclaration{
						{Path: "github.com/mailru/activerecord/internal/pkg/parser/testdata/foo"},
						{Path: "github.com/mailru/activerecord/internal/pkg/conv", ImportName: "mutatorFooMutatorField"},
						{Path: "github.com/mailru/activerecord/internal/pkg/parser/testdata/ds"},
					},
					ImportMap: map[string]int{
						"github.com/mailru/activerecord/internal/pkg/conv":                1,
						"github.com/mailru/activerecord/internal/pkg/parser/testdata/foo": 0,
						"github.com/mailru/activerecord/internal/pkg/parser/testdata/ds":  2,
					},
					ImportPkgMap: map[string]int{
						"mutatorFooMutatorField": 1,
						"ds":                     2,
						"foo":                    0,
					},
				},
				TriggerMap:    map[string]ds.TriggerDeclaration{},
				FlagMap:       map[string]ds.FlagDeclaration{},
				ProcOutFields: map[int]ds.ProcFieldDeclaration{},
				ProcFieldsMap: map[string]int{},
				LinkedStructsMap: map[string]ds.LinkedPackageDeclaration{
					"ds": {
						Types: map[string]struct{}{"AppInfo": {}},
						Import: struct {
							Imports      []ds.ImportDeclaration
							ImportMap    map[string]int
							ImportPkgMap map[string]int
						}{Imports: []ds.ImportDeclaration{}, ImportMap: map[string]int{}, ImportPkgMap: map[string]int{}},
					},
					"foo": {
						Types: map[string]struct{}{"Beer": {}, "Foo": {}},
						Import: struct {
							Imports      []ds.ImportDeclaration
							ImportMap    map[string]int
							ImportPkgMap map[string]int
						}{
							Imports: []ds.ImportDeclaration{
								{Path: "github.com/mailru/activerecord/internal/pkg/parser/testdata/ds"},
							},
							ImportMap: map[string]int{
								"github.com/mailru/activerecord/internal/pkg/parser/testdata/ds": 0,
							},
							ImportPkgMap: map[string]int{
								"ds": 0,
							},
						},
					},
				},
				ImportStructFieldsMap: map[string][]ds.PartialFieldDeclaration{
					"ds.AppInfo": {
						{Name: "appName", Type: "string", MappingKeyName: "appName"},
						{Name: "version", Type: "string", MappingKeyName: "version"},
						{Name: "buildTime", Type: "string", MappingKeyName: "buildTime"},
						{Name: "buildOS", Type: "string", MappingKeyName: "buildOS"},
						{Name: "buildCommit", Type: "string", MappingKeyName: "buildCommit"},
						{Name: "generateTime", Type: "string", MappingKeyName: "generateTime"},
					},
					"foo.Foo": {
						{Name: "Key", Type: "string", MappingKeyName: "Key"},
						{Name: "Bar", Type: "ds.AppInfo", MappingKeyName: "Bar"},
						{Name: "BeerData", Type: "[]foo.Beer", MappingKeyName: "beer_data"},
						{Name: "MapData", Type: "map[string]any", MappingKeyName: "MapData"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "not imported package for mutator type",
			args: args{
				fields: []*ast.Field{
					{
						Names: []*ast.Ident{{Name: "Foo"}},
						Tag:   &ast.BasicLit{Value: "`ar:\"pkg:github.com/mailru/activerecord/notexistsfolder\"`"},
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
			dst, err := NewRecordPackage(t)
			require.NoError(t, err)

			if err := parser.ParseMutators(dst, tt.args.fields); (err != nil) != tt.wantErr {
				t.Errorf("ParseMutators() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				assert.Check(t, cmp.DeepEqual(dst, tt.want), "Invalid response package, test `%s`", tt.name)
			}
		})
	}
}
