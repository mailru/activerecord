package parser

import (
	"go/ast"
	"go/token"
	"reflect"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/pkg/octopus"
)

func Test_parseDoc(t *testing.T) {
	type args struct {
		dst  *ds.RecordPackage
		docs *ast.CommentGroup
	}

	tests := []struct {
		name    string
		args    args
		want    *ds.RecordPackage
		wantErr bool
	}{
		{
			name: "doc",
			args: args{
				dst: ds.NewRecordPackage(),
				docs: &ast.CommentGroup{
					List: []*ast.Comment{
						{Text: `//ar:serverHost:127.0.0.1;serverPort:11011;serverTimeout:500`},
						{Text: `//ar:namespace:5`},
						{Text: `//ar:backend:octopus`},
					},
				},
			},
			wantErr: false,
			want: &ds.RecordPackage{
				Server: ds.ServerDeclaration{
					Host:    "127.0.0.1",
					Port:    "11011",
					Timeout: 500,
				},
				Namespace: ds.NamespaceDeclaration{
					Num:         5,
					PublicName:  "",
					PackageName: "",
				},
				Backends:        []string{"octopus"},
				Fields:          []ds.FieldDeclaration{},
				FieldsMap:       map[string]int{},
				ProcFieldsMap:   map[string]int{},
				FieldsObjectMap: map[string]ds.FieldObject{},
				Indexes:         []ds.IndexDeclaration{},
				IndexMap:        map[string]int{},
				SelectorMap:     map[string]int{},
				Imports:         []ds.ImportDeclaration{},
				ImportMap:       map[string]int{},
				ImportPkgMap:    map[string]int{},
				SerializerMap:   map[string]ds.SerializerDeclaration{},
				TriggerMap:      map[string]ds.TriggerDeclaration{},
				FlagMap:         map[string]ds.FlagDeclaration{},
			},
		},
		{
			name: "docComment",
			args: args{
				dst: ds.NewRecordPackage(),
				docs: &ast.CommentGroup{
					List: []*ast.Comment{
						{Text: `//blablabla`},
					},
				},
			},
			wantErr: false,
			want:    ds.NewRecordPackage(),
		},
		{
			name: "docError",
			args: args{
				dst: ds.NewRecordPackage(),
				docs: &ast.CommentGroup{
					List: []*ast.Comment{
						{Text: `//ar:fdgsdsfgdf`},
					},
				},
			},
			wantErr: true,
			want:    ds.NewRecordPackage(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parseDoc(tt.args.dst, string(Fields), tt.args.docs); (err != nil) != tt.wantErr {
				t.Errorf("parseDoc() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(tt.args.dst, tt.want) {
				t.Errorf("parseDoc() dst = %+v, want %+v", tt.args.dst, tt.want)
				return
			}
		})
	}
}

func Test_parseGen(t *testing.T) {
	type args struct {
		dst  *ds.RecordPackage
		genD *ast.GenDecl
	}
	w := ds.NewRecordPackage()
	w.Backends = []string{"octopus"}
	w.Namespace = ds.NamespaceDeclaration{
		Num:         5,
		PublicName:  "Baz",
		PackageName: "baz",
	}
	w.Server = ds.ServerDeclaration{
		Timeout: 500,
		Host:    "127.0.0.1",
		Port:    "11011",
	}
	wLinked := ds.NewRecordPackage()
	wLinked.Backends = []string{"octopus"}
	wLinked.Namespace = ds.NamespaceDeclaration{
		Num:         5,
		PublicName:  "Foo",
		PackageName: "foo",
	}
	wLinked.Server = ds.ServerDeclaration{
		Timeout: 500,
		Host:    "127.0.0.1",
		Port:    "11011",
	}
	wLinked.FieldsMap["ID"] = len(wLinked.Fields)
	wLinked.Fields = append(wLinked.Fields, ds.FieldDeclaration{
		Name:       "ID",
		Format:     octopus.Int,
		PrimaryKey: true,
		Mutators:   []ds.FieldMutator{},
		Size:       0,
		Serializer: []string{},
		ObjectLink: "",
	})
	wLinked.FieldsMap["BarID"] = len(wLinked.Fields)
	wLinked.Fields = append(wLinked.Fields, ds.FieldDeclaration{
		Name:       "BarID",
		Format:     octopus.Int,
		PrimaryKey: false,
		Mutators:   []ds.FieldMutator{},
		Size:       0,
		Serializer: []string{},
		ObjectLink: "Bar",
	})
	wLinked.FieldsObjectMap["Bar"] = ds.FieldObject{
		Name:       "Bar",
		Key:        "ID",
		ObjectName: "bar",
		Field:      "BarID",
		Unique:     true,
	}
	wantIndex := ds.IndexDeclaration{
		Name:      "ID",
		Primary:   true,
		Fields:    []int{0},
		FieldsMap: map[string]ds.IndexField{"ID": {IndField: 0, Order: 0}},
		Selector:  "SelectByID",
		Unique:    true,
		Num:       0,
	}
	wLinked.IndexMap[wantIndex.Name] = len(wLinked.Indexes)
	wLinked.SelectorMap[wantIndex.Selector] = len(wLinked.Indexes)
	wLinked.Indexes = append(wLinked.Indexes, wantIndex)

	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    *ds.RecordPackage
	}{
		{
			name: "private and public names",
			args: args{
				dst: ds.NewRecordPackage(),
				genD: &ast.GenDecl{
					Tok: token.TYPE,
					Doc: &ast.CommentGroup{
						List: []*ast.Comment{
							{Text: `//ar:serverHost:127.0.0.1;serverPort:11011;serverTimeout:500`},
							{Text: `//ar:namespace:5`},
							{Text: `//ar:backend:octopus`},
						},
					},
					Specs: []ast.Spec{
						&ast.TypeSpec{
							Type: &ast.StructType{
								Fields: &ast.FieldList{
									List: []*ast.Field{},
								},
							},
							Name: &ast.Ident{
								Name: "FieldsBaz",
							},
						},
					},
				},
			},
			wantErr: false,
			want:    w,
		},
		{
			name: "Invalid names in struct",
			args: args{
				dst: ds.NewRecordPackage(),
				genD: &ast.GenDecl{
					Tok: token.TYPE,
					Doc: &ast.CommentGroup{
						List: []*ast.Comment{
							{Text: `//ar:serverHost:127.0.0.1;serverPort:11011;serverTimeout:500`},
							{Text: `//ar:namespace:5`},
							{Text: `//ar:backend:octopus`},
						},
					},
					Specs: []ast.Spec{
						&ast.TypeSpec{
							Type: &ast.StructType{
								Fields: &ast.FieldList{
									List: []*ast.Field{},
								},
							},
							Name: &ast.Ident{
								Name: "FieldsBaz",
							},
						},
						&ast.TypeSpec{
							Type: &ast.StructType{
								Fields: &ast.FieldList{
									List: []*ast.Field{},
								},
							},
							Name: &ast.Ident{
								Name: "IndexesInvalid",
							},
						},
					},
				},
			},
			wantErr: true,
			want:    w,
		},
		{
			name: "linked objects",
			args: args{
				dst: ds.NewRecordPackage(),
				genD: &ast.GenDecl{
					Tok: token.TYPE,
					Doc: &ast.CommentGroup{
						List: []*ast.Comment{
							{Text: `//ar:serverHost:127.0.0.1;serverPort:11011;serverTimeout:500`},
							{Text: `//ar:namespace:5`},
							{Text: `//ar:backend:octopus`},
						},
					},
					Specs: []ast.Spec{
						&ast.TypeSpec{
							Type: &ast.StructType{
								Fields: &ast.FieldList{
									List: []*ast.Field{
										{
											Names: []*ast.Ident{{Name: "ID"}},
											Type:  &ast.Ident{Name: "int"},
											Tag:   &ast.BasicLit{Value: "`" + `ar:"primary_key"` + "`"},
										},
										{
											Names: []*ast.Ident{{Name: "BarID"}},
											Type:  &ast.Ident{Name: "int"},
											Tag:   &ast.BasicLit{Value: "`" + `ar:""` + "`"},
										},
									},
								},
							},
							Name: &ast.Ident{
								Name: "FieldsFoo",
							},
						},
						&ast.TypeSpec{
							Type: &ast.StructType{
								Fields: &ast.FieldList{
									List: []*ast.Field{
										{
											Names: []*ast.Ident{{Name: "Bar"}},
											Type:  &ast.Ident{Name: "bool"},
											Tag:   &ast.BasicLit{Value: "`" + `ar:"key:ID;object:bar;field:BarID"` + "`"},
										},
									},
								},
							},
							Name: &ast.Ident{
								Name: "FieldsObjectFoo",
							},
						},
					},
				},
			},
			wantErr: false,
			want:    wLinked,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parseGen(tt.args.dst, tt.args.genD); (err != nil) != tt.wantErr {
				t.Errorf("parseGen() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(tt.args.dst, tt.want) {
				t.Errorf("parseGen() %+v, want %+v", tt.args.dst, tt.want)
			}
		})
	}
}

