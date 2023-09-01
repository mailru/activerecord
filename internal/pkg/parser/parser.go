package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

type StructNameType string

const (
	Fields       StructNameType = "Fields"
	ProcFields   StructNameType = "ProcFields"
	FieldsObject StructNameType = "FieldsObject"
	Indexes      StructNameType = "Indexes"
	IndexParts   StructNameType = "IndexParts"
	Serializers  StructNameType = "Serializers"
	Triggers     StructNameType = "Triggers"
	Flags        StructNameType = "Flags"
	Mutators     StructNameType = "Mutators"
)

type TagNameType string

const (
	SelectorTag        TagNameType = "selector"
	PrimaryKeyTag      TagNameType = "primary_key"
	UniqueTag          TagNameType = "unique"
	MutatorsTag        TagNameType = "mutators"
	SizeTag            TagNameType = "size"
	SerializerTag      TagNameType = "serializer"
	FieldsTag          TagNameType = "fields"
	OrderDescTag       TagNameType = "orderdesc"
	ProcInputParamTag  TagNameType = "input"
	ProcOutputParamTag TagNameType = "output"
)

type TypeName string

const (
	TypeBool TypeName = "bool"
)

// Parse запуск парсера
func Parse(srcFileName string, rc *ds.RecordPackage) error {
	fset := token.NewFileSet()

	node, err := parser.ParseFile(fset, srcFileName, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("error parse file `%s`: %w", srcFileName, err)
	}

	//ast.Print(&token.FileSet{}, node)
	if node.Name.Name != "repository" {
		return fmt.Errorf("package name in file `%s` is not `repository` skipping", srcFileName)
	}

	err = parseAst(srcFileName, node.Decls, rc)
	if err != nil {
		return fmt.Errorf("error model(%s) parse: %s", srcFileName, err)
	}

	return nil
}

// parseAst парсинг декларативного описания модели (ast)
// Пока нельзя объявлять функции
func parseAst(pkgName string, decls []ast.Decl, rc *ds.RecordPackage) error {
	for _, decl := range decls {
		switch gen := decl.(type) {
		case *ast.GenDecl:
			genErr := parseGen(rc, gen)
			if genErr != nil {
				return &arerror.ErrParseGenDecl{Name: pkgName, Err: genErr}
			}
		case *ast.FuncDecl:
			return &arerror.ErrParseGenDecl{Name: pkgName, Err: arerror.ErrParseFuncDeclNotSupported}
		default:
			return &arerror.ErrParseGenTypeDecl{Name: pkgName, Type: decl, Err: arerror.ErrUnknown}
		}
	}

	return nil
}

func parseStructNameType(dst *ds.RecordPackage, nodeName string, curr *ast.StructType) error {
	switch StructNameType(nodeName) {
	case Fields:
		return ParseFields(dst, curr.Fields.List)
	case FieldsObject:
		return ParseFieldsObject(dst, curr.Fields.List)
	case Indexes:
		return ParseIndexes(dst, curr.Fields.List)
	case IndexParts:
		return ParseIndexPart(dst, curr.Fields.List)
	case Serializers:
		return ParseSerializer(dst, curr.Fields.List)
	case Triggers:
		return ParseTrigger(dst, curr.Fields.List)
	case Flags:
		return ParseFlags(dst, curr.Fields.List)
	case ProcFields:
		return ParseProcFields(dst, curr.Fields.List)
	case Mutators:
		return ParseMutators(dst, curr.Fields.List)
	default:
		return arerror.ErrUnknown
	}
}

func parseStructType(dst *ds.RecordPackage, name string, doc *ast.CommentGroup, curr *ast.StructType) error {
	nodeName, public, private, err := getNodeName(name)
	if err != nil {
		return &arerror.ErrParseGenDecl{Name: name, Err: err}
	}

	switch StructNameType(nodeName) {
	case Fields:
		fallthrough
	case ProcFields:
		if err = parseDoc(dst, nodeName, doc); err != nil {
			return err
		}

		dst.Namespace.PublicName = public
		dst.Namespace.PackageName = private
	}

	if err = parseStructNameType(dst, nodeName, curr); err != nil {
		return &arerror.ErrParseTypeStructDecl{Name: nodeName, Err: err}
	}

	if dst.Namespace.PublicName != public || dst.Namespace.PackageName != private {
		return &arerror.ErrParseTypeStructDecl{Name: nodeName, Err: arerror.ErrParseFieldNameInvalid}
	}

	return nil
}

