package parser

import (
	"fmt"
	"go/ast"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

// Процесс парсинга декларативного описания связи между сущностями
func ParseFieldsObject(dst *ds.RecordPackage, fieldsobject []*ast.Field) error {
	for _, fieldobject := range fieldsobject {
		if fieldobject.Names == nil || len(fieldobject.Names) != 1 {
			return &arerror.ErrParseTypeFieldStructDecl{Err: arerror.ErrNameDeclaration}
		}

		newfieldobj := ds.FieldObject{
			Name: fieldobject.Names[0].Name,
		}

		switch t := fieldobject.Type.(type) {
		case *ast.Ident:
			if err := checkBoolType(fieldobject.Type); err != nil {
				return &arerror.ErrParseTypeFieldStructDecl{Name: newfieldobj.Name, FieldType: t.Name, Err: arerror.ErrTypeNotBool}
			}

			newfieldobj.Unique = true
		case *ast.ArrayType:
			if t.Len != nil {
				return &arerror.ErrParseTypeFieldStructDecl{Name: newfieldobj.Name, FieldType: t.Elt.(*ast.Ident).Name, Err: arerror.ErrTypeNotSlice}
			}

			if t.Elt.(*ast.Ident).Name != string(TypeBool) {
				return &arerror.ErrParseTypeFieldStructDecl{Name: newfieldobj.Name, FieldType: t.Elt.(*ast.Ident).Name, Err: arerror.ErrTypeNotBool}
			}

			newfieldobj.Unique = false
		default:
			return &arerror.ErrParseTypeFieldStructDecl{Name: newfieldobj.Name, FieldType: fmt.Sprintf("%T", t), Err: arerror.ErrUnknown}
		}

		tagParam, err := splitTag(fieldobject, CheckFlagEmpty, map[TagNameType]ParamValueRule{})
		if err != nil {
			return &arerror.ErrParseTypeFieldStructDecl{Name: newfieldobj.Name, Err: err}
		}

		for _, kv := range tagParam {
			switch kv[0] {
			case "field":
				fldNum, ok := dst.FieldsMap[kv[1]]
				if !ok {
					return &arerror.ErrParseTypeFieldObjectTagDecl{Name: newfieldobj.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrFieldNotExist}
				}

				dst.Fields[fldNum].ObjectLink = newfieldobj.Name
				newfieldobj.Field = kv[1]
			case "key":
				newfieldobj.Key = kv[1]
			case "object":
				newfieldobj.ObjectName = kv[1]
			default:
				return &arerror.ErrParseTypeFieldObjectTagDecl{Name: newfieldobj.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagUnknown}
			}
		}

		if err = dst.AddFieldObject(newfieldobj); err != nil {
			return err
		}
	}

	return nil
}
