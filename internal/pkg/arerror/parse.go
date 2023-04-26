package arerror

import "errors"

var ErrTypeNotBool = errors.New("type not bool")
var ErrTypeNotSlice = errors.New("type not slice")
var ErrUnknown = errors.New("unknown entity")
var ErrRedefined = errors.New("entity redefined")
var ErrNameDeclaration = errors.New("error name declaration")
var ErrInvalidParams = errors.New("invalid params")
var ErrDuplicate = errors.New("duplicate")
var ErrFieldNotExist = errors.New("field not exists")
var ErrIndexNotExist = errors.New("index not exists")
var ErrParseNodeNameUnknown = errors.New("unknown node name")
var ErrParseNodeNameInvalid = errors.New("invalid struct name")
var ErrParseFuncDeclNotSupported = errors.New("func declaration not implemented")

// Описание ошибки парсинга
type ErrParseGenDecl struct {
	Name string
	Err  error
}

func (e *ErrParseGenDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки парсинга типов
type ErrParseGenTypeDecl struct {
	Name string
	Type interface{} `format:"%T"`
	Err  error
}

func (e *ErrParseGenTypeDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки парсинга комментариев
type ErrParseDocDecl struct {
	Name  string
	Value string
	Err   error
}

func (e *ErrParseDocDecl) Error() string {
	return ErrorBase(e)
}

var ErrParseConst = errors.New("constant declaration not implemented")
var ErrParseVar = errors.New("variable declaration not implemented")
var ErrParseCastImportType = errors.New("error cast type to TypeImport")
var ErrParseCastSpecType = errors.New("error cast type to TypeSpec")
var ErrGetImportName = errors.New("error get import name")
var ErrImportDeclaration = errors.New("import name declaration invalid")
var ErrParseTagSplitAbsent = errors.New("tag is absent")
var ErrParseTagSplitEmpty = errors.New("tag is empty")
var ErrParseTagInvalidFormat = errors.New("invalid tag format")
var ErrParseTagValueInvalid = errors.New("invalid value format")
var ErrParseTagUnknown = errors.New("unknown tag")
var ErrParseTagNoValue = errors.New("tag value required")
var ErrParseTagWithValue = errors.New("wrong tag. Flag can't has value")

var ErrParseDocEmptyBoxDeclaration = errors.New("empty declaration box params in doc")
var ErrParseDocTimeoudDecl = errors.New("invalid timeout declaration")
var ErrParseDocNamespaceDecl = errors.New("invalid namespace declaration")

// Описание ошибки парсинга поля
type ErrParseTypeFieldStructDecl struct {
	Name      string
	FieldType string
	Err       error
}

func (e *ErrParseTypeFieldStructDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки парсинга тегов
type ErrParseTagDecl struct {
	Name string
	Err  error
}

func (e *ErrParseTagDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошики парсинга тегов связанных сущностей
type ErrParseTypeFieldObjectTagDecl struct {
	Name     string
	TagName  string
	TagValue string
	Err      error
}

func (e *ErrParseTypeFieldObjectTagDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки парсинга полей сущности
type ErrParseTypeFieldDecl struct {
	Name      string
	FieldType string
	Err       error
}

func (e *ErrParseTypeFieldDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки парсинга тегов полей сущности
type ErrParseTypeFieldTagDecl struct {
	Name     string
	TagName  string
	TagValue string
	Err      error
}

func (e *ErrParseTypeFieldTagDecl) Error() string {
	return ErrorBase(e)
}

var ErrParseFieldArrayOfNotByte = errors.New("support only array of byte")
var ErrParseProcFieldArraySlice = errors.New("support only array|slice of byte|string")
var ErrParseFieldArrayNotSlice = errors.New("only array of byte not a slice")
var ErrParseFieldBinary = errors.New("binary format not implemented")
var ErrParseFieldMutatorInvalid = errors.New("invalid mutator")
var ErrParseFieldSizeInvalid = errors.New("error parse size")
var ErrParseFieldNameInvalid = errors.New("invalid declaration name")

// Описание ошибки парсинга флагов поля сущности
type ErrParseFlagTagDecl struct {
	Name     string
	TagName  string
	TagValue string
	Err      error
}

func (e *ErrParseFlagTagDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки парсинга флагов
type ErrParseFlagDecl struct {
	Name string
	Err  error
}

func (e *ErrParseFlagDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки парсинга импортов
type ErrParseImportDecl struct {
	Path string
	Name string
	Err  error
}

func (e *ErrParseImportDecl) Error() string {
	return ErrorBase(e)
}

var ErrParseImportNotFound = errors.New("import not found")

// Описание ошибки парсинга индексов
type ErrParseTypeIndexDecl struct {
	IndexType string
	Name      string
	Err       error
}

func (e *ErrParseTypeIndexDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки парсинга тегов индекса
type ErrParseTypeIndexTagDecl struct {
	IndexType string
	Name      string
	TagName   string
	TagValue  string
	Err       error
}

func (e *ErrParseTypeIndexTagDecl) Error() string {
	return ErrorBase(e)
}

var ErrParseIndexInvalidType = errors.New("invalid type of index")
var ErrParseIndexFieldnumRequired = errors.New("fieldnum required and must be more than 0")
var ErrParseIndexFieldnumToBig = errors.New("fieldnum greater than fields")
var ErrParseIndexFieldnumEqual = errors.New("fieldnum equivalent with fields. duplicate index")

// Описание ошибки парсинга сериализаторов
type ErrParseSerializerDecl struct {
	Name string
	Err  error
}

func (e *ErrParseSerializerDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки парсинга тегов сериализатора
type ErrParseSerializerTagDecl struct {
	Name     string
	TagName  string
	TagValue string
	Err      error
}

func (e *ErrParseSerializerTagDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки парсинга типов сериализатора
type ErrParseSerializerTypeDecl struct {
	Name           string
	SerializerType interface{} `format:"%T"`
	Err            error
}

func (e *ErrParseSerializerTypeDecl) Error() string {
	return ErrorBase(e)
}

var ErrParseSerializerAddInternalImport = errors.New("error add internal serializer to import")

// Описание ошибки парсинга структур
type ErrParseTypeStructDecl struct {
	Name string
	Err  error
}

func (e *ErrParseTypeStructDecl) Error() string {
	return ErrorBase(e)
}

var ErrParseStructureEmpty = errors.New("empty structure")

// Описание ошибки парсинга тегов триггеров
type ErrParseTriggerTagDecl struct {
	Name     string
	TagName  string
	TagValue string
	Err      error
}

func (e *ErrParseTriggerTagDecl) Error() string {
	return ErrorBase(e)
}

// Описание ошибки парсинга триггера
type ErrParseTriggerDecl struct {
	Name string
	Err  error
}

func (e *ErrParseTriggerDecl) Error() string {
	return ErrorBase(e)
}

var ErrParseTriggerPackageNotDefined = errors.New("package not defined")
