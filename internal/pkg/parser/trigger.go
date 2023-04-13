package parser

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

// Список доступных триггеров
var availableTriggers = map[string]map[string]bool{
	"RepairTuple":        {"Defaults": true},
	"DublicateUniqTuple": {},
}

// Парсинг заявленных триггеров в описании модели
func ParseTrigger(dst *ds.RecordPackage, fields []*ast.Field) error {
	for _, field := range fields {
		if field.Names == nil || len(field.Names) != 1 {
			return &arerror.ErrParseTriggerDecl{Err: arerror.ErrNameDeclaration}
		}

		if err := checkBoolType(field.Type); err != nil {
			return &arerror.ErrParseTriggerDecl{Err: arerror.ErrTypeNotBool}
		}

		trigger := ds.TriggerDeclaration{
			Name:       field.Names[0].Name,
			Func:       field.Names[0].Name,
			Params:     map[string]bool{},
			ImportName: "trigger" + field.Names[0].Name,
		}

		if err := ParseTriggerTag(&trigger, field); err != nil {
			return fmt.Errorf("error parse trigger tag: %w", err)
		}

		if trigger.Pkg == "" {
			return &arerror.ErrParseTriggerDecl{Name: field.Names[0].Name, Err: arerror.ErrParseTriggerPackageNotDefined}
		}

		imp, err := dst.FindOrAddImport(trigger.Pkg, trigger.ImportName)
		if err != nil {
			return &arerror.ErrParseTriggerDecl{Name: trigger.Name, Err: err}
		}

		trigger.ImportName = imp.ImportName

		if err = dst.AddTrigger(trigger); err != nil {
			return err
		}
	}

	return nil
}

func ParseTriggerTag(trigger *ds.TriggerDeclaration, field *ast.Field) error {
	atr, ex := availableTriggers[field.Names[0].Name]
	if !ex {
		return &arerror.ErrParseTriggerDecl{Name: field.Names[0].Name, Err: arerror.ErrUnknown}
	}

	tagParam, err := splitTag(field, CheckFlagEmpty, map[TagNameType]ParamValueRule{})
	if err != nil {
		return &arerror.ErrParseTriggerDecl{Name: field.Names[0].Name, Err: err}
	}

	for _, kv := range tagParam {
		switch kv[0] {
		case "pkg":
			trigger.Pkg = kv[1]
		case "func":
			trigger.Func = kv[1]
		case "param":
			for _, param := range strings.Split(kv[1], ",") {
				if _, ex := atr[param]; ex {
					trigger.Params[param] = true
				} else {
					return &arerror.ErrParseTriggerTagDecl{Name: field.Names[0].Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagUnknown}
				}
			}
		default:
			return &arerror.ErrParseTriggerTagDecl{Name: field.Names[0].Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagUnknown}
		}
	}

	return nil
}
