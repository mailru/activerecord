package generator

import (
	"bufio"
	"bytes"
	_ "embed"
	"io"
	"strings"
	"text/template"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/pkg/iproto/util/text"
)

//go:embed tmpl/octopus/fixturestore.tmpl
var tmpl string

func generateFixture(params PkgData) (map[string]bytes.Buffer, *arerror.ErrGeneratorPhases) {
	fixtureWriter := bytes.Buffer{}

	fixtureFile := bufio.NewWriter(&fixtureWriter)

	err := GenerateFixtureTmpl(fixtureFile, params)
	if err != nil {
		return nil, err
	}

	fixtureFile.Flush()

	ret := map[string]bytes.Buffer{
		"fixture": fixtureWriter,
	}

	return ret, nil
}

func GenerateFixtureTmpl(dstFile io.Writer, params PkgData) *arerror.ErrGeneratorPhases {
	templatePackage, err := template.New(TemplateName).Funcs(templateFuncs).Funcs(OctopusTemplateFuncs).Parse(disclaimer + tmpl)
	if err != nil {
		tmplLines, errgetline := getTmplErrorLine(strings.SplitAfter(disclaimer+tmpl, "\n"), err.Error())
		if errgetline != nil {
			tmplLines = errgetline.Error()
		}

		return &arerror.ErrGeneratorPhases{Backend: "fixture", Phase: "parse", TmplLines: tmplLines, Err: err}
	}

	err = templatePackage.Execute(dstFile, params)
	if err != nil {
		tmplLines, errgetline := getTmplErrorLine(strings.SplitAfter(disclaimer+tmpl, "\n"), err.Error())
		if errgetline != nil {
			tmplLines = errgetline.Error()
		}

		return &arerror.ErrGeneratorPhases{Backend: "fixture", Phase: "execute", TmplLines: tmplLines, Err: err}
	}

	return nil
}

var templateFuncs = template.FuncMap{"snakeCase": text.ToSnakeCase, "split": strings.Split}
