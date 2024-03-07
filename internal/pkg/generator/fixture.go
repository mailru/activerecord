package generator

import (
	"bufio"
	"bytes"
	_ "embed"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
	"golang.org/x/tools/imports"
)

type FixturePkgData struct {
	FixturePkg       string
	ARPkg            string
	ARPkgTitle       string
	FieldList        []ds.FieldDeclaration
	FieldMap         map[string]int
	FieldObject      map[string]ds.FieldObject
	ProcInFieldList  []ds.ProcFieldDeclaration
	ProcOutFieldList []ds.ProcFieldDeclaration
	Container        ds.NamespaceDeclaration
	Indexes          []ds.IndexDeclaration
	Serializers      map[string]ds.SerializerDeclaration
	Mutators         map[string]ds.MutatorDeclaration
	Imports          []ds.ImportDeclaration
	AppInfo          string
}

type FixtureMetaData struct {
	MetaData
	FixturePkg string
}

//nolint:revive
//go:embed tmpl/fixture_meta.tmpl
var fixtureMetaTmpl string

func generateFixtureMeta(params FixtureMetaData) (*bytes.Buffer, *arerror.ErrGeneratorFile) {
	metaWriter := new(bytes.Buffer)
	metaFile := bufio.NewWriter(metaWriter)

	if err := GenerateByTmpl(metaFile, params, "fixture_meta", fixtureMetaTmpl); err != nil {
		return nil, &arerror.ErrGeneratorFile{Name: "repository.go", Backend: "fixture_meta", Filename: "repository.go", Err: err}
	}

	metaFile.Flush()

	return metaWriter, nil
}

func GenerateFixtureMeta(packageNamespaces map[string][]*ds.RecordPackage, appInfo, pkgFixture string) ([]GenerateFile, error) {
	var ret = make([]GenerateFile, 0, len(packageNamespaces))

	for backend, namespaces := range packageNamespaces {
		metaData := FixtureMetaData{
			MetaData: MetaData{
				Namespaces: namespaces,
				AppInfo:    appInfo,
			},
			FixturePkg: pkgFixture,
		}
		var generated *bytes.Buffer

		switch backend {
		case "tarantool15":
			fallthrough
		case "octopus":
			fallthrough
		case "tarantool16":
			fallthrough
		case "tarantool2":
			var err *arerror.ErrGeneratorFile

			generated, err = generateFixtureMeta(metaData)
			if err != nil {
				err.Name = "fixture_meta"
				return nil, err
			}
		case "postgres":
			return nil, &arerror.ErrGeneratorFile{Name: "fixture_meta", Backend: backend, Err: arerror.ErrGeneratorBackendNotImplemented}
		default:
			return nil, &arerror.ErrGeneratorFile{Name: "fixture_meta", Backend: backend, Err: arerror.ErrGeneratorBackendUnknown}
		}

		genRes := GenerateFile{
			Dir:     pkgFixture,
			Name:    "stores.go",
			Backend: "fixture_meta",
		}

		genData := generated.Bytes()

		var err error

		genRes.Data, err = imports.Process("", genData, nil)
		if err != nil {
			return nil, &arerror.ErrGeneratorFile{Name: "repository.go", Backend: "fixture_meta", Filename: genRes.Name, Err: ErrorLine(err, string(genData))}
		}

		ret = append(ret, genRes)
	}

	return ret, nil
}
