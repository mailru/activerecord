package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/internal/pkg/testutil"
	"github.com/mailru/activerecord/pkg/octopus"
	"gotest.tools/assert"
	"gotest.tools/assert/cmp"
)

type args struct {
	appInfo    ds.AppInfo
	srcDir     string
	dstDir     string
	dstFixture string
	modName    string
}

func TestArGen_addRecordPackage(t *testing.T) {
	tempDirs := testutil.InitTmps()
	defer tempDirs.Defer()

	src, dst, err := tempDirs.CreateDirs(testutil.EmptyDstDir)
	if err != nil {
		t.Errorf("ArGen.Init() error = %v", err)
		return
	}

	argen, err := Init(context.Background(), &testutil.TestAppInfo, src, dst, "", "github.com/mailru/activerecord")
	if err != nil {
		t.Errorf("ArGen.Init() error = %v", err)
		return
	}

	emptyRP := ds.NewRecordPackage()

	type args struct {
		pkgName string
	}
	tests := []struct {
		name    string
		args    args
		want    *ds.RecordPackage
		wantErr bool
	}{
		{
			name: "Pkg with camecase name",
			args: args{
				pkgName: "firstTestClassName",
			},
			wantErr: true,
		},
		{
			name: "too long pkg name",
			args: args{
				pkgName: "secondtestclassnametooverylong",
			},
			wantErr: true,
		},
		{
			name: "normal package name",
			args: args{
				pkgName: "foo",
			},
			wantErr: false,
		},
		{
			name: "dup",
			args: args{
				pkgName: "foo",
			},
			wantErr: true,
		},
		{
			name: "notdup",
			args: args{
				pkgName: "bar",
			},
			wantErr: false,
			want:    emptyRP,
		},
	}

	var got *ds.RecordPackage

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err = argen.addRecordPackage(tt.args.pkgName)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddRecordClass() error = %v, got = %+v, wantErr %v", err, argen, tt.wantErr)
				return
			}

			if tt.want != nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddRecordClass() = %+v, want %+v", got, tt.want)
			}
		})
	}

	got, err = argen.addRecordPackage("yarp")
	if err != nil {
		t.Errorf("AddRecordClass() error = %s", err)
		return
	}

	got.Backends = []string{"testbackend"}
	emptyRP.Backends = append(emptyRP.Backends, "testbackend")

	if !reflect.DeepEqual(argen.packagesParsed["yarp"], emptyRP) {
		t.Errorf("ModifyRecordClass() = %+v, want %+v", argen.packagesParsed["yarp"], emptyRP)
	}
}

