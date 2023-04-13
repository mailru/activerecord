package ds

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mailru/activerecord/pkg/octopus"
)

// Описание приложения. Информация необходимая для разметки артефактов
// Остаёт в сгенерированных файлах, что бы было понятно какой версией сгенерированны файлы
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
	Num         int64
	PublicName  string
	PackageName string
	ModuleName  string
}

// Структура для описания конфигурации сервера
// Может быть указан путь к конфигурации `Conf` или параматры подключения напрямую
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
	Imports         []ImportDeclaration              // Список необходимых дополнительных импортов, формируется из дериктивы import
	ImportMap       map[string]int                   // Обратный индекс от имен по импортам
	ImportPkgMap    map[string]int                   // Обратный индекс от пакетов к импортам
	TriggerMap      map[string]TriggerDeclaration    // Список триггеров используемых в сущности
	FlagMap         map[string]FlagDeclaration       // Список флагов используемых в полях сущности
}

// Конструктор для RecordPackage, инициализирует ссылочные типы
func NewRecordPacakge() *RecordPackage {
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
	Num       uint8                 // номер индекса, используется для частичных индексов
	Selector  string                // название функции селектора
	Fields    []int                 // список номеров полей участвующих в индексе (последовательность имеет значение)
	FieldsMap map[string]IndexField // Обратный индекс по именам полей (используется для выявления дублей)
	Primary   bool                  // признак того, что индекс является первичным ключом
	Unique    bool                  // прихнак того, что индекс являетяс уникальным
	Type      string                // Тип индекса, для индексов по одному полю простой тип, для составных индексов собственный тип
}

// Тип описывающий поле в сущности
type FieldDeclaration struct {
	Name       string         // Название поля
	Format     octopus.Format // формат поля
	PrimaryKey bool           // участвует ли поле в первичном ключе (при изменении таких полей необходимо делать delete + insert вместо update)
	Mutators   []FieldMutator // список мутаторов (атомарных действий на уровне БД)
	Size       int64          // Размер поля, используется только для строковых значений
	Serializer []string       // Сериализаторф для поля
	ObjectLink string         // является ли поле ссылкой на другую сущность
}

// Метод возвращающий имя сериализоватора, если он установлен, иначе пустую строку
func (f *FieldDeclaration) SerializerName() string {
	if len(f.Serializer) > 0 {
		return f.Serializer[0]
	}

	return ""
}

// Парамтеры передаваемые при сериализации. Используется, когда на уровне декларирования
// известно, что сериализоатор/десериализатор требует дополнительных константных значений
func (f *FieldDeclaration) SerializerParams() string {
	if len(f.Serializer) > 1 {
		return `"` + strings.Join(f.Serializer[1:], `", "`) + `", `
	}

	return ""
}

// Тип и константы описывающие мутаторы для поля
type FieldMutator string

const (
	IncMutator      FieldMutator = "inc"       // инкремент (только для числовых типов)
	DecMutator      FieldMutator = "dec"       // дектиремн (только для числовых типов)
	SetBitMutator   FieldMutator = "set_bit"   // установка бита (только для целочисленных типов)
	ClearBitMutator FieldMutator = "clear_bit" // снятие бита (только для целочисленных типов)
	AndMutator      FieldMutator = "and"       // дизюнкция (только для целочисленных типов)
	OrMutator       FieldMutator = "or"        // конюнкция (только для целочисленных типов)
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
	Field      string // Имя поля в текужей сущности
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

// Структура описывающая дополнитеьный импорты
type ImportDeclaration struct {
	Path       string // Путь к пакету
	ImportName string // Симлинк для пакета при импорте
}

// Структура описывающая тригеры
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
