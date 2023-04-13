package arerror

import (
	"errors"
)

var ErrCheckBackendEmpty = errors.New("backend empty")
var ErrCheckBackendUnknown = errors.New("backend unknown")
var ErrCheckEmptyNamespace = errors.New("empty namespace")
var ErrCheckPkgBackendToMatch = errors.New("many backends for one class not supported yet")
var ErrCheckFieldSerializerNotFound = errors.New("serializer not found")
var ErrCheckFieldInvalidFormat = errors.New("invalid format")
var ErrCheckFieldMutatorConflictPK = errors.New("conflict mutators with primary_key")
var ErrCheckFieldMutatorConflictSerializer = errors.New("conflict mutators with serializer")
var ErrCheckFieldMutatorConflictObject = errors.New("conflict mutators with object link")
var ErrCheckFieldSerializerConflictObject = errors.New("conflict serializer with object link")
var ErrCheckServerEmpty = errors.New("serverConf and serverHost is empty")
var ErrCheckPortEmpty = errors.New("serverPort is empty")
var ErrCheckServerConflict = errors.New("conflict ServerHost and serverConf params")
var ErrCheckFieldIndexEmpty = errors.New("field for index is empty")
var ErrCheckObjectNotFound = errors.New("linked object not found")

// Описание ошибки декларации пакета
type ErrCheckPackageDecl struct {
	Pkg     string
	Backend string
	Err     error
}

func (e *ErrCheckPackageDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки декларации неймспейса
type ErrCheckPackageNamespaceDecl struct {
	Pkg  string
	Name string
	Err  error
}

func (e *ErrCheckPackageNamespaceDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки декларации связанных сущностей
type ErrCheckPackageLinkedDecl struct {
	Pkg    string
	Object string
	Err    error
}

func (e *ErrCheckPackageLinkedDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки декларации полей
type ErrCheckPackageFieldDecl struct {
	Pkg   string
	Field string
	Err   error
}

func (e *ErrCheckPackageFieldDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки декларации мутаторов
type ErrCheckPackageFieldMutatorDecl struct {
	Pkg     string
	Field   string
	Mutator string
	Err     error
}

func (e *ErrCheckPackageFieldMutatorDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки декларации индексов
type ErrCheckPackageIndexDecl struct {
	Pkg   string
	Index string
	Err   error
}

func (e *ErrCheckPackageIndexDecl) Error() string {
	return ErrorBase(e)
}