func Test_parseAst(t *testing.T) {
	type args struct {
		pkgName string
		decls   []ast.Decl
		rc      *ds.RecordPackage
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
		want    *ds.RecordPackage
	}{
		{
			name: "private and public names",
			args: args{
				rc: ds.NewRecordPackage(),
				decls: []ast.Decl{
					&ast.GenDecl{
						Tok: token.TYPE,
						Doc: &ast.CommentGroup{
							List: []*ast.Comment{
								{Text: `//ar:serverHost:127.0.0.1;serverPort:11011;serverTimeout:500`},
								{Text: `//ar:namespace:5`},
								{Text: `//ar:backend:octopus`},
							},
						},
						Specs: []ast.Spec{
							&ast.TypeSpec{
								Type: &ast.StructType{
									Fields: &ast.FieldList{
										List: []*ast.Field{},
									},
								},
								Name: &ast.Ident{
									Name: "FieldsBaz",
								},
							},
						},
					},
				},
			},
			wantErr: false,
			want: &ds.RecordPackage{
				Server:          ds.ServerDeclaration{Timeout: 500, Host: "127.0.0.1", Port: "11011"},
				Namespace:       ds.NamespaceDeclaration{Num: 5, PublicName: "Baz", PackageName: "baz"},
				ProcFieldsMap:   map[string]int{},
				Fields:          []ds.FieldDeclaration{},
				FieldsMap:       map[string]int{},
				FieldsObjectMap: map[string]ds.FieldObject{},
				Indexes:         []ds.IndexDeclaration{},
				IndexMap:        map[string]int{},
				SelectorMap:     map[string]int{},
				Backends:        []string{"octopus"},
				SerializerMap:   map[string]ds.SerializerDeclaration{},
				Imports:         []ds.ImportDeclaration{},
				ImportMap:       map[string]int{},
				ImportPkgMap:    map[string]int{},
				TriggerMap:      map[string]ds.TriggerDeclaration{},
				FlagMap:         map[string]ds.FlagDeclaration{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := parseAst(tt.args.pkgName, tt.args.decls, tt.args.rc); (err != nil) != tt.wantErr {
				t.Errorf("parseAst() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(tt.args.rc, tt.want) {
				t.Errorf("parseAst() %+v, want %+v", tt.args.rc, tt.want)
			}
		})
	}
}
