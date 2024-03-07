package arerror

import "errors"

var ErrGeneratorBackendUnknown = errors.New("backend unknown")
var ErrGeneratorBackendNotImplemented = errors.New("backend not implemented")
var ErrGeneragorGetTmplLine = errors.New("can't get error lines")
var ErrGeneragorEmptyTmplLine = errors.New("tmpl lines not set")
var ErrGeneragorErrorLineNotFound = errors.New("template lines not found in error")
var ErrGeneratorTemplateUnkhown = errors.New("template unknown")

// Описание ошибки генерации
type ErrGeneratorPkg struct {
	Name string
	Err  error
}

func (e *ErrGeneratorPkg) Error() string {
	return ErrorBase(e)
}

// Описание ошибки записи в файл результата генерации
type ErrGeneratorFile struct {
	Name     string
	Filename string
	Backend  string
	Err      error
}

func (e *ErrGeneratorFile) Error() string {
	return ErrorBase(e)
}

// Описание ошибки фаз генерации
type ErrGeneratorPhases struct {
	Name      string
	Backend   string
	Phase     string
	TmplLines string
	Err       error
}

func (e *ErrGeneratorPhases) Error() string {
	return ErrorBase(e)
}
