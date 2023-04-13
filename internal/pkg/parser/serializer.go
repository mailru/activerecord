package parser

import (
	"go/ast"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

func ParseTypeSerializer(dst *ds.RecordPackage, serializerName string, t interface{}) (string, error) {
	switch tv := t.(type) {
	case *ast.Ident:
		return tv.String(), nil
	case *ast.ArrayType:
		var err error

		len := ""
		if tv.Len != nil {
			len, err = ParseTypeSerializer(dst, serializerName, tv.Len)
			if err != nil {
				return "", err
			}
		}

		t, err := ParseTypeSerializer(dst, serializerName, tv.Elt)
		if err != nil {
			return "", err
		}

		return "[" + len + "]" + t, nil
	case *ast.InterfaceType:
		return "interface{}", nil
	case *ast.StarExpr:
		t, err := ParseTypeSerializer(dst, serializerName, tv.X)
		if err != nil {
			return "", err
		}

		return "*" + t, nil
	case *ast.MapType:
		k, err := ParseTypeSerializer(dst, serializerName, tv.Key)
		if err != nil {
			return "", nil
		}

		v, err := ParseTypeSerializer(dst, serializerName, tv.Value)
		if err != nil {
			return "", nil
		}

		return "map[" + k + "]" + v, nil
	case *ast.SelectorExpr:
		pName, err := ParseTypeSerializer(dst, serializerName, tv.X)
		if err != nil {
			return "", err
		}

		imp, err := dst.FindImportByPkg(pName)
		if err != nil {
			return "", &arerror.ErrParseSerializerTypeDecl{Name: serializerName, SerializerType: tv, Err: err}
		}

		reqImportName := imp.ImportName
		if reqImportName == "" {
			reqImportName = pName
		}

		return reqImportName + "." + tv.Sel.Name, nil
	default:
		return "", &arerror.ErrParseSerializerTypeDecl{Name: serializerName, SerializerType: tv, Err: arerror.ErrUnknown}
	}
}

func ParseSerializer(dst *ds.RecordPackage, fields []*ast.Field) error {
	defaultSerializerPkg := "github.com/mailru/activerecord/pkg/serializer"
	for _, field := range fields {
		if field.Names == nil || len(field.Names) != 1 {
			return &arerror.ErrParseSerializerDecl{Err: arerror.ErrNameDeclaration}
		}

		newserializer := ds.SerializerDeclaration{
			Name:        field.Names[0].Name,
			ImportName:  "serializer" + field.Names[0].Name,
			Pkg:         defaultSerializerPkg,
			Marshaler:   field.Names[0].Name + "Marshal",
			Unmarshaler: field.Names[0].Name + "Unmarshal",
		}

		tagParam, err := splitTag(field, NoCheckFlag, map[TagNameType]ParamValueRule{})
		if err != nil {
			return &arerror.ErrParseSerializerDecl{Name: newserializer.Name, Err: err}
		}

		for _, kv := range tagParam {
			switch kv[0] {
			case "pkg":
				newserializer.Pkg = kv[1]
			case "marshaler":
				newserializer.Marshaler = kv[1]
			case "unmarshaler":
				newserializer.Unmarshaler = kv[1]
			default:
				return &arerror.ErrParseSerializerTagDecl{Name: newserializer.Name, TagName: kv[0], TagValue: kv[1], Err: arerror.ErrParseTagUnknown}
			}
		}

		imp, err := dst.FindOrAddImport(newserializer.Pkg, newserializer.ImportName)
		if err != nil {
			return &arerror.ErrParseSerializerDecl{Name: newserializer.Name, Err: err}
		}

		newserializer.ImportName = imp.ImportName

		newserializer.Type, err = ParseTypeSerializer(dst, newserializer.Name, field.Type)
		if err != nil {
			return &arerror.ErrParseSerializerDecl{Name: newserializer.Name, Err: err}
		}

		if err = dst.AddSerializer(newserializer); err != nil {
			return err
		}
	}

	return nil
}
