package parser

import (
	"go/ast"
	"reflect"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
)

func TestParseImport(t *testing.T) {
	rp := ds.NewRecordPackage()
	type args struct {
		dst        *ds.RecordPackage
		importSpec *ast.ImportSpec
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    *ds.RecordPackage
	}{
		{
			name: "simple import",
			args: args{
				dst: rp,
				importSpec: &ast.ImportSpec{
					Path: &ast.BasicLit{
						Value: `"github.com/mailru/activerecord-cookbook.git/example/model/dictionary"`,
					},
				},
			},
			want: &ds.RecordPackage{
				Server: ds.ServerDeclaration{
					Host:    "",
					Port:    "",
					Timeout: 0,
				},
				Namespace: ds.NamespaceDeclaration{
					ObjectName:  "",
					PublicName:  "",
					PackageName: "",
				},
				Backends:        []string{},
				ProcFieldsMap:   map[string]int{},
				ProcOutFields:   map[int]ds.ProcFieldDeclaration{},
				Fields:          []ds.FieldDeclaration{},
				FieldsMap:       map[string]int{},
				FieldsObjectMap: map[string]ds.FieldObject{},
				Indexes:         []ds.IndexDeclaration{},
				IndexMap:        map[string]int{},
				SelectorMap:     map[string]int{},
				Imports: []ds.ImportDeclaration{
					{
						Path: "github.com/mailru/activerecord-cookbook.git/example/model/dictionary",
					},
				},
				ImportMap:             map[string]int{"github.com/mailru/activerecord-cookbook.git/example/model/dictionary": 0},
				ImportPkgMap:          map[string]int{"dictionary": 0},
				SerializerMap:         map[string]ds.SerializerDeclaration{},
				TriggerMap:            map[string]ds.TriggerDeclaration{},
				FlagMap:               map[string]ds.FlagDeclaration{},
				MutatorMap:            map[string]ds.MutatorDeclaration{},
				ImportStructFieldsMap: map[string][]ds.PartialFieldDeclaration{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ParseImport(tt.args.dst, tt.args.importSpec); (err != nil) != tt.wantErr {
				t.Errorf("ParseImport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(tt.args.dst, tt.want) {
				t.Errorf("ParseImport() = %+v, wantErr %+v", tt.args.dst, tt.want)
			}
		})
	}
}