func parseTokenType(dst *ds.RecordPackage, genD *ast.GenDecl) error {
	for _, spec := range genD.Specs {
		currType, ok := spec.(*ast.TypeSpec)
		if !ok || currType.Type == nil {
			return &arerror.ErrParseGenDecl{Name: genD.Tok.String(), Err: arerror.ErrParseCastSpecType}
		}

		switch curr := currType.Type.(type) {
		case *ast.StructType:
			if curr.Fields == nil {
				return &arerror.ErrParseTypeStructDecl{Name: currType.Name.Name, Err: arerror.ErrParseStructureEmpty}
			}

			if err := parseStructType(dst, currType.Name.Name, genD.Doc, curr); err != nil {
				return err
			}
		default:
			return &arerror.ErrParseGenDecl{Name: genD.Tok.String(), Err: arerror.ErrUnknown}
		}
	}

	return nil
}

// parseTokenImport финальные проверки перед парсингом импорта
func parseTokenImport(dst *ds.RecordPackage, genD *ast.GenDecl) error {
	for _, spec := range genD.Specs {
		currImport, ok := spec.(*ast.ImportSpec)
		if !ok {
			return &arerror.ErrParseGenDecl{Name: genD.Tok.String(), Err: arerror.ErrParseCastImportType}
		}

		if impErr := ParseImport(dst, currImport); impErr != nil {
			return &arerror.ErrParseGenDecl{Name: genD.Tok.String(), Err: impErr}
		}
	}

	return nil
}

// parseGen парсинг generic declaration
// На данный момент поддерживаются только импорты и структуры
func parseGen(dst *ds.RecordPackage, genD *ast.GenDecl) error {
	switch genD.Tok {
	case token.IMPORT:
		return parseTokenImport(dst, genD)
	case token.TYPE:
		return parseTokenType(dst, genD)
	case token.CONST:
		return &arerror.ErrParseGenDecl{Name: genD.Tok.String(), Err: arerror.ErrParseConst}
	case token.VAR:
		return &arerror.ErrParseGenDecl{Name: genD.Tok.String(), Err: arerror.ErrParseVar}
	default:
		return &arerror.ErrParseGenDecl{Name: genD.Tok.String(), Err: arerror.ErrUnknown}
	}
}

// parseDoc парсинг описания сервеной конфигурации в модели
//
//nolint:gocognit,gocyclo
func parseDoc(dst *ds.RecordPackage, nodeName string, doc *ast.CommentGroup) error {
	if doc == nil {
		return arerror.ErrParseDocEmptyBoxDeclaration
	}

	for _, doc := range doc.List {
		if strings.HasPrefix(doc.Text, "//ar:") {
			params, err := splitParam(doc.Text[5:], map[TagNameType]ParamValueRule{})
			if err != nil {
				return err
			}

			for _, kv := range params {
				switch kv[0] {
				case "serverConf":
					dst.Server.Conf = kv[1]
				case "serverHost":
					dst.Server.Host = kv[1]
				case "serverPort":
					dst.Server.Port = kv[1]
				case "serverTimeout":
					timeout, err := strconv.ParseInt(kv[1], 10, 64)
					if err != nil {
						return &arerror.ErrParseDocDecl{Name: kv[0], Value: kv[1], Err: arerror.ErrParseDocTimeoudDecl}
					}

					dst.Server.Timeout = timeout
				case "namespace":
					switch StructNameType(nodeName) {
					case Fields:
						fallthrough
					case ProcFields:
						if len(kv[1]) == 0 {
							return &arerror.ErrParseDocDecl{Name: kv[0], Value: kv[1], Err: arerror.ErrParseDocNamespaceDecl}
						}

						dst.Namespace.ObjectName = kv[1]
					default:
						return &arerror.ErrParseDocDecl{Name: kv[0], Value: kv[1], Err: arerror.ErrParseDocNamespaceDecl}
					}
				case "backend":
					dst.Backends = strings.Split(kv[1], ",")
				default:
					retErr := arerror.ErrParseDocDecl{Name: kv[0], Err: arerror.ErrUnknown}
					if len(kv) > 1 {
						retErr.Value = kv[1]
					}

					return &retErr
				}
			}
		}
	}

	return nil
}
