package parser

import (
	"go/ast"
	"strings"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
)

func ParseImport(dst *ds.RecordPackage, importSpec *ast.ImportSpec) error {
	var pkg string

	path := strings.Trim(importSpec.Path.Value, `"`)

	if importSpec.Name != nil {
		pkg = importSpec.Name.Name
	}

	if _, err := dst.AddImport(path, pkg); err != nil {
		return &arerror.ErrParseImportDecl{Name: pkg, Err: err}
	}

	return nil
}
