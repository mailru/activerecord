package checker

import (
	"log"
	"strconv"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/internal/pkg/ds"
	"github.com/mailru/activerecord/pkg/octopus"
)

// Checker структура описывающая checker
type Checker struct {
	files map[string]*ds.RecordPackage
}

// Init конструктор checker-а
func Init(files map[string]*ds.RecordPackage) *Checker {
	checker := Checker{
		files: files,
	}

	return &checker
}

// checkBackend проверка на указание бекенда
// В данный момент поддерживается один и толлько один бекенд
func checkBackend(cl *ds.RecordPackage) error {
	if len(cl.Backends) == 0 {
		return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Err: arerror.ErrCheckBackendEmpty}
	}

	if len(cl.Backends) > 1 {
		return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Err: arerror.ErrCheckPkgBackendToMatch}
	}

	return nil
}

// checkLinkedObject проверка существования сущностей на которые ссылаются другие сущности
func checkLinkedObject(cl *ds.RecordPackage, linkedObjects map[string]string) error {
	for _, fobj := range cl.FieldsObjectMap {
		if _, ok := linkedObjects[fobj.ObjectName]; !ok {
			return &arerror.ErrCheckPackageLinkedDecl{Pkg: cl.Namespace.PackageName, Object: fobj.ObjectName, Err: arerror.ErrCheckObjectNotFound}
		}
	}

	return nil
}

// checkNamespace проверка правильного описания неймспейса у сущности
func checkNamespace(ns ds.NamespaceDeclaration) error {
	if ns.PackageName == "" || ns.PublicName == "" {
		return &arerror.ErrCheckPackageNamespaceDecl{Pkg: ns.PackageName, Name: ns.PublicName, Err: arerror.ErrCheckEmptyNamespace}
	}

	return nil
}

// checkFields функция проверки правильности описания полей структуры
// - указан допустимый тип полей
// - описаны все необходимые сериализаторы для полей с сериализацией
// - поля с мутаторами не могут быть праймари клчом
// - поля с мутаторами не могут быть сериализованными
// - поля с мутаторами не могут являться ссылками на другие сущности
// - сериализуемые поля не могут быть ссылками на другие сущности
// - есть первичный ключ
// - имена сущностей на которые ссылаемся на могут пересекаться с именами полей
//
//nolint:gocognit,gocyclo
func checkFields(cl *ds.RecordPackage) error {
	if len(cl.Fields) > 0 && len(cl.ProcOutFields) > 0 {
		return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Err: arerror.ErrCheckFieldsManyDecl}
	}

	if !cl.ProcOutFields.Validate() {
		return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Err: arerror.ErrCheckFieldsOrderDecl}
	}

	primaryFound := false

	octopusAvailFormat := map[octopus.Format]bool{}
	for _, form := range octopus.AllFormat {
		octopusAvailFormat[form] = true
	}

	for _, fld := range cl.Fields {
		if _, ex := octopusAvailFormat[fld.Format]; !ex {
			return &arerror.ErrCheckPackageFieldDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldInvalidFormat}
		}

		if len(fld.Serializer) > 0 {
			if _, ex := cl.SerializerMap[fld.Serializer[0]]; len(cl.SerializerMap) == 0 || !ex {
				return &arerror.ErrCheckPackageFieldDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldSerializerNotFound}
			}

		}

		customMutCnt := 0
		if len(fld.Mutators) > 0 {
			fieldMutatorsChecker := ds.GetFieldMutatorsChecker()

			for _, m := range fld.Mutators {
				_, ex := fieldMutatorsChecker[m]

				md, ok := cl.MutatorMap[m]
				if ok {
					customMutCnt++
					if customMutCnt > 1 {
						return &arerror.ErrCheckPackageFieldMutatorDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Mutator: m, Err: arerror.ErrParseFieldMutatorInvalid}
					}
				}

				if !ok && !ex {
					return &arerror.ErrCheckPackageFieldMutatorDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Mutator: m, Err: arerror.ErrParseFieldMutatorInvalid}
				}

				if len(md.PartialFields) > 0 && len(fld.Serializer) == 0 {
					return &arerror.ErrCheckPackageFieldMutatorDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Mutator: m, Err: arerror.ErrParseFieldMutatorTypeHasNotSerializer}
				}
			}

			if fld.PrimaryKey {
				return &arerror.ErrCheckPackageFieldMutatorDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Mutator: string(fld.Mutators[0]), Err: arerror.ErrCheckFieldMutatorConflictPK}
			}

			if fld.ObjectLink != "" {
				return &arerror.ErrCheckPackageFieldMutatorDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Mutator: string(fld.Mutators[0]), Err: arerror.ErrCheckFieldMutatorConflictObject}
			}
		}

		if len(fld.Serializer) > 0 && fld.ObjectLink != "" {
			return &arerror.ErrCheckPackageFieldMutatorDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldSerializerConflictObject}
		}

		if fo, ex := cl.FieldsObjectMap[fld.Name]; ex {
			return &arerror.ErrParseTypeFieldStructDecl{Name: fo.Name, Err: arerror.ErrRedefined}
		}

		if fld.PrimaryKey {
			primaryFound = true
		}
	}

	for _, fld := range cl.ProcOutFields.List() {
		if _, ex := octopusAvailFormat[fld.Format]; !ex {
			return &arerror.ErrCheckPackageFieldDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldInvalidFormat}
		}

		if len(fld.Serializer) > 0 {
			if _, ex := cl.SerializerMap[fld.Serializer[0]]; !ex {
				return &arerror.ErrCheckPackageFieldDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldSerializerNotFound}
			}
		}

		if fld.Type == 0 {
			return &arerror.ErrCheckPackageFieldDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldTypeNotFound}
		}
	}

	octopusProcAvailFormat := map[octopus.Format]bool{}
	for _, form := range octopus.AllProcFormat {
		octopusProcAvailFormat[form] = true
	}

	for _, fld := range cl.ProcInFields {
		if _, ex := octopusProcAvailFormat[fld.Format]; !ex {
			return &arerror.ErrCheckPackageFieldDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldInvalidFormat}
		}

		if len(fld.Serializer) > 0 {
			if _, ex := cl.SerializerMap[fld.Serializer[0]]; !ex {
				return &arerror.ErrCheckPackageFieldDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldSerializerNotFound}
			}
		}

		if fld.Type == 0 {
			return &arerror.ErrCheckPackageFieldDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldTypeNotFound}
		}
	}

	if len(cl.Fields) == 0 && len(cl.ProcOutFields) == 0 {
		return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Err: arerror.ErrCheckFieldsEmpty}
	}

	if len(cl.Fields) > 0 && !primaryFound {
		return &arerror.ErrCheckPackageIndexDecl{Pkg: cl.Namespace.PackageName, Index: "primary", Err: arerror.ErrIndexNotExist}
	}

	return nil
}

