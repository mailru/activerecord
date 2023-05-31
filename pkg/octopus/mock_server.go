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

func (l *DefaultLogger) DebugSelectRequest(ns uint32, indexnum uint32, offset uint32, limit uint32, keys [][][]byte) {
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

func (l *DefaultLogger) DebugUpdateRequest(ns uint32, primaryKey [][]byte, updateOps []Ops) {
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

func (l *DefaultLogger) DebugDeleteRequest(ns uint32, primaryKey [][]byte) {
	if l.DebugMeta != nil {
		l.Debug("Delete: " + l.DebugMeta.GetDeleteDebugInfo(ns, primaryKey))
		return
	}

	l.Debug("Delete: Space: %d, pk: %+v", ns, primaryKey)
}

func (l *DefaultLogger) DebugInsertRequest(ns uint32, needRetVal bool, insertMode InsertMode, tuple TupleData) {
	if l.DebugMeta != nil {
		l.Debug("Insert: " + l.DebugMeta.GetInsertDebugInfo(ns, needRetVal, insertMode, tuple))
		return
	}

	l.Debug("Inserty: Space: %d, need return value: %t, insertMode: %b, tuple: % X", ns, needRetVal, insertMode, tuple)
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
	reqNs, err := oms.DebugFixtureRequest(msg, req, nil)
	if err != nil {
		oms.logger.Debug("error while unpack request fixture %s", err)
	}

	oms.logger.Debug("---dump of '%s' fixture requests that a setup in mockserver on space: %d---", RequetsTypeType(msg), reqNs)
	for _, fix := range oms.oft {
		if fix.Msg == RequetsTypeType(msg) {
			if _, err := oms.DebugFixtureRequest(msg, fix.Request, func(ns uint32) bool {
				return ns == reqNs
			}); err != nil {
				oms.logger.Debug("error while unpack setup fixture %s", err)
			}
		}
	}
	oms.logger.Debug("---------------------------------------------------------------------------------")
}

/*
func (oms *MockServer) DumpFixturesResponse() {
	for _, fix := range oms.oft {
		reqNs, err := oms.DebugFixtureRequest(uint8(fix.Msg), fix.Request, nil)
		if err != nil {
			oms.logger.Debug("error while unpack request fixture %s", err)
		}

		oms.DebugFixtureResponse(reqNs, uint8(fix.Msg), fix.Response)
	}
}

func (oms *MockServer) DebugFixtureResponse(ns uint32, msg uint8, respBytes []byte) {
	tuplesData, err := ProcessResp(respBytes, 0)
	if err != nil {
		oms.logger.Debug("error while unpack request fixture %s", err)
	}

	switch RequetsTypeType(msg) {
	case RequestTypeSelect:
		oms.logger.DebugSelectResponse(ns, tuplesData)
	default:
		oms.logger.Debug("Request type %d not support debug message", msg)
	}

}*/

func (oms *MockServer) DebugFixtureRequest(msg uint8, req []byte, filter func(ns uint32) bool) (ns uint32, err error) {
	switch RequetsTypeType(msg) {
	case RequestTypeSelect:
		ns, indexnum, offset, limit, keys, err := UnpackSelect(req)
		if err != nil {
			return 0, fmt.Errorf("error while unpack select (% X): %w", req, err)
		}

		if filter == nil || filter(ns) {
			oms.logger.DebugSelectRequest(ns, indexnum, offset, limit, keys)
		}

		return ns, nil
	case RequestTypeUpdate:
		ns, primaryKey, updateOps, err := UnpackUpdate(req)
		if err != nil {
			return 0, fmt.Errorf("error while unpack update (% X): %w", req, err)
		}

		if filter == nil || filter(ns) {
			oms.logger.DebugUpdateRequest(ns, primaryKey, updateOps)
		}

		return ns, nil
	case RequestTypeInsert:
		ns, needRetVal, insertMode, tuple, err := UnpackInsertReplace(req)
		if err != nil {
			return 0, fmt.Errorf("error while unpack insert (% X): %w", req, err)
		}

		if filter == nil || filter(ns) {
			oms.logger.DebugInsertRequest(ns, needRetVal, insertMode, TupleData{Cnt: uint32(len(tuple)), Data: tuple})
		}

		return ns, nil
	case RequestTypeDelete:
		ns, primaryKey, err := UnpackDelete(req)
		if err != nil {
			return 0, fmt.Errorf("error while unpack delete (% X): %w", req, err)
		}

		if filter == nil || filter(ns) {
			oms.logger.DebugDeleteRequest(ns, primaryKey)
		}

		return ns, nil
	default:
		return 0, fmt.Errorf("Request type %d not support debug message", msg)
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
