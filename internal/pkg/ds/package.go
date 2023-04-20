package ds

import (
	"errors"
	"regexp"

	"github.com/mailru/activerecord/internal/pkg/arerror"
)

// PkgNameRx регулярное выражение вырезающее имя импортируемого пакета
var PkgNameRx = regexp.MustCompile(`([^/"]+)"?$`)

// Функция для получения имени импортируемого пакета из пути
func getImportName(path string) (string, error) {
	matched := PkgNameRx.FindStringSubmatch(path)
	if len(matched) < 2 {
		return "", &arerror.ErrParseImportDecl{Name: path, Err: arerror.ErrNameDeclaration}
	}

	return matched[1], nil
}

// Добавление нового поля в результирующий пакет
func (rc *RecordPackage) AddField(f FieldDeclaration) error {
	// Проверка на то, что имя не дублируется
	if _, ex := rc.FieldsMap[f.Name]; ex {
		return &arerror.ErrParseTypeFieldDecl{Name: f.Name, FieldType: string(f.Format), Err: arerror.ErrRedefined}
	}

	// добавляем поле и не забываем про обратны индекс
	rc.FieldsMap[f.Name] = len(rc.Fields)
	rc.Fields = append(rc.Fields, f)

	return nil
}

// Добавление нового параметра процедуры в результирующий пакет
func (rc *RecordPackage) AddProcField(f ProcFieldDeclaration) error {
	// Проверка на то, что имя не дублируется
	if _, ex := rc.ProcFieldsMap[f.Name]; ex {
		return &arerror.ErrParseTypeFieldDecl{Name: f.Name, FieldType: string(f.Format), Err: arerror.ErrRedefined}
	}

	// добавляем поле и не забываем про обратны индекс
	rc.ProcFieldsMap[f.Name] = len(rc.ProcFields)
	rc.ProcFields = append(rc.ProcFields, f)

	return nil
}

// Добавление нового ссылочного поля
func (rc *RecordPackage) AddFieldObject(fo FieldObject) error {
	if _, ex := rc.FieldsObjectMap[fo.Name]; ex {
		return &arerror.ErrParseTypeFieldStructDecl{Name: fo.Name, Err: arerror.ErrRedefined}
	}

	rc.FieldsObjectMap[fo.Name] = fo

	return nil
}

// Добавление индекса
func (rc *RecordPackage) AddIndex(ind IndexDeclaration) error {
	if _, ex := rc.IndexMap[ind.Name]; ex {
		return &arerror.ErrParseTypeIndexDecl{IndexType: "index", Name: ind.Name, Err: arerror.ErrRedefined}
	}

	if (ind.Primary || ind.Unique) && ind.Selector == "" {
		ind.Selector = "SelectBy" + ind.Name
	}

	if _, ex := rc.SelectorMap[ind.Selector]; ex {
		return &arerror.ErrParseTypeIndexDecl{IndexType: "index", Name: ind.Name, Err: arerror.ErrRedefined}
	}

	if ind.Primary {
		ind.Unique = true
		for fInd := range ind.Fields {
			rc.Fields[ind.Fields[fInd]].PrimaryKey = true
		}
	}

	if !ind.Partial && ind.Num == 0 && len(rc.Indexes) > 0 {
		ind.Num = rc.Indexes[len(rc.Indexes)-1].Num + 1
	}

	rc.IndexMap[ind.Name] = len(rc.Indexes)
	rc.SelectorMap[ind.Selector] = len(rc.Indexes)
	rc.Indexes = append(rc.Indexes, ind)

	return nil
}

func (rc *RecordPackage) AddImport(path string, reqImportName ...string) (ImportDeclaration, error) {
	if len(reqImportName) > 1 {
		return ImportDeclaration{}, &arerror.ErrParseImportDecl{Path: path, Err: arerror.ErrInvalidParams}
	}

	resultImportName := ""

	searchImportName, err := getImportName(path)
	if err != nil {
		return ImportDeclaration{}, &arerror.ErrParseImportDecl{Path: path, Name: "UNKNOWN", Err: arerror.ErrGetImportName}
	}

	if len(reqImportName) == 1 && reqImportName[0] != "" {
		if reqImportName[0] != searchImportName {
			resultImportName = reqImportName[0]
		}

		searchImportName = reqImportName[0]
	}

	if imp, ex := rc.ImportMap[path]; ex {
		if rc.Imports[imp].ImportName == resultImportName {
			return rc.Imports[imp], nil
		}
	}

	if imp, ex := rc.ImportPkgMap[searchImportName]; ex {
		if rc.Imports[imp].Path != path {
			return ImportDeclaration{}, &arerror.ErrParseImportDecl{Path: path, Name: searchImportName, Err: arerror.ErrDuplicate}
		}

		return rc.Imports[imp], nil
	}

	newImport := ImportDeclaration{
		Path:       path,
		ImportName: resultImportName,
	}

	rc.ImportMap[newImport.Path] = len(rc.Imports)
	rc.ImportPkgMap[searchImportName] = len(rc.Imports)
	rc.Imports = append(rc.Imports, newImport)

	return newImport, nil
}

func (rc *RecordPackage) FindImport(path string) (ImportDeclaration, error) {
	if impNum, ex := rc.ImportMap[path]; ex {
		return rc.Imports[impNum], nil
	}

	return ImportDeclaration{}, &arerror.ErrParseImportDecl{Path: path, Err: arerror.ErrParseImportNotFound}
}

func (rc *RecordPackage) FindImportByPkg(pkg string) (*ImportDeclaration, error) {
	if impNum, ex := rc.ImportPkgMap[pkg]; ex {
		return &rc.Imports[impNum], nil
	}

	return nil, &arerror.ErrParseImportDecl{Name: pkg, Err: arerror.ErrParseImportNotFound}
}

func (rc *RecordPackage) FindOrAddImport(path, importName string) (ImportDeclaration, error) {
	imp, err := rc.FindImport(path)
	if err != nil {
		var impErr *arerror.ErrParseImportDecl

		if errors.As(err, &impErr) && errors.Is(impErr.Err, arerror.ErrParseImportNotFound) {
			imp, err = rc.AddImport(path, importName)
			if err != nil {
				return ImportDeclaration{}, err
			}

			return imp, nil
		}

		return ImportDeclaration{}, err
	}

	return imp, nil
}

func (rc *RecordPackage) AddTrigger(t TriggerDeclaration) error {
	if _, ex := rc.TriggerMap[t.Name]; ex {
		return &arerror.ErrParseTriggerDecl{Name: t.Name, Err: arerror.ErrRedefined}
	}

	rc.TriggerMap[t.Name] = t

	return nil
}

func (rc *RecordPackage) AddSerializer(s SerializerDeclaration) error {
	if _, ex := rc.SerializerMap[s.Name]; ex {
		return &arerror.ErrParseSerializerDecl{Name: s.Name, Err: arerror.ErrRedefined}
	}

	rc.SerializerMap[s.Name] = s

	return nil
}

func (rc *RecordPackage) AddFlag(f FlagDeclaration) error {
	if _, ok := rc.FlagMap[f.Name]; ok {
		return &arerror.ErrParseFlagDecl{Name: f.Name, Err: arerror.ErrDuplicate}
	}

	rc.FlagMap[f.Name] = f

	return nil
}
