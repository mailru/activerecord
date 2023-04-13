package parser

import (
	"go/ast"
	"strings"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

// Парсинг флагов. В описании модели можно указать, что целочисленное значение используется для хранения
// битовых флагов. В этом случае на поле навешиваются мутаторы SetFlag и ClearFlag
func ParseFlags(dst *ds.RecordPackage, fields []*ast.Field) error {
	for _, field := range fields {
		if field.Names == nil || len(field.Names) != 1 {
			return &arerror.ErrParseFlagDecl{Err: arerror.ErrNameDeclaration}
		}

		newflag := ds.FlagDeclaration{
			Name:  field.Names[0].Name,
			Flags: []string{},
		}

		tagParam, err := splitTag(field, CheckFlagEmpty, map[TagNameType]ParamValueRule{})
		if err != nil {
			return &arerror.ErrParseFlagDecl{Name: newflag.Name, Err: err}
		}

		for _, kv := range tagParam {
			switch kv[0] {
			case "flags":
				newflag.Flags = strings.Split(kv[1], ",")
			default:
				return &arerror.ErrParseFlagTagDecl{Name: newflag.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagUnknown}
			}
		}

		fldNum, ok := dst.FieldsMap[newflag.Name]
		if !ok {
			return &arerror.ErrParseFlagDecl{Name: newflag.Name, Err: arerror.ErrFieldNotExist}
		}

		foundSet, foundClear := false, false

		for _, mut := range dst.Fields[fldNum].Mutators {
			if mut == ds.SetBitMutator {
				foundSet = true
			}

			if mut == ds.SetBitMutator {
				foundClear = true
			}
		}

		if !foundSet {
			dst.Fields[fldNum].Mutators = append(dst.Fields[fldNum].Mutators, ds.SetBitMutator)
		}

		if !foundClear {
			dst.Fields[fldNum].Mutators = append(dst.Fields[fldNum].Mutators, ds.ClearBitMutator)
		}

		if err = dst.AddFlag(newflag); err != nil {
			return err
		}
	}

	return nil
}
