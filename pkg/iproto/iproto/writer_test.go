package iproto

import (
	"bytes"
	"fmt"
	"io"
	"runtime/debug"
	"testing"

	pbufio "github.com/mailru/activerecord/pkg/iproto/util/bufio"
)

func TestPutPacket(t *testing.T) {
	for _, test := range []struct {
		buf   []byte
		pkt   Packet
		exp   []byte
		panic bool
	}{
		{
			pkt:   Packet{Header{1, 0, 3}, []byte{}},
			panic: true,
		},
		{
			pkt: Packet{Header{1, 0, 3}, []byte{}},
			buf: make([]byte, 12),
			exp: []byte{
				1, 0, 0, 0,
				0, 0, 0, 0,
				3, 0, 0, 0,
			},
		},
	} {
		t.Run("", func(t *testing.T) {
			defer func() {
				err := recover()
				if test.panic && err == nil {
					t.Fatalf("want panic")
				}
				if !test.panic && err != nil {
					t.Fatalf("unexpected panic: %v\n%s", err, debug.Stack())
				}
			}()
			PutPacket(test.buf, test.pkt)
			if !bytes.Equal(test.buf, test.exp) {
				t.Fatalf(
					"PutPacket(%+v) =\n%v\nwant:\n%v\n",
					test.pkt, test.buf, test.exp,
				)
			}
		})
	}
}

func BenchmarkStreamWriter(b *testing.B) {
	for _, bench := range []struct {
		size int
		data int
	}{
		{
			size: DefaultWriteBufferSize,
			data: 0,
		},
		{
			size: DefaultWriteBufferSize,
			data: 200,
		},
		{
			size: DefaultWriteBufferSize,
			data: 3000,
		},
		{
			size: DefaultWriteBufferSize,
			data: 5000,
		},
	} {
		b.Run(fmt.Sprintf("pooled_buf%d_data%d", bench.size, bench.data), func(b *testing.B) {
			buf := pbufio.AcquireWriterSize(io.Discard, bench.size)
			bw := StreamWriter{
				Dest: buf,
			}
			data := bytes.Repeat([]byte{'x'}, bench.data)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				err := bw.WritePacket(Packet{
					Header: Header{
						Msg:  42,
						Len:  uint32(len(data)),
						Sync: uint32(i),
					},
					Data: data,
				})
				if err != nil {
					b.Fatal(err)
				}
			}

			b.StopTimer()
			pbufio.ReleaseWriter(buf, bench.size)
		})
	}
}
