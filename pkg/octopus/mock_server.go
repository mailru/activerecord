package octopus

import (
	"context"
	"fmt"
	"log"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/mailru/activerecord/pkg/iproto/iproto"
)

type CheckUsesFixtureType uint8

// Константы для проверки использования фикстур
const (
	AnyUsesFixtures CheckUsesFixtureType = iota
	AllFixtureUses
	AllFixtureUsesOnlyOnce
)

type MockServer struct {
	// Сервер
	srv *iproto.Server

	// Хост и порт на котором сервер запускается
	host, port string

	// листенер сервера
	ln net.Listener

	// Список фикстур с которым будет работать сервер
	oft []FixtureType

	// Мьютекс для работы с триггерами
	sync.Mutex

	// Контекст для остановки
	cancelCtx context.CancelFunc

	// Канал сигнализирующий об остановке сервера
	stopServ chan struct{}

	// Функция для логирования работы мок сервера
	logger MockServerLogger

	iprotoLogger iproto.Logger
}

type RepositoryDebugMeta interface {
	GetSelectDebugInfo(ns uint32, indexnum uint32, offset uint32, limit uint32, keys [][][]byte) string
	GetUpdateDebugInfo(ns uint32, primaryKey [][]byte, updateOps []Ops) string
	GetInsertDebugInfo(ns uint32, needRetVal bool, insertMode InsertMode, tuple TupleData) string
	GetDeleteDebugInfo(ns uint32, primaryKey [][]byte) string
}

type DefaultLogger struct {
	DebugMeta RepositoryDebugMeta
}

func (l *DefaultLogger) Debug(fmt string, args ...any) {
	log.Printf("DEBUG: "+fmt, args...)
}

func (l *DefaultLogger) DebugSelectRequest(ns uint32, indexnum uint32, offset uint32, limit uint32, keys [][][]byte, fixture ...SelectMockFixture) {
	if l.DebugMeta != nil {
		l.Debug("Select: " + l.DebugMeta.GetSelectDebugInfo(ns, indexnum, offset, limit, keys))
		return
	}

	keyStr := ""

	for _, key := range keys {
		hexField := []string{}

		for _, field := range key {
			hexField = append(hexField, fmt.Sprintf("% X", field))
		}

		keyStr += "[" + strings.Join(hexField, ", ") + "]"
	}

	l.Debug("Select: Space: %d, index: %d, offset: %d, limit: %d, Keys: %s", ns, indexnum, offset, limit, keyStr)
}

func (l *DefaultLogger) DebugUpdateRequest(ns uint32, primaryKey [][]byte, updateOps []Ops, fixture ...UpdateMockFixture) {
	if l.DebugMeta != nil {
		l.Debug("Update: " + l.DebugMeta.GetUpdateDebugInfo(ns, primaryKey, updateOps))
		return
	}

	opsStr := ""

	for _, op := range updateOps {
		opsStr += fmt.Sprintf("%d %d <= % X; ", op.Op, op.Field, op.Value)
	}

	l.Debug("Update: Space: %d, pk: %+v, updateOps: %s", ns, primaryKey, opsStr)
}

func (l *DefaultLogger) DebugDeleteRequest(ns uint32, primaryKey [][]byte, fixture ...DeleteMockFixture) {
	if l.DebugMeta != nil {
		l.Debug("Delete: " + l.DebugMeta.GetDeleteDebugInfo(ns, primaryKey))
		return
	}

	l.Debug("Delete: Space: %d, pk: %+v", ns, primaryKey)
}

func (l *DefaultLogger) DebugInsertRequest(ns uint32, needRetVal bool, insertMode InsertMode, tuple TupleData, fixture ...InsertMockFixture) {
	if l.DebugMeta != nil {
		l.Debug("Insert: " + l.DebugMeta.GetInsertDebugInfo(ns, needRetVal, insertMode, tuple))
		return
	}

	l.Debug("Inserty: Space: %d, need return value: %t, insertMode: %b, tuple: % X", ns, needRetVal, insertMode, tuple)
}

func (l *DefaultLogger) DebugCallRequest(procName string, args [][]byte, fixtures ...CallMockFixture) {

}

type NopIprotoLogger struct{}

func (l NopIprotoLogger) Printf(ctx context.Context, fmt string, v ...interface{}) {}

func (l NopIprotoLogger) Debugf(ctx context.Context, fmt string, v ...interface{}) {}

