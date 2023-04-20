package parser

import (
	"fmt"
	"go/ast"
	"strconv"
	"strings"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

func ParseIndexPartTag(field *ast.Field, ind *ds.IndexDeclaration, indexMap map[string]int, fields []ds.FieldDeclaration, indexes []ds.IndexDeclaration) error {
	tagParam, err := splitTag(field, CheckFlagEmpty, map[TagNameType]ParamValueRule{})
	if err != nil {
		return &arerror.ErrParseTypeIndexDecl{IndexType: "indexpart", Name: ind.Name, Err: err}
	}

	var exInd ds.IndexDeclaration

	var fieldnum int64 = 1

	for _, kv := range tagParam {
		switch kv[0] {
		case "selector":
			ind.Selector = kv[1]
		case "index":
			exIndNum, ex := indexMap[kv[1]]
			if !ex {
				return &arerror.ErrParseTypeIndexTagDecl{IndexType: "indexpart", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrIndexNotExist}
			}

			exInd = indexes[exIndNum]
			ind.Num = exInd.Num
		case "fieldnum":
			fNum, err := strconv.ParseInt(kv[1], 10, 64)
			if err != nil {
				return &arerror.ErrParseTypeIndexTagDecl{IndexType: "indexpart", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagValueInvalid}
			}

			fieldnum = fNum
		default:
			return &arerror.ErrParseTypeIndexTagDecl{IndexType: "indexpart", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagUnknown}
		}
	}

	if int64(len(exInd.Fields)) < fieldnum {
		return &arerror.ErrParseTypeIndexDecl{IndexType: "indexpart", Name: ind.Name, Err: arerror.ErrParseIndexFieldnumToBig}
	}

	if int64(len(exInd.Fields)) == fieldnum {
		return &arerror.ErrParseTypeIndexDecl{IndexType: "indexpart", Name: ind.Name, Err: arerror.ErrParseIndexFieldnumEqual}
	}

	for f := int64(0); f < fieldnum; f++ {
		ind.Fields = append(ind.Fields, exInd.Fields[f])
		ind.FieldsMap[fields[exInd.Fields[f]].Name] = exInd.FieldsMap[fields[exInd.Fields[f]].Name]
	}

	return nil
}

func ParseIndexPart(dst *ds.RecordPackage, fields []*ast.Field) error {
	if len(fields) == 0 {
		return nil
	}

	for _, field := range fields {
		if field.Names == nil || len(field.Names) != 1 {
			return &arerror.ErrParseTypeIndexDecl{IndexType: "indexpart", Err: arerror.ErrNameDeclaration}
		}

		ind := ds.IndexDeclaration{
			Name:      field.Names[0].Name,
			Fields:    []int{},
			FieldsMap: map[string]ds.IndexField{},
			Selector:  "SelectBy" + field.Names[0].Name,
			Partial:   true,
		}

		if err := checkBoolType(field.Type); err != nil {
			return &arerror.ErrParseTypeIndexDecl{IndexType: "indexpart", Name: ind.Name, Err: arerror.ErrTypeNotBool}
		}

		if err := ParseIndexPartTag(field, &ind, dst.IndexMap, dst.Fields, dst.Indexes); err != nil {
			return fmt.Errorf("error parse trigger tag: %w", err)
		}

		if len(ind.Fields) == 0 {
			return &arerror.ErrParseTypeIndexDecl{IndexType: "indexpart", Name: ind.Name, Err: arerror.ErrParseIndexFieldnumRequired}
		}

		if err := dst.AddIndex(ind); err != nil {
			return err
		}
	}

	return nil
}

func ParseIndexTag(field *ast.Field, ind *ds.IndexDeclaration, fieldsMap map[string]int) error {
	tagParam, err := splitTag(field, CheckFlagEmpty, map[TagNameType]ParamValueRule{PrimaryKeyTag: ParamNotNeedValue, UniqueTag: ParamNotNeedValue})
	if err != nil {
		return &arerror.ErrParseTypeIndexDecl{IndexType: "index", Name: ind.Name, Err: err}
	}

	for _, kv := range tagParam {
		switch TagNameType(kv[0]) {
		case PrimaryKeyTag:
			ind.Primary = true
		case UniqueTag:
			ind.Unique = true
		case SelectorTag:
			ind.Selector = kv[1]
		case FieldsTag:
			for _, fieldName := range strings.Split(kv[1], ",") {
				if fldNum, ex := fieldsMap[fieldName]; !ex {
					return &arerror.ErrParseTypeIndexTagDecl{IndexType: "index", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrFieldNotExist}
				} else if _, ex := ind.FieldsMap[fieldName]; ex {
					return &arerror.ErrParseTypeIndexTagDecl{IndexType: "index", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrDuplicate}
				} else {
					ind.FieldsMap[fieldName] = ds.IndexField{IndField: fldNum, Order: ds.IndexOrderAsc}
					ind.Fields = append(ind.Fields, fldNum)
				}
			}
		case OrderDescTag:
			for _, fn := range strings.Split(kv[1], ",") {
				if _, ex := fieldsMap[fn]; !ex {
					return &arerror.ErrParseTypeIndexTagDecl{IndexType: "index", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrFieldNotExist}
				} else if indField, ex := ind.FieldsMap[fn]; !ex {
					return &arerror.ErrParseTypeIndexTagDecl{IndexType: "index", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrFieldNotExist}
				} else {
					indField.Order = ds.IndexOrderDesc
				}
			}
		default:
			return &arerror.ErrParseTypeIndexTagDecl{IndexType: "index", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagUnknown}
		}
	}

	return nil
}

func ParseIndexes(dst *ds.RecordPackage, fields []*ast.Field) error {
	if len(fields) == 0 {
		return nil
	}

	for _, field := range fields {
		if field.Names == nil || len(field.Names) != 1 {
			return &arerror.ErrParseTypeIndexDecl{IndexType: "index", Err: arerror.ErrNameDeclaration}
		}

		ind := ds.IndexDeclaration{
			Name:      field.Names[0].Name,
			Fields:    []int{},
			FieldsMap: map[string]ds.IndexField{},
			Selector:  "SelectBy" + field.Names[0].Name}

		if err := checkBoolType(field.Type); err != nil {
			return &arerror.ErrParseTypeIndexDecl{IndexType: "index", Name: ind.Name, Err: arerror.ErrTypeNotBool}
		}

		if err := ParseIndexTag(field, &ind, dst.FieldsMap); err != nil {
			return fmt.Errorf("error parse indexTag: %w", err)
		}

		if err := dst.AddIndex(ind); err != nil {
			return err
		}
	}

	return nil
}

func ParseParams(dst *ds.RecordPackage, fields []*ast.Field) error {
	if len(fields) == 0 {
		return nil
	}

	for _, field := range fields {
		if field.Names == nil || len(field.Names) != 1 {
			return &arerror.ErrParseTypeIndexDecl{IndexType: "index", Err: arerror.ErrNameDeclaration}
		}

		ind := ds.IndexDeclaration{
			Name:      field.Names[0].Name,
			Fields:    []int{},
			FieldsMap: map[string]ds.IndexField{},
			Selector:  "SelectBy" + field.Names[0].Name}

		if err := checkBoolType(field.Type); err != nil {
			return &arerror.ErrParseTypeIndexDecl{IndexType: "index", Name: ind.Name, Err: arerror.ErrTypeNotBool}
		}

		if err := ParseParamTag(field, &ind, dst.FieldsMap); err != nil {
			return fmt.Errorf("error parse indexTag: %w", err)
		}

		if err := dst.AddIndex(ind); err != nil {
			return err
		}
	}

	return nil
}

func ParseParamTag(field *ast.Field, ind *ds.IndexDeclaration, fieldsMap map[string]int) error {
	tagParam, err := splitTag(field, CheckFlagEmpty, map[TagNameType]ParamValueRule{PrimaryKeyTag: ParamNotNeedValue, UniqueTag: ParamNotNeedValue})
	if err != nil {
		return &arerror.ErrParseTypeIndexDecl{IndexType: "index", Name: ind.Name, Err: err}
	}

	for _, kv := range tagParam {
		switch TagNameType(kv[0]) {
		case SelectorTag:
			ind.Selector = kv[1]
		case FieldsTag:
			for _, fieldName := range strings.Split(kv[1], ",") {
				if fldNum, ex := fieldsMap[fieldName]; !ex {
					return &arerror.ErrParseTypeIndexTagDecl{IndexType: "index", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrFieldNotExist}
				} else if _, ex := ind.FieldsMap[fieldName]; ex {
					return &arerror.ErrParseTypeIndexTagDecl{IndexType: "index", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrDuplicate}
				} else {
					ind.FieldsMap[fieldName] = ds.IndexField{IndField: fldNum, Order: ds.IndexOrderAsc}
					ind.Fields = append(ind.Fields, fldNum)
				}
			}
		case OrderDescTag:
			for _, fn := range strings.Split(kv[1], ",") {
				if _, ex := fieldsMap[fn]; !ex {
					return &arerror.ErrParseTypeIndexTagDecl{IndexType: "index", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrFieldNotExist}
				} else if indField, ex := ind.FieldsMap[fn]; !ex {
					return &arerror.ErrParseTypeIndexTagDecl{IndexType: "index", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrFieldNotExist}
				} else {
					indField.Order = ds.IndexOrderDesc
				}
			}
		default:
			return &arerror.ErrParseTypeIndexTagDecl{IndexType: "index", Name: ind.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagUnknown}
		}
	}

	return nil
}
