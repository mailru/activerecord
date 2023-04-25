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
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

//nolint:revive
//go:embed tmpl/octopus/mock.tmpl
var OctopusMockRepositoryTmpl string

//nolint:revive
//go:embed tmpl/octopus/main.tmpl
var OctopusRootRepositoryTmpl string

//nolint:revive
//go:embed tmpl/octopus/fixture.tmpl
var OctopusFixtureRepositoryTmpl string

var funcs = template.FuncMap{"snakeCase": text.ToSnakeCase}

func GenerateOctopus(params PkgData) (map[string]bytes.Buffer, *arerror.ErrGeneratorPhases) {
	octopusWriter := bytes.Buffer{}
	mockWriter := bytes.Buffer{}
	fixtureWriter := bytes.Buffer{}

	octopusFile := bufio.NewWriter(&octopusWriter)

	//TODO возможно имеет смысл разделить большой шаблон OctopusRootRepositoryTmpl для удобства поддержки
	err := GenerateByTmpl(octopusFile, params, "octopus", OctopusRootRepositoryTmpl)
	if err != nil {
		return nil, err
	}

	octopusFile.Flush()

	mockFile := bufio.NewWriter(&mockWriter)

	err = GenerateByTmpl(mockFile, params, "octopus", OctopusMockRepositoryTmpl)
	if err != nil {
		return nil, err
	}

	mockFile.Flush()

	fixtureFile := bufio.NewWriter(&fixtureWriter)

	err = GenerateByTmpl(fixtureFile, params, "octopus", OctopusFixtureRepositoryTmpl)
	if err != nil {
		return nil, err
	}

	fixtureFile.Flush()

	ret := map[string]bytes.Buffer{
		"octopus": octopusWriter,
		"mock":    mockWriter,
		"fixture": fixtureWriter,
	}

	return ret, nil
}

var OctopusTemplateFuncs = template.FuncMap{
	"packerParam": func(format octopus.Format) OctopusFormatParam {
		ret, ex := OctopusFormatMapper[format]
		if !ex {
			log.Fatalf("packer for type `%s` not found", format)
		}

		return ret
	},
	"mutatorParam": func(mut ds.FieldMutator, format octopus.Format) OctopusMutatorParam {
		ret, ex := OctopusMutatorMapper[mut]
		if !ex {
			log.Fatalf("mutator packer for type `%s` not found", format)
		}

		for _, availFormat := range ret.AvailableType {
			if availFormat == format {
				return ret
			}
		}

		log.Fatalf("Mutator `%s` not available for type `%s`", mut, format)

		return OctopusMutatorParam{}
	},
	"addImport": func(flds []ds.FieldDeclaration) (imports []string) {
		var needMath, needStrconv bool

		for _, fld := range flds {
			if fld.Format == octopus.Float32 || fld.Format == octopus.Float64 {
				needMath = true
			}

			if len(fld.Mutators) > 0 && fld.Format != "uint32" && fld.Format != "uint64" && fld.Format != "uint" {
				needMath = true
			}

			if fld.PrimaryKey && fld.Format != "string" {
				needStrconv = true
			}
		}

		if needMath {
			imports = append(imports, "math")
		}

		if needStrconv {
			imports = append(imports, "strconv")
		}

		return
	},
	"trimPrefix": strings.TrimPrefix,
	"hasPrefix":  strings.HasPrefix,
}

var ToLower = cases.Title(language.English)

type OctopusFormatParam struct {
	Name           string
	Default        []byte
	packFunc       string
	unpackFunc     string
	packConvFunc   string
	UnpackConvFunc string
	unpackType     string
	len            uint
	lenFunc        func(uint32) uint32
	minValue       string
	maxValue       string
	convstr        string
}

func (p OctopusFormatParam) PackConvFunc(fieldname string) string {
	if p.packConvFunc != "" {
		return p.packConvFunc + "(" + fieldname + ")"
	}

	return fieldname
}

func (p OctopusFormatParam) UnpackFunc() string {
	if p.unpackFunc != "" {
		return p.unpackFunc
	}

	return "iproto.Unpack" + p.Name
}

func (p OctopusFormatParam) PackFunc() string {
	if p.packFunc != "" {
		return p.packFunc
	}

	return "iproto.Pack" + p.Name
}

func (p OctopusFormatParam) DefaultValue() string {
	fname := "iproto.Pack" + p.Name
	if p.packFunc != "" {
		fname = p.packFunc
	}

	if strings.HasPrefix(p.Name, "Uint") {
		return fname + "([]byte{}, 0, iproto.ModeDefault)"
	} else if strings.HasPrefix(p.Name, "String") {
		return fname + `([]byte{}, "", iproto.ModeDefault)`
	} else {
		return "can't detect type"
	}
}

func (p OctopusFormatParam) UnpackType() string {
	if p.unpackType != "" {
		return p.unpackType
	} else if p.packConvFunc != "" {
		return p.packConvFunc
	}

	return ""
}

