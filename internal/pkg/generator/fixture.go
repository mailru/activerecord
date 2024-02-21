package generator

import (
	_ "embed"

	"github.com/mailru/activerecord/internal/pkg/ds"
)

type FixturePkgData struct {
	FixturePkg       string
	ARPkg            string
	ARPkgTitle       string
	FieldList        []ds.FieldDeclaration
	FieldMap         map[string]int
	FieldObject      map[string]ds.FieldObject
	ProcInFieldList  []ds.ProcFieldDeclaration
	ProcOutFieldList []ds.ProcFieldDeclaration
	Container        ds.NamespaceDeclaration
	Indexes          []ds.IndexDeclaration
	Serializers      map[string]ds.SerializerDeclaration
	Mutators         map[string]ds.MutatorDeclaration
	Imports          []ds.ImportDeclaration
	AppInfo          string
}