func TestInternalInit(t *testing.T) {
	tempDirs := testutil.InitTmps()
	defer tempDirs.Defer()

	srcEmpty, dstEmpty, err := tempDirs.CreateDirs(testutil.NonExistsDstDir)
	if err != nil {
		t.Errorf("error initialize dirs: %s", err)
		return
	}

	tests := []struct {
		name    string
		args    args
		want    *ArGen
		wantErr bool
	}{
		{
			name: "empty src dir",
			args: args{
				appInfo: testutil.TestAppInfo,
				srcDir:  srcEmpty,
				dstDir:  dstEmpty,
				modName: "test.package.ar/activerecord/pkg.git",
			},
			want: &ArGen{
				src:            srcEmpty,
				dst:            dstEmpty,
				srcEntry:       []fs.DirEntry{},
				dstEntry:       []fs.DirEntry{},
				packagesParsed: map[string]*ds.RecordPackage{},
				packagesLock:   sync.Mutex{},
				appInfo:        &testutil.TestAppInfo,
				packagesLinked: map[string]string{},
				modName:        "test.package.ar/activerecord/pkg.git",
				fileToRemove:   map[string]bool{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.want != nil {
				tt.want.ctx = ctx
			}

			got, err := Init(ctx, &tt.args.appInfo, tt.args.srcDir, tt.args.dstDir, tt.args.dstFixture, tt.args.modName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, res = %+v, wantErr %v", err, got, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Init() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestArGen_prepareDir(t *testing.T) {
	tempDirs := testutil.InitTmps()
	defer tempDirs.Defer()

	srcNotEmpty, dstNotExist, err := tempDirs.CreateDirs(testutil.NonExistsDstDir)
	if err != nil {
		t.Errorf("can't initialize dirs: %s", err)
	}

	if _, err = os.CreateTemp(srcNotEmpty, "foo.go"); err != nil {
		t.Errorf("error create tmp dir `%s` with repository declaration: %s", srcNotEmpty, err)
		return
	}

	srcEntry, err := os.ReadDir(srcNotEmpty)
	if err != nil {
		t.Errorf("error open tmp dir `%s` with repository declaration: %s", srcNotEmpty, err)
		return
	}

	type want struct {
		srcEntry   []fs.DirEntry
		dstEntry   []fs.DirEntry
		dst        string
		dstCreated bool
	}

	tests := []struct {
		name    string
		args    args
		want    want
		wantErr bool
	}{
		{
			name: "prepare test",
			args: args{
				appInfo: testutil.TestAppInfo,
				srcDir:  srcNotEmpty,
				dstDir:  dstNotExist,
				modName: "github.com/mailru/activerecord",
			},
			want: want{
				srcEntry:   srcEntry,
				dstEntry:   []fs.DirEntry{},
				dst:        dstNotExist,
				dstCreated: true,
			},
			wantErr: false,
		},
		{
			name: "prepare fail test",
			args: args{
				appInfo: testutil.TestAppInfo,
				srcDir:  "/non/exists/src/path",
				dstDir:  "/non/exists/dst/path",
				modName: "github.com/mailru/activerecord",
			},
			want:    want{},
			wantErr: true,
		},
		{
			name: "error dst dir",
			args: args{
				appInfo: testutil.TestAppInfo,
				srcDir:  "/usr",
				dstDir:  "/var",
				modName: "github.com/mailru/activerecord",
			},
			want:    want{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := Init(ctx, &tt.args.appInfo, tt.args.srcDir, tt.args.dstDir, tt.args.dstFixture, tt.args.modName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, res = %+v, wantErr %v", err, got, tt.wantErr)
				return
			}

			if err == nil {
				if !reflect.DeepEqual(got.srcEntry, tt.want.srcEntry) {
					t.Errorf("srcEntry got = %+v, want %+v", got.srcEntry, tt.want.srcEntry)
					return
				}

				if !reflect.DeepEqual(got.dstEntry, tt.want.dstEntry) {
					t.Errorf("srcEntry got = %+v, want %+v", got.dstEntry, tt.want.dstEntry)
					return
				}
			}

			if _, err := os.ReadDir(tt.want.dst); (err != nil && os.IsNotExist(err)) == tt.want.dstCreated {
				t.Errorf("readDstErr got = %+v, want create %+v", err, tt.want.dstCreated)
				return
			}
		})
	}
}

func TestArGen_getExists(t *testing.T) {
	tempDirs := testutil.InitTmps()
	defer tempDirs.Defer()

	srcEmpty, dstNotEmpty, err := tempDirs.CreateDirs(testutil.EmptyDstDir)
	if err != nil {
		t.Errorf("can't initialize dirs: %s", err)
		return
	}

	entDir := filepath.Join(dstNotEmpty, "foo")
	if err := os.Mkdir(entDir, 0755); err != nil {
		t.Errorf("create dir into dst error: %s", err)
		return
	}

	entFile := filepath.Join(entDir, "octopus.go")
	if _, err := os.Create(entFile); err != nil {
		t.Errorf("create file into dst error: %s", err)
		return
	}

	type want struct {
		dstEntry   []fs.DirEntry
		existsFile []string
	}

	tests := []struct {
		name    string
		args    args
		want    want
		wantErr bool
	}{
		{
			name: "prepare fail test",
			args: args{
				appInfo: testutil.TestAppInfo,
				srcDir:  srcEmpty,
				dstDir:  dstNotEmpty,
				modName: "github.com/mailru/activerecord",
			},
			want: want{
				dstEntry:   []fs.DirEntry{},
				existsFile: []string{dstNotEmpty + "/foo", dstNotEmpty + "/foo/octopus.go"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := Init(ctx, &tt.args.appInfo, tt.args.srcDir, tt.args.dstDir, tt.args.dstFixture, tt.args.modName)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, res = %+v, wantErr %v", err, got, tt.wantErr)
				return
			}

			exists, err := got.getExists()

			if err == nil {
				if !reflect.DeepEqual(exists, tt.want.existsFile) {
					t.Errorf("existsFile got = %+v, want %+v", exists, tt.want.existsFile)
					return
				}
			}
		})
	}
}

func TestArGen_preparePackage(t *testing.T) {
	type want struct {
		modName       map[string]string
		linkedObjects map[string]string
	}

	rpFoo := ds.NewRecordPackage()
	rpFoo.Backends = []string{"octopus"}
	rpFoo.Server = ds.ServerDeclaration{Host: "127.0.0.1", Port: "11011"}
	rpFoo.Namespace = ds.NamespaceDeclaration{Num: 0, PackageName: "foo", PublicName: "Foo"}

	err := rpFoo.AddField(ds.FieldDeclaration{
		Name:       "ID",
		Format:     octopus.Int,
		PrimaryKey: true,
		Mutators:   []ds.FieldMutator{},
		Size:       0,
		Serializer: []string{},
		ObjectLink: "",
	})
	if err != nil {
		t.Errorf("can't prepare test data %s", err)
		return
	}

	err = rpFoo.AddField(ds.FieldDeclaration{
		Name:       "BarID",
		Format:     octopus.Int,
		PrimaryKey: false,
		Mutators:   []ds.FieldMutator{},
		Size:       0,
		Serializer: []string{},
		ObjectLink: "Bar",
	})
	if err != nil {
		t.Errorf("can't prepare test data %s", err)
		return
	}

	err = rpFoo.AddFieldObject(ds.FieldObject{
		Name:       "Foo",
		Key:        "ID",
		ObjectName: "bar",
		Field:      "BarID",
		Unique:     true,
	})
	if err != nil {
		t.Errorf("can't prepare test data %s", err)
		return
	}

	rpBar := ds.NewRecordPackage()
	rpBar.Backends = []string{"octopus"}
	rpBar.Namespace = ds.NamespaceDeclaration{Num: 1, PackageName: "bar", PublicName: "Bar"}

	err = rpBar.AddField(ds.FieldDeclaration{
		Name:       "ID",
		Format:     octopus.Int,
		PrimaryKey: false,
		Mutators:   []ds.FieldMutator{},
		Size:       0,
		Serializer: []string{},
		ObjectLink: "",
	})
	if err != nil {
		t.Errorf("can't prepare test data %s", err)
		return
	}

	tests := []struct {
		name    string
		fields  *ArGen
		want    want
		wantErr bool
	}{
		{
			name: "empty package",
			fields: &ArGen{
				ctx:            context.Background(),
				src:            "decl",
				dst:            "gener",
				srcEntry:       []fs.DirEntry{},
				dstEntry:       []fs.DirEntry{},
				packagesParsed: map[string]*ds.RecordPackage{},
				packagesLock:   sync.Mutex{},
				packagesLinked: map[string]string{},
				appInfo:        &ds.AppInfo{},
				modName:        "test.package.ar/activerecord/pkg.git",
			},
			want: want{
				linkedObjects: map[string]string{},
				modName:       map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "linked package",
			fields: &ArGen{
				ctx:            context.Background(),
				src:            "decl",
				dst:            "gener",
				srcEntry:       []fs.DirEntry{},
				dstEntry:       []fs.DirEntry{},
				packagesParsed: map[string]*ds.RecordPackage{"foo": rpFoo, "bar": rpBar},
				packagesLock:   sync.Mutex{},
				packagesLinked: map[string]string{},
				appInfo:        &ds.AppInfo{},
				modName:        "test.package.ar/activerecord/pkg.git",
			},
			want: want{
				linkedObjects: map[string]string{"bar": "bar", "foo": "foo"},
				modName:       map[string]string{"foo": "test.package.ar/activerecord/pkg.git/gener/foo", "bar": "test.package.ar/activerecord/pkg.git/gener/bar"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.fields
			if err := a.prepareCheck(); (err != nil) != tt.wantErr {
				t.Errorf("prepareCheck() error = %v, wantErr %v", err, tt.wantErr)
			}

			for pkg, rp := range a.packagesParsed {
				wnt, ok := tt.want.modName[pkg]
				if !ok {
					t.Errorf("invalid declare test, want.modName for package %s not exists", pkg)
					return
				}

				if rp.Namespace.ModuleName != wnt {
					t.Errorf("preparePackage modName for file %s = %s, want %s", pkg, rp.Namespace.ModuleName, wnt)
				}
			}

			if !reflect.DeepEqual(a.packagesLinked, tt.want.linkedObjects) {
				t.Errorf("preparePackage linkedObjects = %+v, want %+v", a.packagesLinked, tt.want.linkedObjects)
				return
			}
		})
	}
}

func TestArGen_prepareCheck(t *testing.T) {
	type fields struct {
		packagesParsed map[string]*ds.RecordPackage
		modName        string
	}
	tests := []struct {
		name       string
		fields     fields
		want       map[string]*ds.RecordPackage
		wantLinked map[string]string
		wantErr    bool
	}{
		{
			name: "simple package",
			fields: fields{
				modName: "testmodname",
				packagesParsed: map[string]*ds.RecordPackage{
					"test": {
						Namespace: ds.NamespaceDeclaration{
							PackageName: "testPackage",
						},
					},
				},
			},
			wantErr: false,
			want: map[string]*ds.RecordPackage{
				"test": {
					Namespace: ds.NamespaceDeclaration{
						PackageName: "testPackage",
						ModuleName:  "testmodname/testPackage",
					},
				},
			},
			wantLinked: map[string]string{
				"testPackage": "test",
			},
		},
	}
	a := &ArGen{
		ctx:            context.Background(),
		src:            "",
		dst:            "",
		srcEntry:       []fs.DirEntry{},
		dstEntry:       []fs.DirEntry{},
		packagesLock:   sync.Mutex{},
		packagesLinked: map[string]string{},
		appInfo:        &ds.AppInfo{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a.packagesParsed = tt.fields.packagesParsed
			a.modName = tt.fields.modName
			if err := a.prepareCheck(); (err != nil) != tt.wantErr {
				t.Errorf("prepareCheck() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(a.packagesParsed, tt.want) {
				if len(a.packagesParsed) != len(tt.want) {
					t.Errorf("prepareCheck parsed got len = %d, want %d", len(a.packagesParsed), len(tt.want))
				}

				for key := range a.packagesParsed {
					if !reflect.DeepEqual(a.packagesParsed[key], tt.want[key]) {
						t.Errorf("prepareCheck parsed package %s = %+v, want %+v", key, a.packagesParsed[key], tt.want[key])
					}
				}
			}

			if !reflect.DeepEqual(a.packagesLinked, tt.wantLinked) {
				t.Errorf("prepareCheck packageslinked got %+v, want %+v", a.packagesLinked, tt.wantLinked)
			}
		})
	}
}

func Test_writeToFile(t *testing.T) {
	tempDirs := testutil.InitTmps()
	defer tempDirs.Defer()

	dst, err := tempDirs.AddTempDir()
	if err != nil {
		t.Errorf("can't initialize dirs: %s", err)
	}

	type args struct {
		dirPkg      string
		dstFileName string
		data        []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success write",
			args: args{
				dirPkg:      dst,
				dstFileName: filepath.Join(dst, "test.bla"),
				data:        []byte("testbyte"),
			},
			wantErr: false,
		},
		{
			name: "invalid file name",
			args: args{
				dirPkg:      dst,
				dstFileName: "test.bla",
				data:        []byte("testbyte"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeToFile(tt.args.dirPkg, tt.args.dstFileName, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("writeToFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestArGen_parse(t *testing.T) {
	tempDirs := testutil.InitTmps()
	defer tempDirs.Defer()

	repositoryName := "foo"
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

	if err = os.WriteFile(filepath.Join(src, repositoryName+".go"), []byte(textTestPkg), 0644); err != nil {
		t.Errorf("prepare test files error: %s", err)
		return
	}

	tests := []struct {
		name    string
		fields  args
		wantErr bool
		want    map[string]*ds.RecordPackage
	}{
		{
			name:    "simple package",
			fields:  args{srcDir: src, dstDir: filepath.Join(srcRoot, "model/repository/cmpl")},
			wantErr: false,
			want: map[string]*ds.RecordPackage{
				"foo": {
					Namespace: ds.NamespaceDeclaration{Num: 2, PublicName: "Foo", PackageName: "foo"},
					Server:    ds.ServerDeclaration{Timeout: 500, Host: "127.0.0.1", Port: "11111"},
					Fields: []ds.FieldDeclaration{
						{Name: "Field1", Format: "int", PrimaryKey: true, Mutators: []ds.FieldMutator{}, Size: 5, Serializer: []string{}},
						{Name: "Field2", Format: "string", PrimaryKey: true, Mutators: []ds.FieldMutator{}, Size: 5, Serializer: []string{}},
					},
					FieldsMap:       map[string]int{"Field1": 0, "Field2": 1},
					FieldsObjectMap: map[string]ds.FieldObject{},
					ProcFields:      []ds.ProcFieldDeclaration{},
					ProcFieldsMap:   map[string]int{},
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
						{Name: "Field1Part",
							Num:      0,
							Selector: "SelectByField1",
							Fields:   []int{0},
							FieldsMap: map[string]ds.IndexField{
								"Field1": {IndField: 0, Order: 0},
							},
							Primary: false,
							Unique:  false,
							Partial: true,
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
					FlagMap: map[string]ds.FlagDeclaration{}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := Init(context.Background(), &ds.AppInfo{}, tt.fields.srcDir, tt.fields.dstDir, tt.fields.dstFixture, "testmod")
			if err != nil {
				t.Errorf("can't init argen: %s", err)
				return
			}

			if err := a.parse(); (err != nil) != tt.wantErr {
				t.Errorf("ArGen.parse() error = %v, wantErr %v", err, tt.wantErr)
			}

			if len(a.packagesParsed) != len(tt.want) {
				t.Errorf("Mismatch len keys = %+v, want %+v", a.packagesParsed, tt.want)
				return
			}

			for key, pkg := range a.packagesParsed {
				assert.Check(t, cmp.DeepEqual(tt.want[key], pkg), "Invalid response, test `%s`", key)
			}
		})
	}
}
