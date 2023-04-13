package testutil

import (
	"net"

	"github.com/mailru/activerecord/pkg/iproto/iproto"
	"golang.org/x/net/context"
)

type StubResponseWriter struct {
	DoCall       func(context.Context, uint32, []byte) ([]byte, error)
	DoNotify     func(context.Context, uint32, []byte) error
	DoSend       func(context.Context, iproto.Packet) error
	DoOnClose    func(func())
	DoClose      func()
	DoDone       func() <-chan struct{}
	DoShutdown   func()
	DoRemoteAddr func() net.Addr
	DoLocalAddr  func() net.Addr
}

func NewFakeResponseWriter() *StubResponseWriter {
	return &StubResponseWriter{
		DoCall:       func(context.Context, uint32, []byte) ([]byte, error) { return nil, nil },
		DoNotify:     func(context.Context, uint32, []byte) error { return nil },
		DoSend:       func(context.Context, iproto.Packet) error { return nil },
		DoOnClose:    func(func()) {},
		DoClose:      func() {},
		DoDone:       func() <-chan struct{} { return nil },
		DoShutdown:   func() {},
		DoRemoteAddr: func() net.Addr { return localAddr(0) },
		DoLocalAddr:  func() net.Addr { return localAddr(0) },
	}
}

func (s *StubResponseWriter) Call(ctx context.Context, message uint32, data []byte) ([]byte, error) {
	return s.DoCall(ctx, message, data)
}
func (s *StubResponseWriter) Notify(ctx context.Context, message uint32, data []byte) error {
	return s.DoNotify(ctx, message, data)
}
func (s *StubResponseWriter) Send(ctx context.Context, packet iproto.Packet) error {
	return s.DoSend(ctx, packet)
}
func (s *StubResponseWriter) Close() {
	s.DoClose()
}
func (s *StubResponseWriter) Shutdown() {
	s.DoShutdown()
}
func (s *StubResponseWriter) OnClose(f func()) {
	s.DoOnClose(f)
}
func (s *StubResponseWriter) Done() <-chan struct{} {
	return s.DoDone()
}
func (s *StubResponseWriter) RemoteAddr() net.Addr {
	return s.DoRemoteAddr()
}
func (s *StubResponseWriter) LocalAddr() net.Addr {
	return s.DoLocalAddr()
}
func (s *StubResponseWriter) GetBytes(n int) []byte {
	return make([]byte, n)
}
func (s *StubResponseWriter) PutBytes([]byte) {
}

type localAddr int

const Local = "local"

func (l localAddr) Network() string { return Local }
func (l localAddr) String() string  { return Local }
