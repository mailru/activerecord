package octopus

import (
	"context"
)

type (
	CountFlags uint32
	RetCode    uint32
	OpCode     uint8
	Format     string
)

type TupleData struct {
	Cnt  uint32
	Data [][]byte
}

type Ops struct {
	Field uint32
	Op    OpCode
	Value []byte
}

type ModelStruct interface {
	Insert(ctx context.Context) error
	Replace(ctx context.Context) error
	InsertOrReplace(ctx context.Context) error
	Update(ctx context.Context) error
	Delete(ctx context.Context) error
}

type BaseField struct {
	Collection      []ModelStruct
	UpdateOps       []Ops
	ExtraFields     [][]byte
	Objects         map[string][]ModelStruct
	FieldsetAltered bool
	Exists          bool
	ShardNum        uint32
	IsReplica       bool
	Readonly        bool
	Repaired        bool
}

type RequetsTypeType uint8

const (
	RequestTypeInsert RequetsTypeType = 13
	RequestTypeSelect RequetsTypeType = 17
	RequestTypeUpdate RequetsTypeType = 19
	RequestTypeDelete RequetsTypeType = 21
	RequestTypeCall   RequetsTypeType = 22
)

func (r RequetsTypeType) String() string {
	switch r {
	case RequestTypeInsert:
		return "Insert"
	case RequestTypeSelect:
		return "Select"
	case RequestTypeUpdate:
		return "Update"
	case RequestTypeDelete:
		return "Delete"
	case RequestTypeCall:
		return "Call"
	default:
		return "(unknown)"
	}
}

type InsertMode uint8

const (
	InsertModeInserOrReplace InsertMode = iota
	InsertModeInsert
	InsertModeReplace
)

const (
	SpaceLen uint32 = 4
	IndexLen
	LimitLen
	OffsetLen
	FlagsLen
	FieldNumLen
	OpsLen
	OpFieldNumLen
	OpOpLen = 1
)

type BoxMode uint8

const (
	ReplicaMaster BoxMode = iota
	MasterReplica
	ReplicaOnly
	MasterOnly
	SelectModeDefault = ReplicaMaster
)

const (
	UniqRespFlag CountFlags = 1 << iota
	NeedRespFlag
)

const (
	RcOK                   = RetCode(0x0)
	RcReadOnly             = RetCode(0x0401)
	RcLocked               = RetCode(0x0601)
	RcMemoryIssue          = RetCode(0x0701)
	RcNonMaster            = RetCode(0x0102)
	RcIllegalParams        = RetCode(0x0202)
	RcSecondaryPort        = RetCode(0x0301)
	RcBadIntegrity         = RetCode(0x0801)
	RcUnsupportedCommand   = RetCode(0x0a02)
	RcDuplicate            = RetCode(0x2002)
	RcWrongField           = RetCode(0x1e02)
	RcWrongNumber          = RetCode(0x1f02)
	RcWrongVersion         = RetCode(0x2602)
	RcWalIO                = RetCode(0x2702)
	RcDoesntExists         = RetCode(0x3102)
	RcStoredProcNotDefined = RetCode(0x3202)
	RcLuaError             = RetCode(0x3302)
	RcTupleExists          = RetCode(0x3702)
	RcDuplicateKey         = RetCode(0x3802)
)

const (
	OpSet OpCode = iota
	OpAdd
	OpAnd
	OpXor
	OpOr
	OpSplice
	OpDelete
	OpInsert
)

const (
	Uint8       Format = "uint8"
	Uint16      Format = "uint16"
	Uint32      Format = "uint32"
	Uint64      Format = "uint64"
	Uint        Format = "uint"
	Int8        Format = "int8"
	Int16       Format = "int16"
	Int32       Format = "int32"
	Int64       Format = "int64"
	Int         Format = "int"
	String      Format = "string"
	Bool        Format = "bool"
	Float32     Format = "float32"
	Float64     Format = "float64"
	StringArray Format = "[]string"
	ByteArray   Format = "[]byte"
)

var UnsignedFormat = []Format{Uint8, Uint16, Uint32, Uint64, Uint}
var NumericFormat = append(UnsignedFormat, Int8, Int16, Int32, Int64, Int)
var FloatFormat = []Format{Float32, Float64}
var DataFormat = []Format{String}
var AllFormat = append(append(append(
	NumericFormat,
	FloatFormat...),
	DataFormat...),
	Bool,
)
var AllProcFormat = append(append(append(
	NumericFormat,
	FloatFormat...),
	DataFormat...),
	Bool, StringArray, ByteArray,
)

func GetOpCodeName(op OpCode) string {
	switch op {
	case OpSet:
		return "Set"
	case OpAdd:
		return "Add"
	case OpAnd:
		return "And"
	case OpXor:
		return "Xor"
	case OpOr:
		return "Or"
	case OpSplice:
		return "Splice"
	case OpDelete:
		return "Delete"
	case OpInsert:
		return "Insert"
	default:
		return "invalid opcode"
	}
}

func GetInsertModeName(mode InsertMode) string {
	switch mode {
	case InsertMode(0):
		return "InsertOrReplaceMode"
	case InsertModeInsert:
		return "InsertMode"
	case InsertModeReplace:
		return "ReplaceMode"
	default:
		return "Invalid mode"
	}
}
