package checker

import (
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
)

func Test_checkBackend(t *testing.T) {
	rcOctopus := ds.NewRecordPackage()
	rcOctopus.Backends = []string{"octopus"}
	rcMany := ds.NewRecordPackage()
	rcMany.Backends = []string{"octopus", "postgres"}

	type args struct {
		cl *ds.RecordPackage
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "emptyBack", args: args{cl: ds.NewRecordPackage()}, wantErr: true},
		{name: "oneBack", args: args{cl: rcOctopus}, wantErr: false},
		{name: "manyBack", args: args{cl: rcMany}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkBackend(tt.args.cl); (err != nil) != tt.wantErr {
				t.Errorf("CheckBackend() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_checkLinkedObject(t *testing.T) {
	rp := ds.NewRecordPackage()
	rpLinked := ds.NewRecordPackage()

	err := rpLinked.AddFieldObject(ds.FieldObject{
		Name:       "Foo",
		Key:        "ID",
		ObjectName: "bar",
		Field:      "barID",
		Unique:     false,
	})
	if err != nil {
		t.Errorf("can't prepare test data: %s", err)
		return
	}

	type args struct {
		cl            *ds.RecordPackage
		linkedObjects map[string]string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "without linked obj", args: args{cl: rp, linkedObjects: map[string]string{}}, wantErr: false},
		{name: "no linked obj", args: args{cl: rpLinked, linkedObjects: map[string]string{}}, wantErr: true},
		{name: "normal linked obj", args: args{cl: rpLinked, linkedObjects: map[string]string{"bar": "bar"}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkLinkedObject(tt.args.cl, tt.args.linkedObjects); (err != nil) != tt.wantErr {
				t.Errorf("checkLinkedObject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_checkNamespace(t *testing.T) {
	type args struct {
		ns *ds.NamespaceDeclaration
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "normal namespace",
			args: args{
				ns: &ds.NamespaceDeclaration{
					Num:         0,
					PublicName:  "Foo",
					PackageName: "foo",
				},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			args: args{
				ns: &ds.NamespaceDeclaration{
					Num:         0,
					PublicName:  "",
					PackageName: "foo",
				},
			},
			wantErr: true,
		},
		{
			name: "empty package",
			args: args{
				ns: &ds.NamespaceDeclaration{
					Num:         0,
					PublicName:  "Foo",
					PackageName: "",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkNamespace(tt.args.ns); (err != nil) != tt.wantErr {
				t.Errorf("checkNamespace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_checkFields(t *testing.T) {
	type args struct {
		cl ds.RecordPackage
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "empty fields",
			args: args{
				cl: ds.RecordPackage{
					Fields:     []ds.FieldDeclaration{},
					ProcFields: []ds.ProcFieldDeclaration{},
				},
			},
			wantErr: true,
		},
		{
			name: "empty format",
			args: args{
				cl: ds.RecordPackage{
					Fields: []ds.FieldDeclaration{
						{
							Name: "Foo",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			args: args{
				cl: ds.RecordPackage{
					Fields: []ds.FieldDeclaration{
						{
							Name:   "Foo",
							Format: "[]int",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no primary",
			args: args{
				cl: ds.RecordPackage{
					Fields: []ds.FieldDeclaration{
						{
							Name:   "Foo",
							Format: "int",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "normal field",
			args: args{
				cl: ds.RecordPackage{
					Fields: []ds.FieldDeclaration{
						{
							Name:       "Foo",
							Format:     "int",
							PrimaryKey: true,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "fields conflict with links",
			args: args{
				cl: ds.RecordPackage{
					Fields: []ds.FieldDeclaration{
						{
							Name:       "Foo",
							Format:     "int",
							PrimaryKey: true,
						},
					},
					FieldsObjectMap: map[string]ds.FieldObject{
						"Foo": {},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mutators and primary",
			args: args{
				cl: ds.RecordPackage{
					Fields: []ds.FieldDeclaration{
						{
							Name:       "Foo",
							Format:     "int",
							PrimaryKey: true,
							Mutators: []ds.FieldMutator{
								"fmut",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "serializer not declared",
			args: args{
				cl: ds.RecordPackage{
					Fields: []ds.FieldDeclaration{
						{
							Name:       "Foo",
							Format:     "int",
							PrimaryKey: true,
						},
						{
							Name:   "Foo",
							Format: "int",
							Mutators: []ds.FieldMutator{
								"fmut",
							},
							Serializer: []string{
								"fser",
							},
						},
					},
					SerializerMap: map[string]ds.SerializerDeclaration{
						"fser": {},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "serializer and mutators",
			args: args{
				cl: ds.RecordPackage{
					Fields: []ds.FieldDeclaration{
						{
							Name:       "Foo",
							Format:     "int",
							PrimaryKey: true,
						},
						{
							Name:   "Foo",
							Format: "int",
							Mutators: []ds.FieldMutator{
								"fmut",
							},
							Serializer: []string{
								"fser",
							},
						},
					},
					SerializerMap: map[string]ds.SerializerDeclaration{
						"fser": {},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "mutators and links",
			args: args{
				cl: ds.RecordPackage{
					Fields: []ds.FieldDeclaration{
						{
							Name:       "Foo",
							Format:     "int",
							PrimaryKey: true,
						},
						{
							Name:   "Foo",
							Format: "int",
							Mutators: []ds.FieldMutator{
								"fmut",
							},
							ObjectLink: "Bar",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "serializer and links",
			args: args{
				cl: ds.RecordPackage{
					Fields: []ds.FieldDeclaration{
						{
							Name:       "Foo",
							Format:     "int",
							PrimaryKey: true,
						},
						{
							Name:   "Foo",
							Format: "int",
							Serializer: []string{
								"fser",
							},
							ObjectLink: "Bar",
						},
					},
					SerializerMap: map[string]ds.SerializerDeclaration{
						"fser": {},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkFields(&tt.args.cl); (err != nil) != tt.wantErr {
				t.Errorf("checkFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
