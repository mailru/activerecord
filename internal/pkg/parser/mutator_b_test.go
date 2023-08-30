package parser_test

import (
	"fmt"
	"go/ast"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/internal/pkg/parser"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

type Beer struct{}

type Foo struct {
	Bar      ds.AppInfo
	BeerData []Beer
	MapData  map[string]any
}

func NewRecordPackage(t *testing.T) (*ds.RecordPackage, error) {
	dst := ds.NewRecordPackage()
	dst.Namespace.ModuleName = "github.com/mailru/activerecord"

	if _, err := dst.AddImport("github.com/mailru/activerecord", "parser_test"); err != nil {
		return nil, fmt.Errorf("can't create test package: %w", err)
	}

	if _, err := dst.AddImport("github.com/mailru/activerecord/../ds"); err != nil {
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
								X:   &ast.Ident{Name: "parser_test"},
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
				Namespace: ds.NamespaceDeclaration{
					ModuleName: "github.com/mailru/activerecord",
				},
				Imports: []ds.ImportDeclaration{
					{Path: "github.com/mailru/activerecord", ImportName: "parser_test"},
					{Path: "github.com/mailru/activerecord/../ds"},
					{Path: "github.com/mailru/activerecord/internal/pkg/conv", ImportName: "mutatorFooMutatorField"},
				},
				ImportMap: map[string]int{
					"github.com/mailru/activerecord/internal/pkg/conv": 2,
					"github.com/mailru/activerecord/../ds":             1,
					"github.com/mailru/activerecord":                   0,
				},
				ImportPkgMap: map[string]int{
					"mutatorFooMutatorField": 2,
					"ds":                     1,
					"parser_test":            0,
				},
				MutatorMap: map[string]ds.MutatorDeclaration{
					"FooMutatorField": {
						Name:       "FooMutatorField",
						Pkg:        "github.com/mailru/activerecord/internal/pkg/conv",
						Type:       "*parser_test.Foo",
						ImportName: "mutatorFooMutatorField",
						Update:     "updateFunc,param1,param2",
						Replace:    "replaceFunc",
						PartialFields: []ds.PartialFieldDeclaration{
							{Parent: "Foo", Name: "Bar", Type: "ds.AppInfo"},
							{Parent: "Foo", Name: "BeerData", Type: "[]Beer"},
							{Parent: "Foo", Name: "MapData", Type: "map[string]any"},
						},
					},
					"SimpleTypeMutatorField": {
						Name:       "SimpleTypeMutatorField",
						Type:       "int",
						ImportName: "mutatorSimpleTypeMutatorField",
						Update:     "updateSimpleTypeFunc",
					},
				},
				ImportStructFieldsMap: map[string][]ds.PartialFieldDeclaration{
					"ds.AppInfo": {
						{Parent: "AppInfo", Name: "appName", Type: "string"},
						{Parent: "AppInfo", Name: "version", Type: "string"},
						{Parent: "AppInfo", Name: "buildTime", Type: "string"},
						{Parent: "AppInfo", Name: "buildOS", Type: "string"},
						{Parent: "AppInfo", Name: "buildCommit", Type: "string"},
						{Parent: "AppInfo", Name: "generateTime", Type: "string"},
					},
					"parser_test.Foo": {
						{Parent: "Foo", Name: "Bar", Type: "ds.AppInfo"},
						{Parent: "Foo", Name: "BeerData", Type: "[]Beer"},
						{Parent: "Foo", Name: "MapData", Type: "map[string]any"},
					},
				},
				Server:          ds.ServerDeclaration{},
				Fields:          []ds.FieldDeclaration{},
				FieldsMap:       map[string]int{},
				FieldsObjectMap: map[string]ds.FieldObject{},
				Indexes:         []ds.IndexDeclaration{},
				IndexMap:        map[string]int{},
				SelectorMap:     map[string]int{},
				Backends:        []string{},
				SerializerMap:   map[string]ds.SerializerDeclaration{},
				TriggerMap:      map[string]ds.TriggerDeclaration{},
				FlagMap:         map[string]ds.FlagDeclaration{},
				ProcFieldsMap:   map[string]int{},
				ProcOutFields:   map[int]ds.ProcFieldDeclaration{},
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
