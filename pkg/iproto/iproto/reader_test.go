package iproto

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	pbufio "github.com/mailru/activerecord/pkg/iproto/util/bufio"
)

type loopBytesReader struct {
	data []byte
	r    int // read position
}

func (l *loopBytesReader) Read(p []byte) (n int, err error) {
	for n < len(p) {
		c := copy(p[n:], l.data[l.r:])
		n += c
		l.r = (l.r + c) % len(l.data)
	}
	return
}

func makeSyncAlloc() BodyAllocator {
	cache := map[int][]byte{}
	return func(size int) []byte {
		bts, ok := cache[size]
		if !ok {
			bts = make([]byte, size)
			cache[size] = bts
		}
		return bts
	}
}

func getLoopPacketReader(n int) io.Reader {
	buf := &bytes.Buffer{}
	_ = WritePacket(buf, Packet{
		Header: Header{
			Msg:  42,
			Sync: 0,
			Len:  uint32(n),
		},
		Data: bytes.Repeat([]byte{'x'}, n),
	})

	return &loopBytesReader{data: buf.Bytes()}
}

func BenchmarkStreamReader(b *testing.B) {
	for _, bench := range []struct {
		size  int
		data  int
		alloc BodyAllocator
	}{
		{
			size: DefaultReadBufferSize,
			data: 0,
		},
		{
			size: DefaultReadBufferSize,
			data: 200,
		},
		{
			size: DefaultReadBufferSize,
			data: 3000,
		},
		{
			size: DefaultReadBufferSize,
			data: 5000,
		},
		{
			size:  DefaultReadBufferSize,
			data:  0,
			alloc: makeSyncAlloc(),
		},
		{
			size:  DefaultReadBufferSize,
			data:  200,
			alloc: makeSyncAlloc(),
		},
		{
			size:  DefaultReadBufferSize,
			data:  3000,
			alloc: makeSyncAlloc(),
		},
		{
			size:  DefaultReadBufferSize,
			data:  5000,
			alloc: makeSyncAlloc(),
		},
	} {
		var sufix string
		if bench.alloc != nil {
			sufix = "_sync"
		} else {
			sufix = "_dflt"
			bench.alloc = DefaultAlloc
		}

		b.Run(fmt.Sprintf("pooled_buf%d_data%d%s", bench.size, bench.data, sufix), func(b *testing.B) {
			buf := pbufio.AcquireReaderSize(getLoopPacketReader(bench.data), bench.size)
			br := StreamReader{
				Source:    buf,
				SizeLimit: 1 << 16,
				Alloc:     bench.alloc,
			}
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := br.ReadPacket()
				if err != nil {
					b.Fatal(err)
				}
			}

			b.StopTimer()
			pbufio.ReleaseReader(buf, bench.size)
		})
	}
}
