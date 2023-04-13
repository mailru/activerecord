package iproto

import (
	"bytes"
	"crypto/rand"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
)

func TestChannelHijack(t *testing.T) {
	server, client, err := makeSocketPair()
	if err != nil {
		t.Fatal(err)
	}
	ch := NewChannel(server, nil)
	if err = ch.Init(); err != nil {
		t.Fatal(err)
	}
	conn, r, err := ch.Hijack()
	if err != nil {
		t.Fatalf("Hijack() error: %v", err)
	}
	if conn != server {
		t.Fatalf("Hijack() returned different connection")
	}
	if r != nil {
		t.Errorf(
			"Hijack() returned non-nil read buffer; want nil",
		)
	}

	// Test the hijacked connection.
	p := MarshalPacket(Packet{})
	//nolint:errcheck
	go client.Write(p)
	if _, err := ReadPacket(conn); err != nil {
		t.Fatalf("ReadPacket() from hijacked conn error: %v", err)
	}
}

func TestChannelHijackWithoutDeadline(t *testing.T) {
	t.Skip("Test fails on linux")
	server, _ := net.Pipe()
	ch := NewChannel(server, nil)
	if err := ch.Init(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ch.Hijack(); err == nil {
		t.Fatalf("Hijack() expected error; got nil")
	}
}

func TestChannelHijackBuffer(t *testing.T) {
	type (
		respErr struct {
			resp []byte
			err  error
		}
		hijackResult struct {
			conn net.Conn
			r    []byte
			err  error
		}
	)

	server, client, err := makeSocketPair()
	if err != nil {
		t.Fatal(err)
	}
	ch := NewChannel(server, &ChannelConfig{
		Handler: HandlerFunc(func(ctx context.Context, conn Conn, pkt Packet) {
			t.Fatalf("unexpected handler Handle() call: %v", pkt)
		}),
	})
	if err = ch.Init(); err != nil {
		t.Fatal(err)
	}
	// 1) Lock the channel Hijack() until it receive response.
	//    After Hijack() call below, channel must begin buffer incoming packets
	//    that are not responses on made requests.
	resp := make(chan respErr, 1)
	go func() {
		r, errCall := ch.Call(bg, 42, nil)
		resp <- respErr{r, errCall}
	}()
	time.Sleep(time.Millisecond)

	// 2) Call Hijack() to put channel in hijacked state and lead it to
	//    buffer all non-response packets.
	hj := make(chan hijackResult, 1)
	go func() {
		conn, r, errHijack := ch.Hijack()
		hj <- hijackResult{conn, r, errHijack}
	}()
	time.Sleep(time.Millisecond)

	// 3) Send some non-response packets. We expect them to be buffered.
	for i := 0; i < 5; i++ {
		err = WritePacket(client, Packet{
			Header{Msg: 21, Sync: uint32(i)}, nil,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// 4) Send the response to the call made at (1) step.
	req, err := ReadPacket(client)
	if err != nil {
		t.Fatalf("can't read request packet: %v", err)
	}
	if err = WritePacket(client, ResponseTo(req, nil)); err != nil {
		t.Fatalf("can't send response: %v", err)
	}

	// 5) Receive (1) result.
	if r := <-resp; r.err != nil {
		t.Fatalf("unexpected Call() error: %v", r.err)
	}

	// 6) Receive (2) result.
	h := <-hj
	if h.err != nil {
		t.Fatalf("Hijack() error: %v", h.err)
	}
	if h.r == nil {
		t.Fatalf("Hijack() returned nil for bufio.ReadWriter")
	}

	// 7) Check the buffered data to be valid packets sent before at (3).
	rd := bytes.NewReader(h.r)
	for i := 0; i < 5; i++ {
		pkt, err := ReadPacket(rd)
		if err != nil {
			t.Fatalf("can't read %dth packet from hijack buffer: %v", i, err)
		}
		if sync := pkt.Header.Sync; sync != uint32(i) {
			t.Fatalf("buffered packets wrong order: got sync %d; want %d", sync, i)
		}
	}
	if rd.Len() != 0 {
		t.Fatalf("hijack read buffer is non empty after packets read")
	}

	<-ch.Done()
}

func TestChannelHijackSplitPacket(t *testing.T) {
	for _, test := range []struct {
		name  string
		split int
		size  int
	}{
		{"1/2551", 1, 2551},
		{"2/2551", 2, 2551},
		{"3/2551", 3, 2551},
		{"4/2551", 4, 2551},
		{"5/2551", 5, 2551},
		{"6/2551", 6, 2551},
		{"7/2551", 7, 2551},
		{"8/2551", 8, 2551},
		{"9/2551", 9, 2551},
		{"10/2551", 10, 2551},
		{"11/2551", 11, 2551},
		{"12/2551", 12, 2551},
		{"100/2551", 100, 2551},
		{"500/2551", 500, 2551},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, client, err := makeSocketPair()
			if err != nil {
				t.Fatal(err)
			}
			ch := NewChannel(server, nil)
			if err = ch.Init(); err != nil {
				t.Fatal(err)
			}

			packet := genPacket(test.size)
			dump := MarshalPacket(packet)

			// 1) Write test.split bytes to the conn and then stuck up at
			//    release channel. Then write the remaining bytes.
			if _, err = client.Write(dump[:test.split]); err != nil {
				t.Fatal(err)
			}

			release := make(chan struct{})

			//nolint:staticcheck,govet
			go func() {
				<-release
				if _, err = client.Write(dump[test.split:]); err != nil {
					//nolint:staticcheck,govet
					t.Fatal(err)
				}
			}()

			// 2) Call Hijack() to cancel reading packet which write is stucked
			//    up at i-th byte at (1).
			conn, r, err := ch.Hijack()
			if err != nil {
				t.Fatal(err)
			}

			// 3) Signal to send the remaining part of the packet.
			close(release)

			// 4) Check buffered bytes to be valid packet.
			reader := io.MultiReader(
				bytes.NewReader(r),
				conn,
			)
			pkt, err := ReadPacket(reader)
			if err != nil {
				t.Fatalf("read packet error: %v", err)
			}
			if act, exp := pkt.Header, packet.Header; act != exp {
				t.Fatalf("headers mismatch: %+v; want %v", act, exp)
			}
			if act, exp := pkt.Data, packet.Data; !bytes.Equal(act, exp) {
				t.Fatalf("payload mismatch:\n%#x\n%#x", act, exp)
			}
		})
	}
}

func genPacket(n int) Packet {
	payload := make([]byte, n)
	_, _ = rand.Read(payload)
	return Packet{
		Header{Msg: 42, Len: uint32(len(payload)), Sync: 0xffffffff},
		payload,
	}
}

// makeSocketPair is a wrapper around unix syscall.
// It is intended to emulate real-world net.Conn instances which work well with
// the SetDeadline implementations.
func makeSocketPair() (r, w net.Conn, err error) {
	fd, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return
	}
	// SetNonblock the reader part.
	if err = unix.SetNonblock(fd[0], true); err != nil {
		return
	}
	r, err = net.FileConn(os.NewFile(uintptr(fd[0]), "r"))
	if err != nil {
		return nil, nil, err
	}
	w, err = net.FileConn(os.NewFile(uintptr(fd[1]), "w"))
	if err != nil {
		return nil, nil, err
	}
	return r, w, nil
}
