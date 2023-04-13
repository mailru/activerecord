package iproto

import (
	"fmt"
	"log"
	"net"
	"runtime"
	"sync"

	"github.com/mailru/activerecord/pkg/iproto/util/pool"
	"golang.org/x/net/context"
)

// Handler represents IProto packets handler.
type Handler interface {
	// ServeIProto called on each incoming non-technical Packet.
	// It called with underlying channel context. It is handler responsibility to make sub context if needed.
	ServeIProto(ctx context.Context, c Conn, p Packet)
}

type HandlerFunc func(context.Context, Conn, Packet)

func (f HandlerFunc) ServeIProto(ctx context.Context, c Conn, p Packet) { f(ctx, c, p) }

var DefaultServeMux = NewServeMux()

func Handle(message uint32, handler Handler) { DefaultServeMux.Handle(message, handler) }

// Sender represetns iproto packets sender in different forms.
type Sender interface {
	Call(ctx context.Context, message uint32, data []byte) ([]byte, error)
	Notify(ctx context.Context, message uint32, data []byte) error
	Send(ctx context.Context, packet Packet) error
}

// Closer represents channel that could be closed.
type Closer interface {
	Close()
	Shutdown()
	Done() <-chan struct{}
	OnClose(func())
}

// Conn represents channel that has ability to reply to received packets.
type Conn interface {
	Sender
	Closer

	// GetBytes obtains bytes from the Channel's byte pool.
	GetBytes(n int) []byte
	// PutBytes reclaims bytes to the Channel's byte pool.
	PutBytes(p []byte)

	RemoteAddr() net.Addr
	LocalAddr() net.Addr
}

var emptyHandler = HandlerFunc(func(context.Context, Conn, Packet) {})

type ServeMux struct {
	mu       sync.RWMutex
	handlers map[uint32]Handler
}

func NewServeMux() *ServeMux {
	return &ServeMux{
		handlers: make(map[uint32]Handler),
	}
}

func (s *ServeMux) Handle(message uint32, handler Handler) {
	s.mu.Lock()
	if _, ok := s.handlers[message]; ok {
		panic(fmt.Sprintf("iproto: multiple handlers for %x", message))
	}

	s.handlers[message] = handler

	s.mu.Unlock()
}

func (s *ServeMux) Handler(message uint32) Handler {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if h, ok := s.handlers[message]; ok {
		return h
	}

	return emptyHandler
}

func (s *ServeMux) ServeIProto(ctx context.Context, c Conn, p Packet) {
	s.Handler(p.Header.Msg).ServeIProto(ctx, c, p)
}

// RecoverHandler tries to make recover after handling packet.
// If panic was occured it logs its message and stack of panicked goroutine.
// Note that this handler should be the last one in the chain of handler wrappers,
// e.g.: PoolHandler(RecoverHandler(h)) or ParallelHandler(RecoverHandler(h)).
func RecoverHandler(h Handler) Handler {
	return HandlerFunc(func(ctx context.Context, c Conn, pkt Packet) {
		defer func() {
			if err := recover(); err != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				log.Printf("iproto: panic serving %v: %v\n%s", c.RemoteAddr().String(), err, buf)
			}
		}()

		h.ServeIProto(ctx, c, pkt)
	})
}

// PoolHandler returns Handler that schedules to handle packets by h in given pool p.
func PoolHandler(h Handler, p *pool.Pool) Handler {
	return HandlerFunc(func(ctx context.Context, c Conn, pkt Packet) {
		_ = p.Schedule(pool.TaskFunc(func() {
			h.ServeIProto(ctx, c, pkt)
		}))
	})
}

// ParallelHandler wraps handler and starts goroutine for each request on demand.
// It runs maximum n goroutines in one time. After serving request goroutine is exits.
func ParallelHandler(h Handler, n int) Handler {
	sem := make(chan struct{}, n)

	return HandlerFunc(func(ctx context.Context, c Conn, pkt Packet) {
		sem <- struct{}{}
		go func() {
			h.ServeIProto(ctx, c, pkt)
			<-sem
		}()
	})
}
