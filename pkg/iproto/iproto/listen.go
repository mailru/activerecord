package iproto

import (
	"net"
	"time"

	"golang.org/x/net/context"
)

// AcceptFn allows user to construct Channel manually.
// It receives the server's context as first argument. It is user responsibility to make sub context if needed.
type AcceptFn func(net.Conn, *ChannelConfig) (*Channel, error)

func DefaultAccept(conn net.Conn, cfg *ChannelConfig) (*Channel, error) {
	return NewChannel(conn, cfg), nil
}

// Server contains options for serving IProto connections.
type Server struct {
	// Accept allow to rewrite default Channel creation logic.
	//
	// Normally, without setting Accept field, every accepted connection share
	// the same ChannelConfig. This could bring some trouble, when server
	// accepts two connections A and B, and handles packets from them with one
	// config.Handler, say, with N pooled goroutines. Then, if A will produce
	// huge stream of packets, B will not get fair amount of work time. The
	// better approach is to create separate handlers, each with its own pool.
	//
	// Note that if Accept returns error, server Serve() method will return
	// with that error.
	//
	// Note that returned Channel's Init() method will be called if err is
	// non-nil. Returned error from that Init() call will not be checked.
	Accept AcceptFn

	// ChannelConfig is used to initialize new Channel on every incoming
	// connection.
	//
	// Note that copy of config is shared across all channels.
	// To customize this behavior see Accept field of the Server.
	ChannelConfig *ChannelConfig

	// Log is used for write errors in serve process
	Log Logger

	// OnClose calls on channel close
	OnClose []func()

	// OnShutdown calls on channel shutdown
	OnShutdown []func()
}

// Serve begins to accept connection from ln. It does not handles net.Error
// temporary cases.
//
// Note that Serve() copies s.ChannelConfig once before starting accept loop.
//
//nolint:gocognit
func (s *Server) Serve(ctx context.Context, ln net.Listener) (err error) {
	accept := s.Accept
	if accept == nil {
		accept = DefaultAccept
	}

	config := CopyChannelConfig(s.ChannelConfig)

	var (
		tempDelay time.Duration // how long to sleep on accept failure
		log       Logger
	)

	if s.Log != nil {
		log = s.Log
	} else {
		log = &DefaultLogger{}
	}

	for {
		err := ctx.Err()
		if err == context.Canceled || err == context.DeadlineExceeded {
			return err
		}

		conn, err := ln.Accept()
		if err != nil {
			//nolint:staticcheck
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}

				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}

				if log != nil {
					log.Printf(ctx, "Accept error: %v; retrying in %v\n", err, tempDelay)
				}

				time.Sleep(tempDelay)

				continue
			}

			return err
		}

		tempDelay = 0

		ch, err := accept(conn, config)
		if err != nil {
			if log != nil {
				log.Printf(ctx, "Channel initalization error: %v\n", err)
			}

			continue
		}

		for _, f := range s.OnClose {
			ch.OnClose(f)
		}

		for _, f := range s.OnShutdown {
			ch.OnShutdown(f)
		}

		ch.SetContext(ctx)

		if err := ch.Init(); log != nil && err != nil {
			log.Printf(ctx, "Channel error: %v\n", err)
		}
	}
}

func (s *Server) ListenAndServe(ctx context.Context, network, addr string) error {
	ln, err := net.Listen(network, addr)
	if err != nil {
		return err
	}

	return s.Serve(ctx, ln)
}

// ListenAndServe creates listening socket on addr and starts serving IProto
// connections with default configured Server.
func ListenAndServe(ctx context.Context, network, addr string, h Handler) error {
	if h == nil {
		h = DefaultServeMux
	}

	s := &Server{
		ChannelConfig: &ChannelConfig{
			Handler: h,
		},
	}

	return s.ListenAndServe(ctx, network, addr)
}
