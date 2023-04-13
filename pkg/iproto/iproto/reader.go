package iproto

import (
	"encoding/binary"
	"fmt"
	"io"
)

const DefaultPacketLimit = 1 << 16

type BodyAllocator func(int) []byte

func DefaultAlloc(n int) []byte {
	return make([]byte, n)
}

// ReadPacket reads Packet from r.
func ReadPacket(r io.Reader) (ret Packet, err error) {
	return ReadPacketLimit(r, DefaultPacketLimit)
}

// ReadPacketLimit reads Packet from r.
// Size of packet's payload is limited to be at most n.
func ReadPacketLimit(r io.Reader, n uint32) (ret Packet, err error) {
	s := StreamReader{
		Source:    r,
		SizeLimit: n,
		Alloc:     DefaultAlloc,
	}

	return s.ReadPacket()
}

// StreamReader represents iproto stream reader.
type StreamReader struct {
	Source    io.Reader
	SizeLimit uint32
	Alloc     BodyAllocator

	buf [12]byte
	n   int
}

// ReadPackets reads next packet.
func (b *StreamReader) ReadPacket() (ret Packet, err error) {
	ret.Header, err = b.readHeader()
	if err != nil {
		return
	}

	if ret.Header.Len > b.SizeLimit {
		err = fmt.Errorf("iproto: packet data size limit of %v exceeded: %v", b.SizeLimit, ret.Header.Len)
		return
	}

	ret.Data = b.Alloc(int(ret.Header.Len))
	n, err := io.ReadFull(b.Source, ret.Data)
	b.n += n

	return
}

func (b *StreamReader) LastRead() int {
	return b.n
}

func (b *StreamReader) readHeader() (ret Header, err error) {
	b.n, err = io.ReadFull(b.Source, b.buf[:])
	ret.Msg = binary.LittleEndian.Uint32(b.buf[0:])
	ret.Len = binary.LittleEndian.Uint32(b.buf[4:])
	ret.Sync = binary.LittleEndian.Uint32(b.buf[8:])

	return
}
