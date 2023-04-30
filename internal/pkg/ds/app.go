package ds

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mailru/activerecord/internal/pkg/arerror"
	"github.com/mailru/activerecord/pkg/octopus"
)

// Описание приложения. Информация необходимая для разметки артефактов
// Остаётся в сгенерированных файлах, что бы было понятно какой версией сгенерированы файлы
type AppInfo struct {
	appName      string
	version      string
	buildTime    string
	buildOS      string
	buildCommit  string
	generateTime string
}

// Конструктор для AppInfo
func NewAppInfo() *AppInfo {
	return &AppInfo{
		appName:      "argen",
		generateTime: time.Now().Format(time.RFC3339),
	}
}

// Опции для конструктора, используются для модификации полей структуры
func (i *AppInfo) WithVersion(version string) *AppInfo {
	i.version = version
	return i
}

func (i *AppInfo) WithBuildTime(buildTime string) *AppInfo {
	i.buildTime = buildTime
	return i
}

func (i *AppInfo) WithBuildOS(buildOS string) *AppInfo {
	i.buildOS = buildOS
	return i
}

func (i *AppInfo) WithBuildCommit(commit string) *AppInfo {
	i.buildCommit = commit
	return i
}

// Строковое представление версии генератора
func (i *AppInfo) String() string {
	return fmt.Sprintf("%s@%s (Commit: %s)", i.appName, i.version, i.buildCommit)
}

// Структура для описания неймспейса сущности
type NamespaceDeclaration struct {
	ObjectName  string
	PublicName  string
	PackageName string
	ModuleName  string
}

// Структура для описания конфигурации сервера
// Может быть указан путь к конфигурации `Conf` или параметры подключения напрямую
type ServerDeclaration struct {
	Timeout          int64
	Host, Port, Conf string
}

// Структура описывающая отдельную сущность представленную в декларативном файле
type RecordPackage struct {
	Server          ServerDeclaration                // Описание сервера
	Namespace       NamespaceDeclaration             // Описание неймспейса/таблицы
	Fields          []FieldDeclaration               // Описание полей, важна последовательность для некоторых хранилищ
	FieldsMap       map[string]int                   // Обратный индекс от имен к полям
	FieldsObjectMap map[string]FieldObject           // Обратный индекс по имени для ссылок на другие сущности
	Indexes         []IndexDeclaration               // Список индексов, важна последовательность для некоторых хранилищ
	IndexMap        map[string]int                   // Обратный индекс от имён для индексов
	SelectorMap     map[string]int                   // Список селекторов, используется для контроля дублей
	Backends        []string                         // Список бекендов для которых надо сгенерировать пакеты (сейчас допустим один и только один)
	SerializerMap   map[string]SerializerDeclaration // Список сериализаторов используемых в этой сущности
	Imports         []ImportDeclaration              // Список необходимых дополнительных импортов, формируется из директивы import
	ImportMap       map[string]int                   // Обратный индекс от имен по импортам
	ImportPkgMap    map[string]int                   // Обратный индекс от пакетов к импортам
	TriggerMap      map[string]TriggerDeclaration    // Список триггеров используемых в сущности
	FlagMap         map[string]FlagDeclaration       // Список флагов используемых в полях сущности
	ProcInFields    []ProcFieldDeclaration           // Описание входных параметров процедуры, важна последовательность
	ProcOutFields   ProcFieldDeclarations            // Описание выходных параметров процедуры, важна последовательность
	ProcFieldsMap   map[string]int                   // Обратный индекс от имен
}

// Конструктор для RecordPackage, инициализирует ссылочные типы
func NewRecordPackage() *RecordPackage {
	return &RecordPackage{
		Server:          ServerDeclaration{},
		Namespace:       NamespaceDeclaration{},
		Fields:          []FieldDeclaration{},
		FieldsMap:       map[string]int{},
		FieldsObjectMap: map[string]FieldObject{},
		Indexes:         []IndexDeclaration{},
		IndexMap:        map[string]int{},
		SelectorMap:     map[string]int{},
		Imports:         []ImportDeclaration{},
		ImportMap:       map[string]int{},
		ImportPkgMap:    map[string]int{},
		Backends:        []string{},
		SerializerMap:   map[string]SerializerDeclaration{},
		TriggerMap:      map[string]TriggerDeclaration{},
		FlagMap:         map[string]FlagDeclaration{},
		ProcFieldsMap:   map[string]int{},
		ProcOutFields:   map[int]ProcFieldDeclaration{},
	}
}

// Тип и константы для описания направления сортировки индекса
type IndexOrder uint8

const (
	IndexOrderAsc IndexOrder = iota
	IndexOrderDesc
)

// Тип для описания поля внутри индекса (номер поля и направление сортировки)
type IndexField struct {
	IndField int
	Order    IndexOrder
}

// Тип для описания индекса
type IndexDeclaration struct {
	Name      string                // Имя индекса
	Num       uint8                 // Номер индекса в описании спейса
	Selector  string                // Название функции селектора
	Fields    []int                 // Список номеров полей участвующих в индексе (последовательность имеет значение)
	FieldsMap map[string]IndexField // Обратный индекс по именам полей (используется для выявления дублей)
	Primary   bool                  // Признак того, что индекс является первичным ключом
	Unique    bool                  // Признак того, что индекс является уникальным
	Type      string                // Тип индекса, для индексов по одному полю простой тип, для составных индексов собственный тип
	Partial   bool                  // Признак того, что индекс частичный
}

// Serializer Сериализаторы для поля
type Serializer []string

