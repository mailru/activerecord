package generator

import (
	"reflect"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/internal/pkg/testutil"
)

func TestGenerate(t *testing.T) {
	type args struct {
		appInfo      string
		cl           ds.RecordPackage
		linkedObject map[string]ds.RecordPackage
	}
	tests := []struct {
		name    string
		args    args
		wantRet []GenerateFile
		wantErr bool
	}{
		{
			name: "Filename",
			args: args{
				appInfo: testutil.TestAppInfo.String(),
				cl: ds.RecordPackage{
					Server: ds.ServerDeclaration{
						Host:    "127.0.0.1",
						Port:    "11011",
						Timeout: 500,
					},
					Namespace: ds.NamespaceDeclaration{
						ObjectName:  "5",
						PublicName:  "Bar",
						PackageName: "bar",
					},
					Backends: []string{"octopus"},
					Fields: []ds.FieldDeclaration{
						{Name: "Field1", Format: "int", PrimaryKey: true, Mutators: []string{}, Size: 5, Serializer: []string{}},
					},
					FieldsMap:       map[string]int{"Field1": 0},
					FieldsObjectMap: map[string]ds.FieldObject{},
					Indexes: []ds.IndexDeclaration{
						{
							Name:     "Field1",
							Num:      0,
							Selector: "SelectByField1",
							Fields:   []int{0},
							FieldsMap: map[string]ds.IndexField{
								"Field1": {IndField: 0, Order: 0},
							},
							Primary: true,
							Unique:  true,
							Type:    "int",
						},
					},
					IndexMap:      map[string]int{"Field1": 0},
					SelectorMap:   map[string]int{"SelectByField1": 0},
					ImportPackage: ds.NewImportPackage(),
					SerializerMap: map[string]ds.SerializerDeclaration{},
					TriggerMap:    map[string]ds.TriggerDeclaration{},
					FlagMap:       map[string]ds.FlagDeclaration{},
				},
				linkedObject: map[string]ds.RecordPackage{},
			},
			wantRet: []GenerateFile{
				{
					Dir:     "bar",
					Name:    "octopus.go",
					Backend: "octopus",
					Data:    []byte{},
				},
				{
					Dir:     "bar",
					Name:    "mock.go",
					Backend: "octopus",
					Data:    []byte{},
				},
				{
					Dir:     "bar",
					Name:    "fixture.go",
					Backend: "octopus",
					Data:    []byte{},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRet, err := Generate(tt.args.appInfo, tt.args.cl, tt.args.linkedObject)
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Testing in backend specific tests
			for iGotRet := range gotRet {
				gotRet[iGotRet].Data = []byte{}
			}

			got := filesByName(gotRet)

			for name, file := range filesByName(tt.wantRet) {
				if !reflect.DeepEqual(got[name], file) {
					t.Errorf("Generate() = %v, want %v", gotRet, tt.wantRet)
				}
			}
		})
	}
}

func filesByName(files []GenerateFile) map[string]GenerateFile {
	ret := make(map[string]GenerateFile, len(files))
	for _, file := range files {
		ret[file.Name] = file
	}
	return ret
}