func (p OctopusFormatParam) Len(l uint32) uint {
	if p.len != 0 {
		return p.len
	}

	return uint(p.lenFunc(l))
}

func (p OctopusFormatParam) MinValue() string {
	if p.minValue != "" {
		return p.minValue
	}

	return "math.Min" + p.Name
}

func (p OctopusFormatParam) MaxValue() string {
	if p.maxValue != "" {
		return p.maxValue
	}

	return "math.Max" + p.Name
}

func (p OctopusFormatParam) ToString() []string {
	return strings.SplitN(p.convstr, `%%`, 2)
}

func (p OctopusFormatParam) MutatorTypeConv() string {
	if p.UnpackConvFunc != "" {
		return p.UnpackConvFunc
	}

	return ToLower.String(p.Name)
}

type OctopusMutatorParam struct {
	Name          string
	AvailableType []octopus.Format
	ArgType       string
}

var OctopusFormatMapper = map[octopus.Format]OctopusFormatParam{
	octopus.Bool:    {Name: "Uint8", len: 2, convstr: "strconv.FormatBool(%%)", packConvFunc: "octopus.BoolToUint", UnpackConvFunc: "octopus.UintToBool", unpackType: "uint8"},
	octopus.Uint8:   {Name: "Uint8", len: 2, convstr: "strconv.FormatUint(uint64(%%), 10)"},
	octopus.Uint16:  {Name: "Uint16", len: 3, convstr: "strconv.FormatUint(uint64(%%), 10)"},
	octopus.Uint32:  {Name: "Uint32", len: 5, convstr: "strconv.FormatUint(uint64(%%), 10)"},
	octopus.Uint64:  {Name: "Uint64", len: 9, convstr: "strconv.FormatUint(%%, 10)"},
	octopus.Uint:    {Name: "Uint32", len: 5, convstr: "strconv.FormatUint(uint64(%%), 10)", packConvFunc: "uint32", UnpackConvFunc: "uint"},
	octopus.Int8:    {Name: "Uint8", len: 2, convstr: "strconv.FormatInt(int64(%%), 10)", packConvFunc: "uint8", UnpackConvFunc: "int8", minValue: "math.MinInt8", maxValue: "math.MaxInt8"},
	octopus.Int16:   {Name: "Uint16", len: 3, convstr: "strconv.FormatInt(int64(%%), 10)", packConvFunc: "uint16", UnpackConvFunc: "int16", minValue: "math.MinInt16", maxValue: "math.MaxInt16"},
	octopus.Int32:   {Name: "Uint32", len: 5, convstr: "strconv.FormatInt(int64(%%), 10)", packConvFunc: "uint32", UnpackConvFunc: "int32", minValue: "math.MinInt32", maxValue: "math.MaxInt32"},
	octopus.Int64:   {Name: "Uint64", len: 9, convstr: "strconv.FormatInt(%%, 10)", packConvFunc: "uint64", UnpackConvFunc: "int64", minValue: "math.MinInt64", maxValue: "math.MaxInt64"},
	octopus.Int:     {Name: "Uint32", len: 5, convstr: "strconv.FormatInt(int64(%%), 10)", packConvFunc: "uint32", UnpackConvFunc: "int", minValue: "math.MinInt32", maxValue: "math.MaxInt32"},
	octopus.Float32: {Name: "Uint32", len: 5, convstr: "strconv.FormatFloat(%%, 32)", packConvFunc: "math.Float32bits", UnpackConvFunc: "math.Float32frombits", unpackType: "uint32", minValue: "math.MinFloat32", maxValue: "math.MaxFloat32"},
	octopus.Float64: {Name: "Uint64", len: 9, convstr: "strconv.FormatFloat(%%, 64)", packConvFunc: "math.Float64bits", UnpackConvFunc: "math.Float64frombits", unpackType: "uint64", minValue: "math.MinFloat64", maxValue: "math.MaxFloat64"},
	octopus.String:  {Name: "String", convstr: " %% ", lenFunc: octopus.ByteLen, packFunc: "octopus.PackString", unpackFunc: "octopus.UnpackString", minValue: "0", maxValue: "4096", unpackType: "string"}}

var OctopusMutatorMapper = map[ds.FieldMutator]OctopusMutatorParam{
	ds.IncMutator:      {Name: "Inc", AvailableType: octopus.NumericFormat},
	ds.DecMutator:      {Name: "Dec", AvailableType: octopus.NumericFormat},
	ds.AndMutator:      {Name: "And", AvailableType: octopus.UnsignedFormat},
	ds.OrMutator:       {Name: "Or", AvailableType: octopus.UnsignedFormat},
	ds.XorMutator:      {Name: "Xor", AvailableType: octopus.UnsignedFormat},
	ds.ClearBitMutator: {Name: "ClearBit", AvailableType: octopus.UnsignedFormat},
	ds.SetBitMutator:   {Name: "SetBit", AvailableType: octopus.UnsignedFormat},
}
