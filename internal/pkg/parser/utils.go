package parser

import (
	"go/ast"
	"regexp"
	"strings"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var PublicNameChecker = regexp.MustCompile("^[A-Z]")
var ToLower = cases.Lower(language.English)
var availableNodeName = []StructNameType{
	FieldsObject,
	Fields,
	ProcFields,
	Indexes,
	IndexParts,
	Serializers,
	Triggers,
	Flags,
	Mutators,
}

func getNodeName(node string) (name string, publicName string, packageName string, err error) {
	for _, nName := range availableNodeName {
		if strings.HasPrefix(node, string(nName)) {
			name = string(nName)
			publicName = node[len(nName):]

			break
		}
	}

	if publicName == "" {
		err = arerror.ErrParseNodeNameUnknown
		return
	}

	if !PublicNameChecker.MatchString(publicName) {
		err = arerror.ErrParseNodeNameInvalid
		return
	}

	packageName = ToLower.String(publicName)

	return
}

const (
	NoCheckFlag    = 0
	CheckFlagEmpty = 1 << iota
)

type ParamValueRule int

const (
	ParamNeedValue ParamValueRule = iota
	ParamNotNeedValue
)

const NameDefaultRule = "__DEFAULT__"

func splitTag(field *ast.Field, checkFlag uint32, rule map[TagNameType]ParamValueRule) ([][]string, error) {
	if field.Tag == nil {
		return nil, arerror.ErrParseTagSplitAbsent
	}

	tag := searchARTag(field)

	if checkFlag&NoCheckFlag != 0 && tag == "" {
		return [][]string{}, nil
	}

	if checkFlag&CheckFlagEmpty != 0 && (tag == "" || tag == "`ar:\"\"`") {
		return nil, arerror.ErrParseTagInvalidFormat
	}

	idx := strings.LastIndex(tag, "ar:\"")

	return splitParam(tag[idx+3:len(tag)-1], rule)
}

func searchARTag(field *ast.Field) string {
	tags := strings.Split(field.Tag.Value, " ")

	for _, tag := range tags {
		if strings.Contains(tag, "ar:\"") {
			return tag
		}
	}

	return ""
}

func splitParam(str string, rule map[TagNameType]ParamValueRule) ([][]string, error) {
	if _, ex := rule[NameDefaultRule]; !ex {
		rule[NameDefaultRule] = ParamNeedValue
	}

	ret := [][]string{}

	for _, param := range strings.Split(strings.Trim(str, "\""), ";") {
		if param != "" {
			kv := strings.SplitN(param, ":", 2)

			r, ok := rule[TagNameType(kv[0])]
			if !ok {
				r = rule[NameDefaultRule]
			}

			if r == ParamNotNeedValue && len(kv) == 2 {
				return nil, &arerror.ErrParseTagDecl{Name: kv[0], Err: arerror.ErrParseTagWithValue}
			}

			ret = append(ret, kv)
		}
	}

	return ret, nil
}

func checkBoolType(indType ast.Expr) error {
	switch t := indType.(type) {
	case *ast.Ident:
		if t.String() != string(TypeBool) {
			return arerror.ErrTypeNotBool
		}
	default:
		return arerror.ErrTypeNotBool
	}

	return nil
}
