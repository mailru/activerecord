package generator

import (
	"bufio"
	"bytes"
	_ "embed"
	"log"
	"strings"
	"text/template"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/pkg/iproto/util/text"
	"github.com/mailru/activerecord/pkg/octopus"
)

//nolint:revive
//go:embed tmpl/tarantool/main.tmpl
var tarantoolRootRepositoryTmpl string

//nolint:revive
//go:embed tmpl/tarantool/procedure.tmpl
var tarantoolProcRepositoryTmpl string

//nolint:revive
//go:embed tmpl/tarantool/fixture.tmpl
var tarantoolFixtureRepositoryTmpl string

func GenerateTarantool(params PkgData) (map[string]bytes.Buffer, *arerror.ErrGeneratorPhases) {
	mainWriter := bytes.Buffer{}
	fixtureWriter := bytes.Buffer{}

	var repositoryTmpl string
	if len(params.FieldList) > 0 {
		repositoryTmpl = tarantoolRootRepositoryTmpl
	} else if len(params.ProcOutFieldList) > 0 {
		repositoryTmpl = tarantoolProcRepositoryTmpl
	} else {
		return nil, &arerror.ErrGeneratorPhases{Backend: "tarantool", Err: arerror.ErrGeneratorTemplateUnkhown}
	}

	octopusFile := bufio.NewWriter(&mainWriter)

	err := GenerateByTmpl(octopusFile, params, "tarantool", repositoryTmpl, Tarantool2TmplFunc)
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
	"packerParam": func(format octopus.Format) TarantoolFormatParam {
		ret, ex := PrimitiveTypeFormatConverter[format]
		if !ex {
			log.Fatalf("packer for type `%s` not found", format)
		}

		return ret
	},
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
	"trimPrefix":       strings.TrimPrefix,
	"hasPrefix":        strings.HasPrefix,
	"lowerCase":        strings.ToLower,
	"snakeToCamelCase": text.SnakeToCamelCase,
}

type TarantoolFormatParam string

func (p TarantoolFormatParam) ToString() []string {
	return strings.SplitN(string(p), `%%`, 2)
}

var PrimitiveTypeFormatConverter = map[octopus.Format]TarantoolFormatParam{
	octopus.Bool:    "strconv.FormatBool(%%)",
	octopus.Uint8:   "strconv.FormatUint(uint64(%%), 10)",
	octopus.Uint16:  "strconv.FormatUint(uint64(%%), 10)",
	octopus.Uint32:  "strconv.FormatUint(uint64(%%), 10)",
	octopus.Uint64:  "strconv.FormatUint(%%, 10)",
	octopus.Uint:    "strconv.FormatUint(uint64(%%), 10)",
	octopus.Int8:    "strconv.FormatInt(int64(%%), 10)",
	octopus.Int16:   "strconv.FormatInt(int64(%%), 10)",
	octopus.Int32:   "strconv.FormatInt(int64(%%), 10)",
	octopus.Int64:   "strconv.FormatInt(%%, 10)",
	octopus.Int:     "strconv.FormatInt(int64(%%), 10)",
	octopus.Float32: "strconv.FormatFloat(%%, 32)",
	octopus.Float64: "strconv.FormatFloat(%%, 64)",
	octopus.String:  "%%",
}
