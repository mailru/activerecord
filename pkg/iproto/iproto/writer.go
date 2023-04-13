package iproto

import (
	"encoding/binary"
	"io"
	"net"
	"time"

	wio "github.com/mailru/activerecord/pkg/iproto/util/io"
)

// WritePacket writes p to w.
func WritePacket(w io.Writer, p Packet) (err error) {
	s := StreamWriter{Dest: w}
	return s.WritePacket(p)
}

// StreamWriter represents iproto stream writer.
type StreamWriter struct {
	Dest io.Writer

	buf [12]byte // used to encode header
}

// WritePacket writes p to the underlying writer.
func (b *StreamWriter) WritePacket(p Packet) (err error) {
	// Prepare header.
	binary.LittleEndian.PutUint32(b.buf[0:4], p.Header.Msg)
	binary.LittleEndian.PutUint32(b.buf[4:8], uint32(len(p.Data)))
	binary.LittleEndian.PutUint32(b.buf[8:12], p.Header.Sync)

	_, err = b.Dest.Write(b.buf[:])
	if err != nil {
		return
	}

	_, err = b.Dest.Write(p.Data)
	if err != nil {
		return
	}

	return
}

// PutPacket puts packet binary representation to given slice.
// Note that it will panic if p doesn't fit PacketSize(pkt).
func PutPacket(p []byte, pkt Packet) {
	binary.LittleEndian.PutUint32(p[0:], pkt.Header.Msg)
	binary.LittleEndian.PutUint32(p[4:], pkt.Header.Len)
	binary.LittleEndian.PutUint32(p[8:], pkt.Header.Sync)
	copy(p[12:len(pkt.Data)+12], pkt.Data)
}

// MarshalPacket returns binary representation of pkt.
func MarshalPacket(pkt Packet) []byte {
	p := make([]byte, PacketSize(pkt))
	PutPacket(p, pkt)

	return p
}

// PacketSize returns packet binary representation size.
func PacketSize(pkt Packet) int {
	return len(pkt.Data) + 12 // 12 is for header size.
}

type buffers struct {
	// dest is a buffers destination. Note that it must not be wrapped such
	// that its unexported methods become hidden. That is, normally
	// net.Buffers.WriteTo() uses writev() syscall for net.bufferWriter
	// implementors. It is much efficient than for plain io.Writer.
	conn    wio.DeadlineWriter
	timeout time.Duration

	b    net.Buffers
	n    int
	size int
	free func([]byte)

	sent int64
}

func (b *buffers) Flush() error {
	if b.n == 0 {
		return nil
	}

	// Set write deadline. Will reset it below.
	_ = b.conn.SetWriteDeadline(time.Now().Add(b.timeout))

	// Save pointer to the head buffer to call free() below.
	// That is, b.b.WriteTo() will set b.b = b.b[1:] for each item. This is not
	// so good for us because of two reasons: first, we will always alloc space
	// for new appended buffer; second, we can not iterate over buffers after
	// WriteTo() to call free() on them.
	head := b.b

	n, err := b.b.WriteTo(b.conn)

	// Reset deadline anyway.
	_ = b.conn.SetWriteDeadline(noDeadline)

	if err == nil && int(n) != b.n {
		err = io.ErrShortWrite
	}

	if err != nil {
		return err
	}

	b.sent += n

	for _, buf := range head {
		b.free(buf)
	}

	b.b = head[:0]
	b.n = 0

	return err
}

func (b *buffers) Append(p []byte) error {
	b.b = append(b.b, p)

	b.n += len(p)
	if b.n > b.size {
		return b.Flush()
	}

	return nil
}
