package parser

import (
	"go/ast"
	"strings"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

func ParseMutators(dst *ds.RecordPackage, fields []*ast.Field) error {
	for _, field := range fields {
		if field.Names == nil || len(field.Names) != 1 {
			return &arerror.ErrParseMutatorDecl{Err: arerror.ErrNameDeclaration}
		}

		mutatorDeclaration := ds.MutatorDeclaration{
			Name:       field.Names[0].Name,
			ImportName: "mutator" + field.Names[0].Name,
		}

		tagParam, err := splitTag(field, NoCheckFlag, map[TagNameType]ParamValueRule{})
		if err != nil {
			return &arerror.ErrParseMutatorDecl{Name: mutatorDeclaration.Name, Err: err}
		}

		for _, kv := range tagParam {
			switch kv[0] {
			case "pkg":
				mutatorDeclaration.Pkg = kv[1]
			case "update":
				mutatorDeclaration.Update = kv[1]
			case "replace":
				mutatorDeclaration.Replace = kv[1]
			default:
				return &arerror.ErrParseMutatorTagDecl{Name: mutatorDeclaration.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagUnknown}
			}
		}

		if mutatorDeclaration.Pkg != "" {
			imp, e := dst.FindOrAddImport(mutatorDeclaration.Pkg, mutatorDeclaration.ImportName)
			if e != nil {
				return &arerror.ErrParseMutatorDecl{Name: mutatorDeclaration.Name, Err: e}
			}

			mutatorDeclaration.ImportName = imp.ImportName
		}

		mutatorDeclaration.Type, err = ParseFieldType(dst, mutatorDeclaration.Name, "", field.Type)
		if err != nil {
			return &arerror.ErrParseMutatorDecl{Name: mutatorDeclaration.Name, Err: err}
		}

		// Ассоциируем указатель на тип с типом
		structType := strings.Replace(mutatorDeclaration.Type, "*", "", 1)

		mutatorDeclaration.PartialFields = dst.ImportStructFieldsMap[structType]

		if err = dst.AddMutator(mutatorDeclaration); err != nil {
			return err
		}
	}

	return nil
}
