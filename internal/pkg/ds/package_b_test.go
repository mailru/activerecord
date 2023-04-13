package ds_test

import (
	"reflect"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
)

func TestRecordPackage_FindImport(t *testing.T) {
	rc := ds.NewRecordPacakge()

	imp, err := rc.AddImport("go/ast")
	if err != nil {
		t.Errorf("add import: %s", err)
		return
	}

	type args struct {
		path string
	}
	tests := []struct {
		name    string
		rc      *ds.RecordPackage
		args    args
		want    ds.ImportDeclaration
		wantErr bool
	}{
		{
			name:    "getByPkg",
			rc:      rc,
			args:    args{path: "ast"},
			want:    ds.ImportDeclaration{},
			wantErr: true,
		},
		{
			name:    "getByName",
			rc:      rc,
			args:    args{path: "go/ast"},
			want:    imp,
			wantErr: false,
		},
		{
			name:    "Unknown",
			rc:      rc,
			args:    args{path: "bla"},
			want:    ds.ImportDeclaration{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.rc.FindImport(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecordPackage.FindImport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RecordPackage.FindImport() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRecordPackage_AddImport(t *testing.T) {
	rc := ds.NewRecordPacakge()

	type want struct {
		importDeclaration ds.ImportDeclaration
		RcImportPkgMap    map[string]int
		RcImportMap       map[string]int
	}

	type args struct {
		path       string
		importname []string
	}
	tests := []struct {
		name    string
		rc      *ds.RecordPackage
		args    args
		want    want
		wantErr bool
	}{
		{
			name: "simple add",
			rc:   rc,
			args: args{
				path: "go/ast",
			},
			want: want{
				importDeclaration: ds.ImportDeclaration{
					Path:       "go/ast",
					ImportName: "",
				},
				RcImportPkgMap: map[string]int{"ast": 0},
				RcImportMap:    map[string]int{"go/ast": 0},
			},
			wantErr: false,
		},
		{
			name: "duplicate by import name",
			rc:   rc,
			args: args{
				path: "another/ast",
			},
			want: want{
				importDeclaration: ds.ImportDeclaration{},
				RcImportPkgMap:    map[string]int{"ast": 0},
				RcImportMap:       map[string]int{"go/ast": 0},
			},
			wantErr: true,
		},
		{
			name: "import into another scope",
			rc:   rc,
			args: args{
				path:       "go/ast",
				importname: []string{"anotherast"},
			},
			want: want{
				importDeclaration: ds.ImportDeclaration{
					Path:       "go/ast",
					ImportName: "anotherast",
				},
				RcImportPkgMap: map[string]int{"anotherast": 1, "ast": 0},
				RcImportMap:    map[string]int{"go/ast": 1},
			},
			wantErr: false,
		},
		{
			name: "another import with empty importName",
			rc:   rc,
			args: args{
				path:       "github.com/mailru/activerecord-cookbook.git/example/model/dictionary",
				importname: []string{""},
			},
			want: want{
				importDeclaration: ds.ImportDeclaration{
					Path:       "github.com/mailru/activerecord-cookbook.git/example/model/dictionary",
					ImportName: "",
				},
				RcImportPkgMap: map[string]int{"anotherast": 1, "ast": 0, "dictionary": 2},
				RcImportMap:    map[string]int{"go/ast": 1, "github.com/mailru/activerecord-cookbook.git/example/model/dictionary": 2},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.rc.AddImport(tt.args.path, tt.args.importname...)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecordPackage.AddImport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want.importDeclaration) {
				t.Errorf("RecordPackage.AddImport() = %v, want %v", got, tt.want.importDeclaration)
				return
			}

			if !reflect.DeepEqual(tt.rc.ImportPkgMap, tt.want.RcImportPkgMap) {
				t.Errorf("RecordPackage.AddImport() ImportPkgMap = %+v, want %+v", tt.rc.ImportPkgMap, tt.want.RcImportPkgMap)
				return
			}

			if !reflect.DeepEqual(tt.rc.ImportMap, tt.want.RcImportMap) {
				t.Errorf("RecordPackage.AddImport() ImportMap = %+v, want %+v", tt.rc.ImportMap, tt.want.RcImportMap)
				return
			}
		})
	}
}

func TestRecordPackage_FindImportByPkg(t *testing.T) {
	rc := ds.NewRecordPacakge()

	imp, err := rc.AddImport("go/ast")
	if err != nil {
		t.Errorf("add import: %s", err)
		return
	}

	type args struct {
		pkg string
	}
	tests := []struct {
		name    string
		rc      *ds.RecordPackage
		args    args
		want    *ds.ImportDeclaration
		wantErr bool
	}{
		{
			name:    "getByPkg",
			rc:      rc,
			args:    args{pkg: "ast"},
			want:    &imp,
			wantErr: false,
		},
		{
			name:    "getByName",
			rc:      rc,
			args:    args{pkg: "go/ast"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Unknown",
			rc:      rc,
			args:    args{pkg: "bla"},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.rc.FindImportByPkg(tt.args.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecordPackage.FindImport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RecordPackage.FindImport() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRecordClass_AddField(t *testing.T) {
	type args struct {
		f ds.FieldDeclaration
	}

	rc := ds.NewRecordPacakge()

	tests := []struct {
		name    string
		fields  *ds.RecordPackage
		args    args
		wantErr bool
	}{
		{name: "newField", fields: rc, args: args{f: ds.FieldDeclaration{Name: "bla"}}, wantErr: false},
		{name: "dupField", fields: rc, args: args{f: ds.FieldDeclaration{Name: "bla"}}, wantErr: true},
		{name: "anyField", fields: rc, args: args{f: ds.FieldDeclaration{Name: "bla1"}}, wantErr: false},
		{name: "CaseField", fields: rc, args: args{f: ds.FieldDeclaration{Name: "Bla"}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &ds.RecordPackage{
				Server:          tt.fields.Server,
				Namespace:       tt.fields.Namespace,
				Fields:          tt.fields.Fields,
				FieldsMap:       tt.fields.FieldsMap,
				FieldsObjectMap: tt.fields.FieldsObjectMap,
				Indexes:         tt.fields.Indexes,
				IndexMap:        tt.fields.IndexMap,
				SelectorMap:     tt.fields.SelectorMap,
				Backends:        tt.fields.Backends,
				SerializerMap:   tt.fields.SerializerMap,
				Imports:         tt.fields.Imports,
				ImportMap:       tt.fields.ImportMap,
				TriggerMap:      tt.fields.TriggerMap,
				FlagMap:         tt.fields.FlagMap,
			}
			if err := rc.AddField(tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("RecordClass.AddField() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRecordClass_AddFieldObject(t *testing.T) {
	type args struct {
		f ds.FieldObject
	}

	rc := ds.NewRecordPacakge()

	tests := []struct {
		name    string
		fields  *ds.RecordPackage
		args    args
		wantErr bool
	}{
		{name: "newField", fields: rc, args: args{f: ds.FieldObject{Name: "bla"}}, wantErr: false},
		{name: "dupField", fields: rc, args: args{f: ds.FieldObject{Name: "bla"}}, wantErr: true},
		{name: "anyField", fields: rc, args: args{f: ds.FieldObject{Name: "bla1"}}, wantErr: false},
		{name: "CaseField", fields: rc, args: args{f: ds.FieldObject{Name: "Bla"}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &ds.RecordPackage{
				Server:          tt.fields.Server,
				Namespace:       tt.fields.Namespace,
				Fields:          tt.fields.Fields,
				FieldsMap:       tt.fields.FieldsMap,
				FieldsObjectMap: tt.fields.FieldsObjectMap,
				Indexes:         tt.fields.Indexes,
				IndexMap:        tt.fields.IndexMap,
				SelectorMap:     tt.fields.SelectorMap,
				Backends:        tt.fields.Backends,
				SerializerMap:   tt.fields.SerializerMap,
				Imports:         tt.fields.Imports,
				ImportMap:       tt.fields.ImportMap,
				TriggerMap:      tt.fields.TriggerMap,
				FlagMap:         tt.fields.FlagMap,
			}
			if err := rc.AddFieldObject(tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("RecordClass.AddFieldObject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func TestRecordClass_AddTrigger(t *testing.T) {
	type args struct {
		f ds.TriggerDeclaration
	}

	rc := ds.NewRecordPacakge()

	tests := []struct {
		name    string
		fields  *ds.RecordPackage
		args    args
		wantErr bool
	}{
		{name: "newField", fields: rc, args: args{f: ds.TriggerDeclaration{Name: "bla"}}, wantErr: false},
		{name: "dupField", fields: rc, args: args{f: ds.TriggerDeclaration{Name: "bla"}}, wantErr: true},
		{name: "anyField", fields: rc, args: args{f: ds.TriggerDeclaration{Name: "bla1"}}, wantErr: false},
		{name: "CaseField", fields: rc, args: args{f: ds.TriggerDeclaration{Name: "Bla"}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &ds.RecordPackage{
				Server:          tt.fields.Server,
				Namespace:       tt.fields.Namespace,
				Fields:          tt.fields.Fields,
				FieldsMap:       tt.fields.FieldsMap,
				FieldsObjectMap: tt.fields.FieldsObjectMap,
				Indexes:         tt.fields.Indexes,
				IndexMap:        tt.fields.IndexMap,
				SelectorMap:     tt.fields.SelectorMap,
				Backends:        tt.fields.Backends,
				SerializerMap:   tt.fields.SerializerMap,
				Imports:         tt.fields.Imports,
				ImportMap:       tt.fields.ImportMap,
				TriggerMap:      tt.fields.TriggerMap,
				FlagMap:         tt.fields.FlagMap,
			}
			if err := rc.AddTrigger(tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("RecordClass.AddTrigger() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func TestRecordClass_AddSerializer(t *testing.T) {
	type args struct {
		f ds.SerializerDeclaration
	}

	rc := ds.NewRecordPacakge()

	tests := []struct {
		name    string
		fields  *ds.RecordPackage
		args    args
		wantErr bool
	}{
		{name: "newField", fields: rc, args: args{f: ds.SerializerDeclaration{Name: "bla"}}, wantErr: false},
		{name: "dupField", fields: rc, args: args{f: ds.SerializerDeclaration{Name: "bla"}}, wantErr: true},
		{name: "anyField", fields: rc, args: args{f: ds.SerializerDeclaration{Name: "bla1"}}, wantErr: false},
		{name: "CaseField", fields: rc, args: args{f: ds.SerializerDeclaration{Name: "Bla"}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &ds.RecordPackage{
				Server:          tt.fields.Server,
				Namespace:       tt.fields.Namespace,
				Fields:          tt.fields.Fields,
				FieldsMap:       tt.fields.FieldsMap,
				FieldsObjectMap: tt.fields.FieldsObjectMap,
				Indexes:         tt.fields.Indexes,
				IndexMap:        tt.fields.IndexMap,
				SelectorMap:     tt.fields.SelectorMap,
				Backends:        tt.fields.Backends,
				SerializerMap:   tt.fields.SerializerMap,
				Imports:         tt.fields.Imports,
				ImportMap:       tt.fields.ImportMap,
				TriggerMap:      tt.fields.TriggerMap,
				FlagMap:         tt.fields.FlagMap,
			}
			if err := rc.AddSerializer(tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("RecordClass.AddSerializer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
func TestRecordClass_AddFlag(t *testing.T) {
	type args struct {
		f ds.FlagDeclaration
	}

	rc := ds.NewRecordPacakge()

	tests := []struct {
		name    string
		fields  *ds.RecordPackage
		args    args
		wantErr bool
	}{
		{name: "newField", fields: rc, args: args{f: ds.FlagDeclaration{Name: "bla"}}, wantErr: false},
		{name: "dupField", fields: rc, args: args{f: ds.FlagDeclaration{Name: "bla"}}, wantErr: true},
		{name: "anyField", fields: rc, args: args{f: ds.FlagDeclaration{Name: "bla1"}}, wantErr: false},
		{name: "CaseField", fields: rc, args: args{f: ds.FlagDeclaration{Name: "Bla"}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &ds.RecordPackage{
				Server:          tt.fields.Server,
				Namespace:       tt.fields.Namespace,
				Fields:          tt.fields.Fields,
				FieldsMap:       tt.fields.FieldsMap,
				FieldsObjectMap: tt.fields.FieldsObjectMap,
				Indexes:         tt.fields.Indexes,
				IndexMap:        tt.fields.IndexMap,
				SelectorMap:     tt.fields.SelectorMap,
				Backends:        tt.fields.Backends,
				SerializerMap:   tt.fields.SerializerMap,
				Imports:         tt.fields.Imports,
				ImportMap:       tt.fields.ImportMap,
				TriggerMap:      tt.fields.TriggerMap,
				FlagMap:         tt.fields.FlagMap,
			}
			if err := rc.AddFlag(tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("RecordClass.AddFlag() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
