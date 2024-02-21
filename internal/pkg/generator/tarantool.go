package generator

import (
	"bufio"
	"bytes"
	_ "embed"
	"strings"
	"text/template"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

//nolint:revive
//go:embed tmpl/tarantool/main.tmpl
var tarantoolRootRepositoryTmpl string

//nolint:revive
//go:embed tmpl/tarantool/fixture.tmpl
var tarantoolFixtureRepositoryTmpl string

func GenerateTarantool(params PkgData) (map[string]bytes.Buffer, *arerror.ErrGeneratorPhases) {
	mainWriter := bytes.Buffer{}
	fixtureWriter := bytes.Buffer{}

	octopusFile := bufio.NewWriter(&mainWriter)

	err := GenerateByTmpl(octopusFile, params, "tarantool", tarantoolRootRepositoryTmpl, Tarantool2TmplFunc)
	if err != nil {
		return nil, err
	}

	octopusFile.Flush()

	fixtureFile := bufio.NewWriter(&fixtureWriter)

	err = GenerateByTmpl(fixtureFile, params, "tarantool", tarantoolFixtureRepositoryTmpl, Tarantool2TmplFunc)
	if err != nil {
		return nil, err
	}

	fixtureFile.Flush()

	ret := map[string]bytes.Buffer{
		"tarantool": mainWriter,
		"fixture":   fixtureWriter,
	}

	return ret, nil
}

//go:embed tmpl/tarantool/fixturestore.tmpl
var tarantoolFixtureStoreTmpl string

func GenerateTarantoolFixtureStore(params FixturePkgData) (map[string]bytes.Buffer, *arerror.ErrGeneratorPhases) {
	fixtureWriter := bytes.Buffer{}

	file := bufio.NewWriter(&fixtureWriter)

	templatePackage, err := template.New(TemplateName).Funcs(funcs).Parse(disclaimer + tarantoolFixtureStoreTmpl)
	if err != nil {
		tmplLines, errgetline := getTmplErrorLine(strings.SplitAfter(disclaimer+tarantoolFixtureStoreTmpl, "\n"), err.Error())
		if errgetline != nil {
			tmplLines = errgetline.Error()
		}

		return nil, &arerror.ErrGeneratorPhases{Backend: "fixture", Phase: "parse", TmplLines: tmplLines, Err: err}
	}

	err = templatePackage.Execute(file, params)
	if err != nil {
		tmplLines, errgetline := getTmplErrorLine(strings.SplitAfter(disclaimer+tarantoolFixtureStoreTmpl, "\n"), err.Error())
		if errgetline != nil {
			tmplLines = errgetline.Error()
		}

		return nil, &arerror.ErrGeneratorPhases{Backend: "fixture", Phase: "execute", TmplLines: tmplLines, Err: err}
	}

	file.Flush()

	ret := map[string]bytes.Buffer{
		"fixture": fixtureWriter,
	}

	return ret, nil
}

var Tarantool2TmplFunc = template.FuncMap{
	"addImport": func(flds []ds.FieldDeclaration) (imports []string) {
		var needStrconv bool

		for _, fld := range flds {
			if fld.PrimaryKey && fld.Format != "string" {
				needStrconv = true
			}
		}

		if needStrconv {
			imports = append(imports, "strconv")
		}

		return
	},
}
