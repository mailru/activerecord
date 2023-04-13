package parser_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/internal/pkg/parser"
	"github.com/mailru/activerecord/internal/pkg/testutil"
)

func TestParse(t *testing.T) {
	tempDirs := testutil.InitTmps()
	defer tempDirs.Defer()

	textTestPkg := `package repository

//ar:serverHost:127.0.0.1;serverPort:11111;serverTimeout:500
//ar:namespace:2
//ar:backend:octopus
type FieldsFoo struct {
	Field1    int  ` + "`" + `ar:"size:5"` + "`" + `
	Field2    string  ` + "`" + `ar:"size:5"` + "`" + `
}

type (
	IndexesFoo struct {
		Field1Field2 bool ` + "`" + `ar:"fields:Field1,Field2;primary_key"` + "`" + `
	}
	IndexPartsFoo struct {
		Field1Part bool ` + "`" + `ar:"index:Field1Field2;fieldnum:1;selector:SelectByField1"` + "`" + `
	}
)

type TriggersFoo struct {
	RepairTuple bool ` + "`" + `ar:"pkg:github.com/mailru/activerecord-cookbook.git/example/model/repository/repair;func:Promoperiod;param:Defaults"` + "`" + `
}
`

	srcRoot, err := tempDirs.AddTempDir()
	if err != nil {
		t.Errorf("can't initialize dir: %s", err)
		return
	}

	src := filepath.Join(srcRoot, "model/repository/decl")

	if err = os.MkdirAll(src, 0755); err != nil {
		t.Errorf("prepare test files error: %s", err)
		return
	}

	if err = os.WriteFile(filepath.Join(src, "foo.go"), []byte(textTestPkg), 0644); err != nil {
		t.Errorf("prepare test files error: %s", err)
		return
	}

	type args struct {
		srcFileName string
		rc          *ds.RecordPackage
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    *ds.RecordPackage
	}{
		{
			name: "simple decl",
			args: args{
				srcFileName: filepath.Join(src, "foo.go"),
				rc:          ds.NewRecordPacakge(),
			},
			wantErr: false,
			want: &ds.RecordPackage{
				Namespace: ds.NamespaceDeclaration{Num: 2, PublicName: "Foo", PackageName: "foo"},
				Server:    ds.ServerDeclaration{Timeout: 500, Host: "127.0.0.1", Port: "11111"},
				Fields: []ds.FieldDeclaration{
					{Name: "Field1", Format: "int", PrimaryKey: true, Mutators: []ds.FieldMutator{}, Size: 5, Serializer: []string{}},
					{Name: "Field2", Format: "string", PrimaryKey: true, Mutators: []ds.FieldMutator{}, Size: 5, Serializer: []string{}},
				},
				FieldsMap:       map[string]int{"Field1": 0, "Field2": 1},
				FieldsObjectMap: map[string]ds.FieldObject{},
				Indexes: []ds.IndexDeclaration{
					{
						Name:     "Field1Field2",
						Num:      0,
						Selector: "SelectByField1Field2",
						Fields:   []int{0, 1},
						FieldsMap: map[string]ds.IndexField{
							"Field1": {IndField: 0, Order: 0},
							"Field2": {IndField: 1, Order: 0},
						},
						Primary: true,
						Unique:  true,
					},
					{
						Name:     "Field1Part",
						Num:      1,
						Selector: "SelectByField1",
						Fields:   []int{0},
						FieldsMap: map[string]ds.IndexField{
							"Field1": {IndField: 0, Order: 0},
						},
						Primary: false,
						Unique:  false,
					},
				},
				IndexMap:      map[string]int{"Field1Field2": 0, "Field1Part": 1},
				SelectorMap:   map[string]int{"SelectByField1": 1, "SelectByField1Field2": 0},
				Backends:      []string{"octopus"},
				SerializerMap: map[string]ds.SerializerDeclaration{},
				Imports: []ds.ImportDeclaration{
					{
						Path:       "github.com/mailru/activerecord-cookbook.git/example/model/repository/repair",
						ImportName: "triggerRepairTuple",
					},
				},
				ImportMap:    map[string]int{"github.com/mailru/activerecord-cookbook.git/example/model/repository/repair": 0},
				ImportPkgMap: map[string]int{"triggerRepairTuple": 0},
				TriggerMap: map[string]ds.TriggerDeclaration{
					"RepairTuple": {
						Name:       "RepairTuple",
						Pkg:        "github.com/mailru/activerecord-cookbook.git/example/model/repository/repair",
						Func:       "Promoperiod",
						ImportName: "triggerRepairTuple",
						Params:     map[string]bool{"Defaults": true},
					},
				},
				FlagMap: map[string]ds.FlagDeclaration{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parser.Parse(tt.args.srcFileName, tt.args.rc); (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(tt.args.rc, tt.want) {
				t.Errorf("Parse() = %+v, want %+v", tt.args.rc, tt.want)
			}
		})
	}
}