// FieldDeclaration Тип описывающий поле в сущности
type FieldDeclaration struct {
	Name       string         // Название поля
	Format     octopus.Format // формат поля
	PrimaryKey bool           // участвует ли поле в первичном ключе (при изменении таких полей необходимо делать delete + insert вместо update)
	Mutators   []FieldMutator // список мутаторов (атомарных действий на уровне БД)
	Size       int64          // Размер поля, используется только для строковых значений
	Serializer Serializer     // Сериализаторы для поля
	ObjectLink string         // является ли поле ссылкой на другую сущность
}

// Name возвращает имя сериализатора, если он установлен, иначе пустую строку
func (s Serializer) Name() string {
	if len(s) > 0 {
		return s[0]
	}

	return ""
}

// Params Параметры передаваемые при сериализации. Используется, когда на уровне декларирования
// известно, что сериализатор/десериализатор требует дополнительных константных значений
func (s Serializer) Params() string {
	if len(s) > 1 {
		return `"` + strings.Join(s[1:], `", "`) + `", `
	}

	return ""
}

const (
	ProcInputParam  = "input"
	ProcOutputParam = "output"
)

type ProcParameterType uint8

func (p ProcParameterType) String() string {
	switch p {
	case IN:
		return ProcInputParam
	case OUT:
		return ProcOutputParam
	case INOUT:
		return fmt.Sprintf("%v/%v", ProcInputParam, ProcOutputParam)
	default:
		return ""
	}
}

const (
	_     ProcParameterType = iota
	IN                      //тип входного параметра процедуры
	OUT                     //тип выходного параметра процедуры
	INOUT                   //тип одновременно и входного и выходного параметра процедуры
)

// ProcFieldDeclaration Тип описывающий поле процедуры
type ProcFieldDeclaration struct {
	Name       string            // Название поля
	Format     octopus.Format    // формат поля
	Type       ProcParameterType // тип параметра (IN, OUT, INOUT)
	Size       int64             // Размер поля, используется только для строковых значений
	Serializer Serializer        // Сериализатора для поля
	OrderIndex int               // Порядковый номер параметра в сигнатуре вызова процедуры
}

// ProcFieldDeclarations Индекс порядкового значения полей процедуры
type ProcFieldDeclarations map[int]ProcFieldDeclaration

// Add Добавляет декларацию поля процедуры в список.
// Возвращает ошибку, если декларация с таким порядком в [ProcFieldDeclarations] уже существует
func (pfd ProcFieldDeclarations) Add(field ProcFieldDeclaration) error {
	idx := field.OrderIndex

	if _, ok := pfd[idx]; ok {
		return arerror.ErrProcFieldDuplicateOrderIndex
	}

	pfd[idx] = field

	return nil
}

// List список деклараций процедуры в описанном порядке описания
func (pfd ProcFieldDeclarations) List() []ProcFieldDeclaration {
	out := make([]ProcFieldDeclaration, len(pfd))
	for i := range pfd {
		out[i] = pfd[i]
	}

	return out
}

// Validate проверяет корректность декларируемых значений порядкового номера полей процедуры
func (pfd ProcFieldDeclarations) Validate() bool {
	if len(pfd) == 0 {
		return true
	}

	var maxIdx int
	for idx := range pfd {
		if idx > maxIdx {
			maxIdx = idx
		}
	}

	if maxIdx >= len(pfd) {
		return false
	}

	return true
}

// Тип и константы описывающие мутаторы для поля
type FieldMutator string

const (
	IncMutator      FieldMutator = "inc"       // инкремент (только для числовых типов)
	DecMutator      FieldMutator = "dec"       // декремент (только для числовых типов)
	SetBitMutator   FieldMutator = "set_bit"   // установка бита (только для целочисленных типов)
	ClearBitMutator FieldMutator = "clear_bit" // снятие бита (только для целочисленных типов)
	AndMutator      FieldMutator = "and"       // дизъюнкция (только для целочисленных типов)
	OrMutator       FieldMutator = "or"        // конъюнкция (только для целочисленных типов)
	XorMutator      FieldMutator = "xor"       // xor (только для целочисленных типов)
)

var FieldMutators = [...]FieldMutator{
	IncMutator,
	DecMutator,
	SetBitMutator,
	ClearBitMutator,
	AndMutator,
	OrMutator,
	XorMutator}

var fieldMutatorsChecker = map[string]FieldMutator{}
var FieldMutatorsCheckerOnce sync.Once

func GetFieldMutatorsChecker() map[string]FieldMutator {
	FieldMutatorsCheckerOnce.Do(func() {
		for _, f := range FieldMutators {
			fieldMutatorsChecker[string(f)] = f
		}
	})

	return fieldMutatorsChecker
}

// Структура для описания ссылочных полей (когда значение одного из полей является ключом для для другой сущности)
type FieldObject struct {
	Name       string // Имя
	Key        string // Название поля во внешней сущности
	ObjectName string // Название внешней сущности
	Field      string // Имя поля в текущей сущности
	Unique     bool   // Признак связки true => один к одному, false => один ко многим
}

// Структура описывающая сериализатор
type SerializerDeclaration struct {
	Name        string // имя
	Pkg         string // Пакет для импорта
	Type        string // Тип данных
	ImportName  string // Симлинк для импорта
	Marshaler   string // Имя функции маршалера
	Unmarshaler string // Имя функции анмаршаллера
}

// Структура описывающая дополнительный импорты
type ImportDeclaration struct {
	Path       string // Путь к пакету
	ImportName string // Симлинк для пакета при импорте
}

// Структура описывающая триггеры
type TriggerDeclaration struct {
	Name       string          // Имя
	Pkg        string          // Пакет для импорта
	Func       string          // Имя функции
	ImportName string          // Симлинк для импорта пакета
	Params     map[string]bool // Параметры передаваемые в функцию
}

// Структура описывающая флаги для поля
type FlagDeclaration struct {
	Name  string   // Имя
	Flags []string // Список имён флагов
}
