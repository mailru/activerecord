package generator

import (
	"strings"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

func TestGenerateOctopus(t *testing.T) {
	type args struct {
		params PkgData
	}

	namespaceStr := "2"

	packageName := "foo"

	tests := []struct {
		name    string
		args    args
		want    *arerror.ErrGeneratorPhases
		wantStr map[string][]string
	}{
		{
			name: "fieldsPkg",
			want: nil,
			args: args{
				params: PkgData{
					ARPkg:      packageName,
					ARPkgTitle: "Foo",
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
						},
						{
							Name:     "Field2",
							Num:      1,
							Selector: "SelectByField2",
							Fields:   []int{1},
							FieldsMap: map[string]ds.IndexField{
								"Field1": {IndField: 1, Order: 0},
							},
							Primary: false,
							Unique:  false,
						},
					},
					FieldList: []ds.FieldDeclaration{
						{
							Name:       "Field1",
							Format:     "int",
							PrimaryKey: true,
							Mutators:   []string{ds.IncMutator},
							Serializer: []string{},
							ObjectLink: "",
						},
						{
							Name:       "Field2",
							Format:     "bool",
							PrimaryKey: true,
							Mutators:   []string{},
							Serializer: []string{},
							ObjectLink: "",
						},
						{
							Name:       "Fs",
							Format:     "string",
							PrimaryKey: true,
							Mutators:   []string{"FsMutator"},
							Serializer: []string{},
							ObjectLink: "",
						},
					},
					FieldObject: map[string]ds.FieldObject{},
					Server:      ds.ServerDeclaration{Timeout: 500, Host: "127.0.0.1", Port: "11011"},
					Container:   ds.NamespaceDeclaration{ObjectName: "2", PublicName: "Testmodel", PackageName: "testmodel"},
					Serializers: map[string]ds.SerializerDeclaration{},
					Mutators: map[string]ds.MutatorDeclaration{
						"FsMutator": {
							Name:       "FsMutator",
							Pkg:        "github.com/mailru/activerecord/internal/pkg/conv",
							Type:       "*parser_test.Foo",
							ImportName: "mutatorFooMutatorField",
							Update:     "updateFunc",
							Replace:    "replaceFunc",
							PartialFields: []ds.PartialFieldDeclaration{
								{Parent: "Foo", Name: "Bar", Type: "ds.AppInfo"},
								{Parent: "Foo", Name: "BeerData", Type: "[]Beer"},
								{Parent: "Foo", Name: "MapData", Type: "map[string]any"},
							},
						},
					},
					Imports:  []ds.ImportDeclaration{},
					Triggers: map[string]ds.TriggerDeclaration{},
					Flags:    map[string]ds.FlagDeclaration{},
				},
			},
			wantStr: map[string][]string{
				"octopus": {
					`Code generated by argen. DO NOT EDIT.`,
					`func (obj *Foo) insertReplace(ctx context.Context, insertMode octopus.InsertMode) error {`,
					`func (obj *Foo) InsertOrReplace(ctx context.Context) error {`,
					`func (obj *Foo) Replace(ctx context.Context) error {`,
					`func (obj *Foo) Insert(ctx context.Context) error {`,
					`func (obj *Foo) Update(ctx context.Context) error {`,
					`func (obj *Foo) Delete(ctx context.Context) error {`,
					`func (obj *Foo) packPk() ([][]byte, error) {`,
					`func (obj *Foo) Equal (anotherObjI any) bool {`,
					`func (obj *Foo) PrimaryString() string {`,
					`func selectBox (ctx context.Context, indexnum uint32, keysPacked [][][]byte, limiter activerecord.SelectorLimiter) ([]*Foo, error) {`,
					`func (obj *Foo) SetField1(Field1 int) error {`,
					`func (obj *Foo) GetField1() int {`,
					`type Mutators struct {`,
					`newObj.FsMutator.OpFunc`,
					`newObj.FsMutator.PartialFields`,
					`func (obj *Foo) SetFsMutatorBar(Bar ds.AppInfo) error {`,
					`func (obj *Foo) SetFsMutatorBeerData(BeerData []Beer) error {`,
					`func (obj *Foo) SetFsMutatorMapData(MapData map[string]any) error {`,
					`func (obj *Foo) packFsPartialFields(op octopus.OpCode) error {`,
					`func UnpackField1(r *bytes.Reader) (ret int, errRet error) {`,
					`func packField1(w []byte, Field1 int) ([]byte, error) {`,
					`func NewFromBox(ctx context.Context, tuples []octopus.TupleData) ([]*Foo, error) {`,
					`func TupleToStruct(ctx context.Context, tuple octopus.TupleData) (*Foo, error) {`,
					`func New(ctx context.Context) *Foo {`,
					`func (obj *Foo) IncField1(mutArg int) error {`,
					`namespace uint32 = ` + namespaceStr,
					`type Foo struct {`,
					`package ` + packageName,
				},
				"mock": {
					`func (obj *Foo) mockInsertReplace(ctx context.Context, insertMode octopus.InsertMode) []byte {`,
					`func (obj *Foo) MockReplace(ctx context.Context) []byte {`,
					`func (obj *Foo) MockInsert(ctx context.Context) []byte {`,
					`func (obj *Foo) MockInsertOrReplace(ctx context.Context) []byte {`,
					`func (obj *Foo) MockUpdate(ctx context.Context) []byte {`,
					`func (obj *Foo) RepoSelector(ctx context.Context) (any, error) {`,
					`func SelectByField1MockerLogger(keys [], res FooList) func() (activerecord.MockerLogger, error) {`,
					`func (obj *Foo) MockSelectByField1sRequest(ctx context.Context, keys [], ) []byte {`,
					`func (obj *Foo) MockSelectResponse() ([][]byte, error) {`,
					`func (obj *Foo) MockMutatorUpdate(ctx context.Context) [][]byte {`,
				},
				"fixture": {
					`type FooFT struct {`,
					`func MarshalFixtures(objs []*Foo) ([]byte, error) {`,
					`func UnmarshalFixtures(source []byte) []*Foo {`,
					`func (objs FooList) String() string {`,
				},
			},
		},
		{
			name: "simpleProcPkg",
			want: nil,
			args: args{
				params: PkgData{
					ARPkg:           packageName,
					ARPkgTitle:      "Foo",
					FieldList:       []ds.FieldDeclaration{},
					FieldMap:        map[string]int{},
					ProcInFieldList: []ds.ProcFieldDeclaration{},
					ProcOutFieldList: []ds.ProcFieldDeclaration{
						{
							Name:       "Output",
							Format:     "string",
							Type:       ds.OUT,
							Serializer: []string{},
						},
					},
					Server:      ds.ServerDeclaration{Timeout: 500, Host: "127.0.0.1", Port: "11011"},
					Container:   ds.NamespaceDeclaration{ObjectName: "simpleProc", PublicName: "Testmodel", PackageName: "testmodel"},
					Indexes:     []ds.IndexDeclaration{},
					Serializers: map[string]ds.SerializerDeclaration{},
					Imports:     []ds.ImportDeclaration{},
					Triggers:    map[string]ds.TriggerDeclaration{},
					Flags:       map[string]ds.FlagDeclaration{},
					AppInfo:     "",
				},
			},
			wantStr: map[string][]string{
				"octopus": {
					`Code generated by argen. DO NOT EDIT.`,
					`func (obj *Foo) GetOutput() string {`,
					`func Call(ctx context.Context) (*Foo, error)`,
					`func TupleToStruct(ctx context.Context, tuple octopus.TupleData) (*Foo, error) {`,
					`procName string = "simpleProc"`,
					`type Foo struct {`,
					`type FooParams struct {`,
					`package ` + packageName,
				},
				"mock": {
					`func (obj *Foo) RepoSelector(ctx context.Context) (any, error) {`,
					`func CallMockerLogger(res FooList) func() (activerecord.MockerLogger, error) {`,
					`func MockCallRequest(ctx context.Context) []byte {`,
					`func (obj *Foo) MockSelectResponse() ([][]byte, error) {`,
				},
				"fixture": {
					`type FooFTPK struct {`,
					`type FooFT struct {`,
					`func MarshalFixtures(objs []*Foo) ([]byte, error) {`,
					`func UnmarshalFixtures(source []byte) []*Foo {`,
					`func (objs FooList) String() string {`,
				},
			},
		},
		{
			name: "procPkg",
			want: nil,
			args: args{
				params: PkgData{
					ARPkg:      packageName,
					ARPkgTitle: "Foo",
					FieldList:  []ds.FieldDeclaration{},
					FieldMap:   map[string]int{},
					ProcInFieldList: []ds.ProcFieldDeclaration{
						{
							Name:       "Input",
							Format:     "[]string",
							Type:       ds.IN,
							Serializer: []string{},
						},
						{
							Name:       "InputOutput",
							Format:     "string",
							Type:       ds.OUT,
							Serializer: []string{},
						},
					},
					ProcOutFieldList: []ds.ProcFieldDeclaration{
						{
							Name:       "InputOutput",
							Format:     "string",
							Type:       ds.OUT,
							Serializer: []string{},
						},
						{
							Name:       "Output",
							Format:     "string",
							Type:       ds.INOUT,
							Serializer: []string{"s2i"},
						},
					},
					Server:    ds.ServerDeclaration{Timeout: 500, Host: "127.0.0.1", Port: "11011"},
					Container: ds.NamespaceDeclaration{ObjectName: "bar", PublicName: "Testmodel", PackageName: "testmodel"},
					Indexes:   []ds.IndexDeclaration{},
					Serializers: map[string]ds.SerializerDeclaration{
						"s2i": {
							Name:        "Output",
							Pkg:         "github.com/mailru/activerecord/pkg/serializer",
							Type:        "int",
							ImportName:  "serializerOutput",
							Marshaler:   "OutputMarshal",
							Unmarshaler: "OutputUnmarshal",
						},
					},
					Imports:  []ds.ImportDeclaration{},
					Triggers: map[string]ds.TriggerDeclaration{},
					Flags:    map[string]ds.FlagDeclaration{},
					AppInfo:  "",
				},
			},
			wantStr: map[string][]string{
				"octopus": {
					`Code generated by argen. DO NOT EDIT.`,
					`func (obj *Foo) GetOutput() int {`,
					`func (obj *Foo) GetInputOutput() string {`,
					`func Call(ctx context.Context, params FooParams) (*Foo, error)`,
					`func TupleToStruct(ctx context.Context, tuple octopus.TupleData) (*Foo, error) {`,
					`procName string = "bar"`,
					`type Foo struct {`,
					`type FooParams struct {`,
					`func (obj *FooParams) arrayValues() ([]string, error)`,
					`package ` + packageName,
				},
				"mock": {
					`func (obj *Foo) RepoSelector(ctx context.Context) (any, error) {`,
					`func CallMockerLogger(params FooParams, res FooList) func() (activerecord.MockerLogger, error) {`,
					`func MockCallRequest(ctx context.Context, params FooParams) []byte {`,
					`func (obj *Foo) MockSelectResponse() ([][]byte, error) {`,
				},
				"fixture": {
					`type FooFTPK struct {`,
					`type FooFT struct {`,
					`func MarshalFixtures(objs []*Foo) ([]byte, error) {`,
					`func UnmarshalFixtures(source []byte) []*Foo {`,
					`func (objs FooList) String() string {`,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, got := GenerateOctopus(tt.args.params)
			if got != tt.want {
				t.Errorf("GenerateOctopus() = %v, want %v", got, tt.want)
			}

			for name, strs := range tt.wantStr {
				buff, ex := ret[name]
				if !ex {
					t.Errorf("GenerateOctopus() Name %s not generated", name)
					return
				}

				for _, substr := range strs {
					if !strings.Contains(buff.String(), substr) {
						t.Errorf("GenerateOctopus() %s = %v, want %v", name, buff.String(), substr)
					}
				}
			}
		})
	}
}