func InitMockServer(opts ...MockServerOption) (*MockServer, error) {
	oms := &MockServer{
		oft:      []FixtureType{},
		host:     "127.0.0.1",
		port:     "0",
		stopServ: make(chan struct{}),
	}

	oms.logger = &DefaultLogger{}

	if oms.iprotoLogger == nil {
		oms.iprotoLogger = &NopIprotoLogger{}
	}

	for _, opt := range opts {
		err := opt.apply(oms)
		if err != nil {
			return nil, fmt.Errorf("error apply option: %s", err)
		}
	}

	addr, err := net.ResolveTCPAddr("tcp", oms.host+":"+oms.port)
	if err != nil {
		return nil, err
	}

	ln, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("can't start listener: %s", err)
	}

	oms.ln = ln
	oms.port = strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)

	oms.srv = &iproto.Server{
		ChannelConfig: &iproto.ChannelConfig{
			Handler: iproto.HandlerFunc(oms.Handler),
			Logger:  oms.iprotoLogger,
		},
		Log: oms.iprotoLogger,
	}

	return oms, nil
}

func (oms *MockServer) Handler(ctx context.Context, c iproto.Conn, p iproto.Packet) {
	oms.iprotoLogger.Debugf(ctx, "% X (% X)", p.Header, p.Data)

	resp, found := oms.ProcessRequest(uint8(p.Header.Msg), p.Data)
	if !found {
		res := append([]byte{0x01, 0x00, 0x00, 0x00}, []byte("Fixture not found")...)
		if err := c.Send(ctx, iproto.ResponseTo(p, res)); err != nil {
			oms.logger.Debug("error send from octopus server: %s", err)
		}

		return
	}

	if err := c.Send(ctx, iproto.ResponseTo(p, resp)); err != nil {
		oms.logger.Debug("error send from octopus server: %s", err)
	}
}

func (oms *MockServer) DebugFixtureNotFound(msg uint8, req []byte) {
	switch RequetsTypeType(msg) {
	case RequestTypeSelect:
		reqNs, indexnum, offset, limit, keys, err := UnpackSelect(req)
		if err != nil {
			oms.logger.Debug("error while unpack select (% X): %s", req, err)
			return
		}

		var fixtures []SelectMockFixture

		for _, fxt := range oms.oft {
			// only "Select" fixture type
			if fxt.Msg != RequetsTypeType(msg) {
				continue
			}

			ns, idxNum, ofs, lmt, keysFxt, err := UnpackSelect(fxt.Request)
			if err != nil {
				oms.logger.Debug("error while unpack select (% X): %s", fxt.Request, err)
				return
			}

			// only namespace for requestType "Select"
			if reqNs != ns {
				continue
			}

			selectFxt := SelectMockFixture{
				indexnum: idxNum,
				offset:   ofs,
				limit:    lmt,
				keys:     keysFxt,
			}

			if selectFxt.respTuples, err = ProcessResp(fxt.Response, 0); err != nil {
				oms.logger.Debug("error while unpack select response (% X): %s", fxt.Response, err)
				return
			}

			fixtures = append(fixtures, selectFxt)
		}

		oms.logger.DebugSelectRequest(reqNs, indexnum, offset, limit, keys, fixtures...)
	case RequestTypeUpdate:
		reqNs, primaryKey, updateOps, err := UnpackUpdate(req)
		if err != nil {
			oms.logger.Debug("error while unpack update (% X): %s", req, err)
			return
		}

		var fixtures []UpdateMockFixture

		for _, fxt := range oms.oft {
			// only "Update" fixture type
			if fxt.Msg != RequetsTypeType(msg) {
				continue
			}

			ns, pk, ops, err := UnpackUpdate(fxt.Request)
			if err != nil {
				oms.logger.Debug("error while unpack select (% X): %s", fxt.Request, err)
				return
			}

			// only namespace of requestType "Update"
			if reqNs != ns {
				continue
			}

			updateFxt := UpdateMockFixture{
				primaryKey: pk,
				updateOps:  ops,
			}

			fixtures = append(fixtures, updateFxt)
		}

		oms.logger.DebugUpdateRequest(reqNs, primaryKey, updateOps, fixtures...)
	case RequestTypeInsert:
		reqNs, needRetVal, insertMode, reqTuple, err := UnpackInsertReplace(req)
		if err != nil {
			oms.logger.Debug("error while unpack insert (% X): %s", req, err)
			return
		}

		var fixtures []InsertMockFixture

		for _, fxt := range oms.oft {
			// only "Insert" fixture type
			if fxt.Msg != RequetsTypeType(msg) {
				continue
			}

			ns, fxtNeedRetVal, mode, tuple, err := UnpackInsertReplace(fxt.Request)
			if err != nil {
				oms.logger.Debug("error while unpack insert (% X): %s", fxt.Request, err)
				return
			}

			// only namespace of requestType "Insert"
			if reqNs != ns {
				continue
			}

			insertFxt := InsertMockFixture{
				needRetVal: fxtNeedRetVal,
				insertMode: mode,
				tuple:      TupleData{Cnt: uint32(len(tuple)), Data: tuple},
			}

			fixtures = append(fixtures, insertFxt)
		}

		oms.logger.DebugInsertRequest(reqNs, needRetVal, insertMode, TupleData{Cnt: uint32(len(reqTuple)), Data: reqTuple}, fixtures...)
	case RequestTypeDelete:
		reqNs, primaryKey, err := UnpackDelete(req)
		if err != nil {
			oms.logger.Debug("error while unpack delete (% X): %s", req, err)
			return
		}

		var fixtures []DeleteMockFixture

		for _, fxt := range oms.oft {
			// only "Delete" fixture type
			if fxt.Msg != RequetsTypeType(msg) {
				continue
			}

			ns, pk, err := UnpackDelete(fxt.Request)
			if err != nil {
				oms.logger.Debug("error while unpack delete (% X): %s", fxt.Request, err)
				return
			}

			// only namespace of requestType "Delete"
			if reqNs != ns {
				continue
			}

			deleteFxt := DeleteMockFixture{
				primaryKey: pk,
			}

			fixtures = append(fixtures, deleteFxt)
		}

		oms.logger.DebugDeleteRequest(reqNs, primaryKey, fixtures...)
	case RequestTypeCall:
		reqProcName, args, err := UnpackLua(req)
		if err != nil {
			oms.logger.Debug("error while unpack call proc (% X): %s", req, err)
			return
		}

		var fixtures []CallMockFixture

		for _, fxt := range oms.oft {
			// only "Call" fixture type
			if fxt.Msg != RequetsTypeType(msg) {
				continue
			}

			procName, fixArgs, err := UnpackLua(fxt.Request)
			if err != nil {
				oms.logger.Debug("error while unpack select (% X): %s", fxt.Request, err)
				return
			}

			// filter by name
			if reqProcName != procName {
				continue
			}

			callFxt := CallMockFixture{
				procName: procName,
				args:     fixArgs,
			}

			if callFxt.respTuples, err = ProcessResp(fxt.Response, 0); err != nil {
				oms.logger.Debug("error while unpack call proc response (% X): %s", fxt.Response, err)
				return
			}

			fixtures = append(fixtures, callFxt)
		}

		oms.logger.DebugCallRequest(reqProcName, args, fixtures...)
	default:
		oms.logger.Debug("Request type %d not support debug message", msg)
	}
}

