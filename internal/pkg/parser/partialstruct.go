package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

func parseStructFields(dst *ds.RecordPackage, gen *ast.GenDecl, name, pkgName string) ([]ds.PartialFieldDeclaration, error) {
	for _, spec := range gen.Specs {
		currType, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		switch curr := currType.Type.(type) {
		case *ast.StructType:
			if currType.Name.Name != name {
				continue
			}

			if curr.Fields == nil {
				return nil, &arerror.ErrParseTypeStructDecl{Name: currType.Name.Name, Err: arerror.ErrParseStructureEmpty}
			}

			partialFields := make([]ds.PartialFieldDeclaration, 0, len(curr.Fields.List))

			for _, field := range curr.Fields.List {
				if len(field.Names) == 0 {
					continue
				}

				t, err := ParseFieldType(dst, name, pkgName, field.Type)
				if err != nil {
					return nil, &arerror.ErrParseTypeFieldStructDecl{Name: name, FieldType: field.Names[0].Name, Err: err}
				}

				field := ds.PartialFieldDeclaration{
					Name: field.Names[0].Name,
					Type: t,
				}

				partialFields = append(partialFields, field)
			}

			return partialFields, nil
		}
	}

	return nil, nil
}

func ParsePartialStructFields(dst *ds.RecordPackage, name, pkgName, path string) ([]ds.PartialFieldDeclaration, error) {
	relPath, err := filepath.Rel(dst.Namespace.ModuleName, path)
	if err != nil {
		return nil, fmt.Errorf("can't extract rel path of `%s` for module `%s`: %w", path, dst.Namespace.ModuleName, err)
	}

	pkgs, err := parser.ParseDir(token.NewFileSet(), relPath, nil, parser.DeclarationErrors)
	if err != nil {
		return nil, fmt.Errorf("error parse file `%s`: %w", path, err)
	}

	files := make(map[string]*ast.File)
	for _, f := range pkgs[pkgName].Files {
		for name, object := range f.Scope.Objects {
			if object.Kind == ast.Typ {
				files[name] = f
			}
		}
	}

	file, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("can't find struct `%s` in package `%s`: %w", name, pkgName, err)
	}

	importPkg := ds.NewImportPackage()
	for _, spec := range file.Imports {
		if err = ParseImport(&importPkg, spec); err != nil {
			return nil, fmt.Errorf("can't parse import from package file `%s`: %w", file.Name, err)
		}
	}

	pkgDecl := ds.LinkedPackageDeclaration{
		Types:  make(map[string]struct{}),
		Import: importPkg,
	}

	for t := range files {
		pkgDecl.Types[t] = struct{}{}
	}

	dst.LinkedStructsMap[pkgName] = pkgDecl

	for _, decl := range file.Decls {
		switch gen := decl.(type) {
		case *ast.GenDecl:
			if gen.Tok != token.TYPE {
				continue
			}
			partialFields, genErr := parseStructFields(dst, gen, name, pkgName)
			if genErr != nil {
				return nil, &arerror.ErrParseGenDecl{Name: pkgName, Err: fmt.Errorf("error parse struct `%s` in package `%s`: %w", name, pkgName, genErr)}
			}

			if len(partialFields) == 0 {
				continue
			}

			return partialFields, nil
		}
	}

	return nil, nil
}

//nolint:gocognit
func ParseFieldType(dst *ds.RecordPackage, name, pName string, t interface{}) (string, error) {
	switch tv := t.(type) {
	case *ast.Ident:
		v := tv.String()

		if ls, ok := dst.LinkedStructsMap[pName]; ok {
			if _, ok := ls.Types[v]; ok {
				return pName + "." + v, nil
			}

			// если импорта нет, то это простой тип
			imp, err := ls.Import.FindImportByPkg(v)
			if err != nil {
				return v, nil
			}

			_, _ = dst.FindOrAddImport(imp.Path, imp.ImportName)
		}

		return v, nil
	case *ast.ArrayType:
		var err error

		len := ""
		if tv.Len != nil {
			len, err = ParseFieldType(dst, name, "", tv.Len)
			if err != nil {
				return "", err
			}
		}

		t, err := ParseFieldType(dst, name, pName, tv.Elt)
		if err != nil {
			return "", err
		}

		return "[" + len + "]" + t, nil
	case *ast.InterfaceType:
		return "interface{}", nil
	case *ast.StarExpr:
		t, err := ParseFieldType(dst, name, pName, tv.X)
		if err != nil {
			return "", err
		}

		return "*" + t, nil
	case *ast.MapType:
		k, err := ParseFieldType(dst, name, pName, tv.Key)
		if err != nil {
			return "", nil
		}

		v, err := ParseFieldType(dst, name, pName, tv.Value)
		if err != nil {
			return "", nil
		}

		return "map[" + k + "]" + v, nil
	case *ast.SelectorExpr:
		pkgName, err := ParseFieldType(dst, name, pName, tv.X)
		if err != nil {
			return "", err
		}

		imp, err := dst.FindImportByPkg(pkgName)
		if err != nil {
			return "", &arerror.ErrParseTypeStructDecl{Name: name, Err: err}
		}

		reqImportName := imp.ImportName
		if reqImportName == "" {
			reqImportName = pkgName
		}

		if _, ok := dst.ImportStructFieldsMap[reqImportName+"."+tv.Sel.Name]; !ok {
			fieldDeclarations, err := ParsePartialStructFields(dst, tv.Sel.Name, pkgName, imp.Path)
			if err != nil {
				return "", &arerror.ErrParseTypeStructDecl{Name: name, Err: err}
			}

			dst.ImportStructFieldsMap[reqImportName+"."+tv.Sel.Name] = fieldDeclarations
		}

		return reqImportName + "." + tv.Sel.Name, nil
	default:
		return "", &arerror.ErrParseTypeStructDecl{Name: name, Err: arerror.ErrUnknown}
	}
}
