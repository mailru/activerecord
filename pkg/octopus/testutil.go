package octopus

import (
	"bytes"
	"context"
	"fmt"
	"sync/atomic"

	// Надо избавиться от использования этого модуля
	"log"
)

// FixtureType - структура определяющая ответ Response
// для конкретного запроса Request
type FixtureType struct {
	// Уникальный идентификатор фикстуры
	ID uint32

	// Msg - задаёт тип запроса (select, insert, delete, update)
	Msg RequetsTypeType

	// Байтовое представление запроса
	Request []byte

	// Байтовое представление ответа
	Response []byte

	// Возвращаемые объекты. Используется в режиме mock
	RespObjs []MockEntities

	// Trigger - функция, которая будет выполнена при обработке запроса
	// в случае если надо проверить insert или delete внутри этой функции
	// можно модифицировать список фикстур сервера
	Trigger func([]FixtureType) []FixtureType
}

type MockEntities interface {
	// Метод который позволяет отдать ответ из mock сервера
	MockSelectResponse() ([][]byte, error)

	// Метод который позволяет поднять сущность из БД
	RepoSelector(ctx context.Context) (any, error)
}

// CreateFixture - конструктор фикстур
func CreateFixture(id uint32, msg uint8, reqData []byte, respData []byte, trigger func([]FixtureType) []FixtureType) FixtureType {
	return FixtureType{
		ID:       id,
		Msg:      RequetsTypeType(msg),
		Request:  reqData,
		Response: respData,
		RespObjs: []MockEntities{},
		Trigger:  trigger,
	}
}

var fixtureID uint32

// CreateSelectFixture - конструктор фикстур для select-а
func CreateSelectFixture(reqData func(mocks []MockEntities) []byte, respEnt []MockEntities) (FixtureType, error) {
	newID := atomic.AddUint32(&fixtureID, 1)

	respByte, err := PackMockResponse(respEnt)
	if err != nil {
		return FixtureType{}, fmt.Errorf("error prepare fixture response: %s", err)
	}

	oft := FixtureType{
		ID:       newID,
		Msg:      RequestTypeSelect,
		Request:  reqData(respEnt),
		Response: respByte,
		RespObjs: respEnt,
		Trigger:  nil,
	}

	return oft, nil
}

func CreateUpdateFixture(reqData []byte, trigger func([]FixtureType) []FixtureType) FixtureType {
	newID := atomic.AddUint32(&fixtureID, 1)

	dummyRespBytes := [][][]byte{{{'0'}}}

	respData, err := PackResopnseStatus(RcOK, dummyRespBytes)
	if err != nil {
		log.Fatalf("error while pack update response: %s", err)
	}

	oft := FixtureType{
		ID:       newID,
		Msg:      RequestTypeUpdate,
		Request:  reqData,
		Response: respData,
		RespObjs: nil,
		Trigger:  trigger,
	}

	return oft
}

func CreateInsertOrReplaceFixture(entity MockEntities, reqData []byte, trigger func([]FixtureType) []FixtureType) FixtureType {
	newID := atomic.AddUint32(&fixtureID, 1)

	// by default return same entity
	respData, err := PackMockResponse([]MockEntities{entity})
	if err != nil {
		log.Fatalf("error while pack insert or replace response: %s", err)
	}

	oft := FixtureType{
		ID:       newID,
		Msg:      RequestTypeInsert,
		Request:  reqData,
		Response: respData,
		RespObjs: nil,
		Trigger:  trigger,
	}

	return oft
}

// CreateCallFixture - конструктор фикстур для вызова процедуры
func CreateCallFixture(reqData func(mocks []MockEntities) []byte, respEnt []MockEntities) (FixtureType, error) {
	newID := atomic.AddUint32(&fixtureID, 1)

	respByte, err := PackMockResponse(respEnt)
	if err != nil {
		return FixtureType{}, fmt.Errorf("error prepare fixture response: %s", err)
	}

	oft := FixtureType{
		ID:       newID,
		Msg:      RequestTypeCall,
		Request:  reqData(respEnt),
		Response: respByte,
		RespObjs: respEnt,
		Trigger:  nil,
	}

	return oft, nil
}

func WrapTriggerWithOnUsePromise(trigger func(types []FixtureType) []FixtureType) (wrappedTrigger func(types []FixtureType) []FixtureType, isUsed func() bool) {
	used := false

	promise := func() bool {
		return used
	}

	wrappedTrigger = func(types []FixtureType) []FixtureType {
		used = true

		if trigger != nil {
			return trigger(types)
		}

		return types
	}

	return wrappedTrigger, promise
}

func PackMockResponse(ome []MockEntities) ([]byte, error) {
	tuples := [][][]byte{}

	for _, om := range ome {
		tuple, err := om.MockSelectResponse()
		if err != nil {
			return nil, fmt.Errorf("error prepare fixtures: %s", err)
		}

		tuples = append(tuples, tuple)
	}

	return PackResopnseStatus(RcOK, tuples)
}

func UnpackSelect(data []byte) (ns, indexnum, offset, limit uint32, keys [][][]byte, err error) {
	rdr := bytes.NewReader(data)

	if ns, err = UnpackSpace(rdr); err != nil {
		return
	}

	if indexnum, err = UnpackIndexNum(rdr); err != nil {
		return
	}

	if offset, err = UnpackOffset(rdr); err != nil {
		return
	}

	if limit, err = UnpackLimit(rdr); err != nil {
		return
	}

	if keys, err = UnpackTuples(rdr); err != nil {
		return
	}

	return
}

type SelectMockFixture struct {
	Indexnum   uint32
	Offset     uint32
	Limit      uint32
	Keys       [][][]byte
	RespTuples []TupleData
}

type InsertMockFixture struct {
	NeedRetVal bool
	InsertMode InsertMode
	Tuple      TupleData
}

type UpdateMockFixture struct {
	PrimaryKey [][]byte
	UpdateOps  []Ops
}

type DeleteMockFixture struct {
	PrimaryKey [][]byte
}

type CallMockFixture struct {
	ProcName   string
	Args       [][]byte
	RespTuples []TupleData
}
