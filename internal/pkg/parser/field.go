package parser

import (
	"fmt"
	"go/ast"
	"strconv"
	"strings"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/pkg/octopus"
)

// Функция парсинга тегов полей модели
func ParseFieldsTag(field *ast.Field, newfield *ds.FieldDeclaration, newindex *ds.IndexDeclaration) error {
	tagParam, err := splitTag(field, NoCheckFlag, map[TagNameType]ParamValueRule{PrimaryKeyTag: ParamNotNeedValue, UniqueTag: ParamNotNeedValue})
	if err != nil {
		return &arerror.ErrParseTypeFieldDecl{Name: newfield.Name, FieldType: string(newfield.Format), Err: err}
	}

	if len(tagParam) > 0 {
		for _, kv := range tagParam {
			switch TagNameType(kv[0]) {
			case SelectorTag:
				newindex.Name = newfield.Name
				newindex.Selector = kv[1]
			case PrimaryKeyTag:
				newindex.Name = newfield.Name
				newindex.Primary = true
			case UniqueTag:
				newindex.Name = newfield.Name
				newindex.Unique = true
			case MutatorsTag:
				for _, mut := range strings.Split(kv[1], ",") {
					fieldMutatorsChecker := ds.GetFieldMutatorsChecker()

					m, ex := fieldMutatorsChecker[mut]
					if !ex {
						return &arerror.ErrParseTypeFieldTagDecl{Name: newfield.Name, TagName: kv[0], TagValue: mut, Err: arerror.ErrParseFieldMutatorInvalid}
					}

					newfield.Mutators = append(newfield.Mutators, m)
				}
			case SizeTag:
				if kv[1] != "" {
					size, err := strconv.ParseInt(kv[1], 10, 64)
					if err != nil {
						return &arerror.ErrParseTypeFieldTagDecl{Name: newfield.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagValueInvalid}
					}

					newfield.Size = size
				}
			case SerializerTag:
				newfield.Serializer = strings.Split(kv[1], ",")
			default:
				return &arerror.ErrParseTypeFieldTagDecl{Name: newfield.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagUnknown}
			}
		}
	}

	return nil
}

// Функция парсинга полей модели
func ParseFields(dst *ds.RecordPackage, fields []*ast.Field) error {
	for _, field := range fields {
		if field.Names == nil || len(field.Names) != 1 {
			return &arerror.ErrParseTypeFieldDecl{Err: arerror.ErrNameDeclaration}
		}

		newfield := ds.FieldDeclaration{
			Name:       field.Names[0].Name,
			Mutators:   []ds.FieldMutator{},
			Serializer: []string{},
		}

		newindex := ds.IndexDeclaration{
			FieldsMap: map[string]ds.IndexField{},
		}

		switch t := field.Type.(type) {
		case *ast.Ident:
			newfield.Format = octopus.Format(t.String())
		case *ast.ArrayType:
			//Todo точно ли массив надо, а не срез?
			if t.Elt.(*ast.Ident).Name != "byte" {
				return &arerror.ErrParseTypeFieldDecl{Name: newfield.Name, FieldType: t.Elt.(*ast.Ident).Name, Err: arerror.ErrParseFieldArrayOfNotByte}
			}

			if t.Len == nil {
				return &arerror.ErrParseTypeFieldDecl{Name: newfield.Name, FieldType: t.Elt.(*ast.Ident).Name, Err: arerror.ErrParseFieldArrayNotSlice}
			}

			return &arerror.ErrParseTypeFieldDecl{Name: newfield.Name, FieldType: t.Elt.(*ast.Ident).Name, Err: arerror.ErrParseFieldBinary}
		default:
			return &arerror.ErrParseTypeFieldDecl{Name: newfield.Name, FieldType: fmt.Sprintf("%T", t), Err: arerror.ErrUnknown}
		}

		if err := ParseFieldsTag(field, &newfield, &newindex); err != nil {
			return fmt.Errorf("error ParseFieldsTag: %w", err)
		}

		if err := dst.AddField(newfield); err != nil {
			return err
		}

		if newindex.Name != "" {
			newindex.Fields = append(newindex.Fields, len(dst.Fields)-1)
			newindex.FieldsMap[newfield.Name] = ds.IndexField{IndField: len(dst.Fields) - 1, Order: ds.IndexOrderAsc}

			errIndex := dst.AddIndex(newindex)
			if errIndex != nil {
				return &arerror.ErrParseTypeFieldDecl{Name: newfield.Name, FieldType: string(newfield.Format), Err: errIndex}
			}
		}
	}

	return nil
}

