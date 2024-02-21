package generator

import (
	"strings"
	"testing"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

func TestGenerateFixture(t *testing.T) {
	type args struct {
		params FixturePkgData
	}

	packageName := "gift"

	tests := []struct {
		name    string
		args    args
		want    *arerror.ErrGeneratorPhases
		wantStr map[string][]string
	}{
		{
			name: "simplePkg",
			want: nil,
			args: args{
				params: FixturePkgData{
					FixturePkg: "simplefixture",
					ARPkg:      packageName,
					ARPkgTitle: "Gift",
					Indexes: []ds.IndexDeclaration{
						{
							Name:     "Id",
							Num:      0,
							Selector: "SelectById",
							Fields:   []int{0},
							FieldsMap: map[string]ds.IndexField{
								"Id": {IndField: 0, Order: 0},
							},
							Primary: true,
							Unique:  true,
							Type:    "string",
						},
						{
							Name:     "Inv",
							Num:      1,
							Selector: "SelectByInv",
							Fields:   []int{2},
							FieldsMap: map[string]ds.IndexField{
								"Inv": {IndField: 2, Order: 0},
							},
							Primary: false,
							Unique:  false,
							Type:    "bool",
						},
					},
					FieldList: []ds.FieldDeclaration{
						{
							Name:       "Id",
							Format:     "string",
							PrimaryKey: true,
							Mutators:   []string{},
							Serializer: []string{},
							ObjectLink: "",
						},
						{
							Name:       "Code",
							Format:     "string",
							Mutators:   []string{"Any"},
							Serializer: []string{},
							ObjectLink: "",
						},
						{
							Name:       "Inv",
							Format:     "bool",
							Mutators:   []string{},
							Serializer: []string{},
							ObjectLink: "",
						},
					},
					FieldObject: map[string]ds.FieldObject{},
					Container:   ds.NamespaceDeclaration{ObjectName: "0", PublicName: "Testmodel", PackageName: "testmodel"},
					Serializers: map[string]ds.SerializerDeclaration{},
					Mutators: map[string]ds.MutatorDeclaration{
						"Any": {
							Name:   "Any",
							Update: "any",
						},
					},
					Imports: []ds.ImportDeclaration{
						{
							ImportName: "obj",
							Path:       "github.com/foo/bar/baz.git/internal/pkg/model/repository/cmpl/gift",
						},
					},
				},
			},
			wantStr: map[string][]string{
				"fixture": {
					`Code generated by argen. DO NOT EDIT.`,
					`package simplefixture`,
					`type GiftBySelectByIdMocker struct {`,
					`func GetGiftById(Id string) *gift.Gift`,
					`type GiftBySelectByInvMocker struct {`,
					`var giftStore map[string]int`,
					`var giftFixtures []*gift.Gift`,
					`func initGift() {`,
					`func GetUpdateMutatorAnyFixtureById(ctx context.Context, Id string) (fxt octopus.FixtureType) {`,
				},
			},
		},
		{
			name: "simpleProcPkg",
			want: nil,
			args: args{
				params: FixturePkgData{
					FixturePkg:      "procfixture",
					ARPkg:           packageName,
					ARPkgTitle:      "Gift",
					FieldList:       []ds.FieldDeclaration{},
					FieldMap:        map[string]int{},
					FieldObject:     nil,
					ProcInFieldList: []ds.ProcFieldDeclaration{},
					ProcOutFieldList: []ds.ProcFieldDeclaration{
						{
							Name:       "Output",
							Format:     "string",
							Type:       ds.OUT,
							Serializer: []string{},
						},
					},
					Container:   ds.NamespaceDeclaration{ObjectName: "simpleProc", PublicName: "Testmodel", PackageName: "testmodel"},
					Indexes:     []ds.IndexDeclaration{},
					Serializers: map[string]ds.SerializerDeclaration{},
					Imports:     []ds.ImportDeclaration{},
				},
			},
			wantStr: map[string][]string{
				"fixture": {
					`package procfixture`,
					`var giftStore map[string]int`,
					`var giftFixtures []*gift.Gift`,
					`func initGift() {`,
					`func GetGiftByParams(params gift.GiftParams) *gift.Gift {`,
					`type GiftProcedureMocker struct {}`,
					`func GetGiftProcedureMocker() GiftProcedureMocker {`,
					`func (m GiftProcedureMocker) ByFixture(ctx context.Context) octopus.FixtureType {`,
					`func (m GiftProcedureMocker) ByMocks(ctx context.Context, mocks []octopus.MockEntities) octopus.FixtureType {`,
				},
			},
		},
		{
			name: "procPkg",
			want: nil,
			args: args{
				params: FixturePkgData{
					FixturePkg: "procfixture",
					ARPkg:      packageName,
					ARPkgTitle: "Gift",
					FieldList:  []ds.FieldDeclaration{},
					FieldMap:   map[string]int{},
					ProcInFieldList: []ds.ProcFieldDeclaration{
						{
							Name:       "Input",
							Format:     "string",
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
					Imports: []ds.ImportDeclaration{},
					AppInfo: "",
				},
			},
			wantStr: map[string][]string{
				"fixture": {
					`package procfixture`,
					`var giftStore map[string]int`,
					`var giftFixtures []*gift.Gift`,
					`func initGift() {`,
					`func GetGiftByParams(params gift.GiftParams) *gift.Gift {`,
					`type GiftProcedureMocker struct {}`,
					`func GetGiftProcedureMocker() GiftProcedureMocker {`,
					`func (m GiftProcedureMocker) ByFixtureParams(ctx context.Context, params gift.GiftParams) octopus.FixtureType {`,
					`func (m GiftProcedureMocker) ByParamsMocks(ctx context.Context, params gift.GiftParams, mocks []octopus.MockEntities) octopus.FixtureType {`,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret, got := GenerateOctopusFixtureStore(tt.args.params)
			if got != tt.want {
				t.Errorf("GenerateFixture() = %v, want %v", got, tt.want)
			}

			for name, strs := range tt.wantStr {
				buff, ex := ret[name]
				if !ex {
					t.Errorf("GenerateFixture() Name %s not generated", name)
					return
				}

				for _, substr := range strs {
					if !strings.Contains(buff.String(), substr) {
						t.Errorf("GenerateFixture() = %v, want %v", buff.String(), substr)
					}
				}
			}
		})
	}
}