func (oms *MockServer) ProcessRequest(msg uint8, req []byte) ([]byte, bool) {
	oms.Lock()
	defer oms.Unlock()

	for _, fix := range oms.oft {
		if fix.Msg == RequetsTypeType(msg) && reflect.DeepEqual(fix.Request, req) {
			if fix.Trigger != nil {
				oms.oft = fix.Trigger(oms.oft)
			}

			return fix.Response, true
		}
	}

	oms.DebugFixtureNotFound(msg, req)

	return nil, false
}

func (oms *MockServer) SetFixtures(oft []FixtureType) {
	oms.Lock()
	oms.oft = oft
	oms.Unlock()
}

func (oms *MockServer) Stop() error {
	oms.logger.Debug("Try to stop server")
	oms.cancelCtx()

	timer := time.NewTimer(time.Second * 10)

	oms.logger.Debug("Close listener")

	if err := oms.ln.Close(); err != nil {
		oms.logger.Debug("can't close listener: %s", err)
	}

	select {
	case <-oms.stopServ:
		oms.logger.Debug("Server stoped successfully")
	case <-timer.C:
		oms.logger.Debug("error stop server: timeout")
	}

	return nil
}

func (oms *MockServer) GetServerHostPort() string {
	return oms.host + ":" + oms.port
}

func (oms *MockServer) Start() error {
	ctx, cancel := context.WithCancel(context.Background())

	oms.cancelCtx = cancel

	go func(ctx context.Context, ln net.Listener) {
		oms.logger.Debug("Start octopus test server on %s", ln.Addr().String())

		err := oms.srv.Serve(ctx, ln)
		if err != nil && !errors.Is(err, context.Canceled) {
			oms.logger.Debug("Error get Serve: %s", err)
		}

		oms.stopServ <- struct{}{}

		oms.logger.Debug("Stop octopus test server")
	}(ctx, oms.ln)

	return nil
}