// Функция парсинга тегов полей модели
func ParseProcFieldsTag(field *ast.Field, newfield *ds.ProcFieldDeclaration) error {
	tagParam, err := splitTag(field, NoCheckFlag, map[TagNameType]ParamValueRule{PrimaryKeyTag: ParamNotNeedValue, UniqueTag: ParamNotNeedValue})
	if err != nil {
		return &arerror.ErrParseTypeFieldDecl{Name: newfield.Name, FieldType: string(newfield.Format), Err: err}
	}

	if len(tagParam) > 0 {
		for _, kv := range tagParam {
			switch TagNameType(kv[0]) {
			case ProcInputParamTag:
				//результат бинарной операции 0|IN => IN; 1|IN => IN; 2|IN => INTOUT (3);
				newfield.Type = newfield.Type | ds.IN
			case ProcOutputParamTag:
				//результат бинарной операции 0|OUT => OUT; 1|OUT => INTOUT (3); 2|OUT => OUT;
				newfield.Type = newfield.Type | ds.OUT
			case SizeTag:
				if kv[1] != "" {
					size, err := strconv.ParseInt(kv[1], 10, 64)
					if err != nil {
						return &arerror.ErrParseTypeFieldTagDecl{Name: newfield.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagValueInvalid}
					}

					newfield.Size = size
				}
			case SerializerTag:
				newfield.Serializer = strings.Split(kv[1], ",")
			default:
				return &arerror.ErrParseTypeFieldTagDecl{Name: newfield.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagUnknown}
			}
		}
	}

	return nil
}

// Функция парсинга полей процедуры
func ParseProcFields(dst *ds.RecordPackage, fields []*ast.Field) error {
	for _, field := range fields {
		if field.Names == nil || len(field.Names) != 1 {
			return &arerror.ErrParseTypeFieldDecl{Err: arerror.ErrNameDeclaration}
		}

		newField := ds.ProcFieldDeclaration{
			Name:       field.Names[0].Name,
			Serializer: []string{},
		}

		switch t := field.Type.(type) {
		case *ast.Ident:
			newField.Format = octopus.Format(t.String())
		case *ast.ArrayType:
			if t.Elt.(*ast.Ident).Name != "byte" {
				return &arerror.ErrParseTypeFieldDecl{Name: newField.Name, FieldType: t.Elt.(*ast.Ident).Name, Err: arerror.ErrParseFieldArrayOfNotByte}
			}

			if t.Len == nil {
				return &arerror.ErrParseTypeFieldDecl{Name: newField.Name, FieldType: t.Elt.(*ast.Ident).Name, Err: arerror.ErrParseFieldArrayNotSlice}
			}

			return &arerror.ErrParseTypeFieldDecl{Name: newField.Name, FieldType: t.Elt.(*ast.Ident).Name, Err: arerror.ErrParseFieldBinary}
		default:
			return &arerror.ErrParseTypeFieldDecl{Name: newField.Name, FieldType: fmt.Sprintf("%T", t), Err: arerror.ErrUnknown}
		}

		if err := ParseProcFieldsTag(field, &newField); err != nil {
			return fmt.Errorf("error ParseFieldsTag: %w", err)
		}

		if err := dst.AddProcField(newField); err != nil {
			return err
		}
	}

	return nil
}
