package iproto

import (
	"bytes"
	"io"
	"net"
	"testing"

	"golang.org/x/net/context"
)

func TestListenPacket(t *testing.T) {
	s, err := ListenPacket("udp", "127.0.0.1:0", &PacketServerConfig{
		Handler: HandlerFunc(func(_ context.Context, conn Conn, pkt Packet) {
			if string(pkt.Data) != "request" {
				t.Fatalf("unexpected packet: %v (%s)", pkt, pkt.Data)
			}
			if err := conn.Send(bg, ResponseTo(pkt, []byte("response"))); err != nil {
				t.Fatal(err)
			}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	pool, err := Dial(bg, "udp", s.LocalAddr().String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := pool.Call(bg, 42, []byte("request"))
	if err != nil {
		t.Fatal(err)
	}
	if string(resp) != "response" {
		t.Fatalf("unexpected response: %s", resp)
	}

	s.Close()
}

func TestPacketServerSend(t *testing.T) {
	packet := func(m, s uint32, d string) Packet {
		return Packet{
			Header{m, uint32(len(d)), s},
			[]byte(d),
		}
	}

	closed := make(chan struct{})
	var writes []bytesWithAddr
	conn := &stubPacketConn{
		close: func() error {
			close(closed)
			return nil
		},
		readFrom: func(p []byte) (int, net.Addr, error) {
			<-closed
			return 0, nil, io.EOF
		},
		writeTo: func(p []byte, addr net.Addr) (int, error) {
			writes = append(writes, bytesWithAddr{
				append(([]byte)(nil), p...), addr,
			})
			return len(p), nil
		},
	}
	s := NewPacketServer(conn, &PacketServerConfig{
		MaxTransmissionUnit: 54, // At most 4 empty packets.
	})
	_ = s.Init()

	for _, send := range []struct {
		packet Packet
		addr   net.Addr
	}{
		{packet(42, 1, ""), strAddr("A")},

		{packet(99, 1, ""), strAddr("B")},

		{packet(42, 2, ""), strAddr("A")},
		{packet(42, 3, ""), strAddr("A")},

		{packet(99, 2, ""), strAddr("B")},
		{packet(99, 3, ""), strAddr("B")},

		{packet(42, 4, ""), strAddr("A")},

		{packet(33, 1, ""), strAddr("C")},
		{packet(33, 2, ""), strAddr("C")},
		{packet(33, 3, ""), strAddr("C")},
		{packet(33, 4, ""), strAddr("C")},
		{packet(33, 5, ""), strAddr("C")},
		{packet(33, 6, ""), strAddr("C")},
		{packet(33, 7, ""), strAddr("C")},
		{packet(33, 8, ""), strAddr("C")},
	} {
		_ = s.send(bg, send.addr, send.packet)
	}
	s.Close()
	for i, w := range writes {
		r := bytes.NewReader(w.bytes)
		for r.Len() > 0 {
			if _, err := ReadPacketLimit(r, 0); err != nil {
				t.Errorf(
					"can not read packet from %d-th datagram from %s: %v",
					i, w.addr, err,
				)
				break
			}
		}
	}
}

type strAddr string

const stub = "stub"

func (s strAddr) Network() string { return stub }
func (s strAddr) String() string  { return string(s) }

type stubPacketConn struct {
	net.PacketConn

	close    func() error
	readFrom func([]byte) (int, net.Addr, error)
	writeTo  func([]byte, net.Addr) (int, error)
}

func (s stubPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	if s.writeTo != nil {
		return s.writeTo(p, addr)
	}
	return len(p), nil
}

func (s stubPacketConn) LocalAddr() net.Addr {
	return strAddr(stub)
}

func (s stubPacketConn) Close() error {
	if s.close != nil {
		return s.close()
	}
	return nil
}

func (s stubPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	if s.readFrom != nil {
		return s.readFrom(p)
	}
	return 0, nil, nil
}