// Check основная функция, которая запускает процесс проверки
// Должна вызываться только после окончания процесса парсинга всех деклараций
func Check(files map[string]*ds.RecordPackage, linkedObjects map[string]string) error {
	for _, cl := range files {
		if err := checkBackend(cl); err != nil {
			return err
		}

		if err := checkNamespace(cl.Namespace); err != nil {
			return err
		}

		if err := checkLinkedObject(cl, linkedObjects); err != nil {
			return err
		}

		if err := checkFields(cl); err != nil {
			return err
		}

		// Бекендозависимые проверки
		for _, backend := range cl.Backends {
			switch backend {
			case "tarantool16":
				fallthrough
			case "tarantool2":
				if err := checkTarantool(cl); err != nil {
					return err
				}
			case "tarantool15":
				fallthrough
			case "octopus":
				if err := checkOctopus(cl); err != nil {
					return err
				}
			default:
				return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Backend: backend, Err: arerror.ErrCheckBackendUnknown}
			}
		}
	}

	return nil
}

//nolint:gocognit,gocyclo
func checkOctopus(cl *ds.RecordPackage) error {
	if cl.Server.Host == "" && cl.Server.Conf == "" {
		return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Err: arerror.ErrCheckServerEmpty}
	}

	if cl.Server.Host == "" && cl.Server.Port != "" {
		return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Err: arerror.ErrCheckPortEmpty}
	}

	if cl.Server.Host != "" && cl.Server.Conf != "" {
		return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Err: arerror.ErrCheckServerConflict}
	}

	for _, fl := range cl.Fields {
		if (fl.Format == "string" || fl.Format == "[]byte") && fl.Size == 0 {
			log.Printf("Warn: field `%s` declaration. Field with type string or []byte not contain size.", fl.Name)
		}
	}

	for _, ind := range cl.Indexes {
		if len(ind.Fields) == 0 {
			return &arerror.ErrCheckPackageIndexDecl{Pkg: cl.Namespace.PackageName, Index: ind.Name, Err: arerror.ErrCheckFieldIndexEmpty}
		}
	}

	if len(cl.Fields) > 0 {
		_, err := strconv.ParseInt(cl.Namespace.ObjectName, 10, 64)
		if err != nil {
			return &arerror.ErrCheckPackageNamespaceDecl{Pkg: cl.Namespace.PackageName, Name: cl.Namespace.ObjectName, Err: arerror.ErrCheckFieldInvalidFormat}
		}
	}

	octopusProcAvailFormat := map[octopus.Format]bool{}
	for _, form := range []octopus.Format{octopus.String, octopus.StringArray, octopus.ByteArray} {
		octopusProcAvailFormat[form] = true
	}

	for _, fld := range cl.ProcInFields {
		if _, ex := octopusProcAvailFormat[fld.Format]; !ex {
			return &arerror.ErrCheckPackageFieldDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldInvalidFormat}
		}

		if fld.Format != octopus.String && len(fld.Serializer) == 0 {
			return &arerror.ErrCheckPackageFieldDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldSerializerNotFound}
		}
	}

	return nil
}

//nolint:gocognit,gocyclo
func checkTarantool(cl *ds.RecordPackage) error {
	if cl.Server.Host == "" && cl.Server.Conf == "" {
		return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Err: arerror.ErrCheckServerEmpty}
	}

	if cl.Server.Host == "" && cl.Server.Port != "" {
		return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Err: arerror.ErrCheckPortEmpty}
	}

	if cl.Server.Host != "" && cl.Server.Conf != "" {
		return &arerror.ErrCheckPackageDecl{Pkg: cl.Namespace.PackageName, Err: arerror.ErrCheckServerConflict}
	}

	for _, fl := range cl.Fields {
		if (fl.Format == "string" || fl.Format == "[]byte") && fl.Size == 0 {
			log.Printf("Warn: field `%s` declaration. Field with type string or []byte not contain size.", fl.Name)
		}
	}

	for _, ind := range cl.Indexes {
		if len(ind.Fields) == 0 {
			return &arerror.ErrCheckPackageIndexDecl{Pkg: cl.Namespace.PackageName, Index: ind.Name, Err: arerror.ErrCheckFieldIndexEmpty}
		}
	}

	for _, fld := range cl.ProcInFields {
		if fld.Format != octopus.String && len(fld.Serializer) == 0 {
			return &arerror.ErrCheckPackageFieldDecl{Pkg: cl.Namespace.PackageName, Field: fld.Name, Err: arerror.ErrCheckFieldSerializerNotFound}
		}
	}

	return nil
}
