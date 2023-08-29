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

//nolint:gocognit
func searchStruct(structName string, pkg *ast.Package) (*ast.StructType, error) {
	for _, f := range pkg.Files {
		for _, decl := range f.Decls {
			switch gen := decl.(type) {
			case *ast.GenDecl:
				if gen.Tok != token.TYPE {
					continue
				}

				for _, spec := range gen.Specs {
					currType, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}

					switch curr := currType.Type.(type) {
					case *ast.StructType:
						if currType.Name.Name != structName {
							continue
						}

						if curr.Fields == nil || len(curr.Fields.List) == 0 {
							return nil, &arerror.ErrParseTypeStructDecl{Name: currType.Name.Name, Err: arerror.ErrParseStructureEmpty}
						}

						return curr, nil
					default:
						continue
					}
				}
			default:
				continue
			}
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

	s, err := searchStruct(name, pkgs[pkgName])
	if err != nil {
		return nil, fmt.Errorf("error parse package `%s`: %w", pkgName, err)
	}

	if s == nil {
		return nil, fmt.Errorf("`%s` not found on path `%s`", name, path)
	}

	pd := make([]ds.PartialFieldDeclaration, 0, len(s.Fields.List))

	for _, field := range s.Fields.List {
		if len(field.Names) == 0 {
			continue
		}

		t, err := ParseFieldType(dst, name, field.Type)
		if err != nil {
			return nil, &arerror.ErrParseTypeFieldStructDecl{Name: name, FieldType: field.Names[0].Name, Err: err}
		}

		pd = append(pd, ds.PartialFieldDeclaration{
			Parent: name,
			Name:   field.Names[0].Name,
			Type:   t,
		})
	}

	return pd, nil
}
