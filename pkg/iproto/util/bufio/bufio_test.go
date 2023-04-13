package bufio

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"testing"
)

func TestAcquireReaderSize(t *testing.T) {
	str := "hello, world"
	size := minPooledSize

	// Prepare two sources that consists of odd and even bytes of str
	// plus size-1 trailing trash bytes. This is done for bufio.Reader
	// fill underlying buffer with one byte from str per one Read() call.
	var b1, b2 []byte
	for i := 0; i < len(str); i++ {
		switch i % 2 {
		case 0:
			b1 = append(b1, str[i])
			b1 = append(b1, bytes.Repeat([]byte{'-'}, size-1)...)
		case 1:
			b2 = append(b2, str[i])
			b2 = append(b2, bytes.Repeat([]byte{'-'}, size-1)...)
		}
	}
	s1 := bytes.NewReader(b1)
	s2 := bytes.NewReader(b2)

	buf := &bytes.Buffer{}

	// Put source bufio.Writer in the pool.
	// We expect that this writer will be reused in all cases below.
	initial := AcquireReaderSize(nil, size)
	ReleaseReader(initial, size)

	var (
		r   *bufio.Reader
		src io.Reader
	)
	for i := 0; buf.Len() < len(str); i++ {
		// Detect which action we should perform next.
		switch i % 2 {
		case 0:
			src = s1
		case 1:
			src = s2
		}

		// Get the reader. Expect that we reuse initial reader.
		if r = AcquireReaderSize(src, size); r != initial {
			t.Errorf("%dth AcquireWriterSize did not returned initial writer", i)
		}

		// Write byte to the writer.
		b, err := r.ReadByte()
		if err != nil {
			t.Errorf("%dth ReadBytes unexpected error: %s", i, err)
			break
		}

		buf.WriteByte(b)

		// Put writer back to be resued in next iteration.
		ReleaseReader(r, size)
	}

	if buf.String() != str {
		t.Errorf("unexpected contents of buf: %s; want %s", buf.String(), str)
	}
}

func TestAcquireWriterSize(t *testing.T) {
	buf1 := &bytes.Buffer{}
	buf2 := &bytes.Buffer{}
	str := "hello, world!"
	size := minPooledSize

	// Put source bufio.Writer in the pool.
	// We expect that this writer will be reused in all cases below.
	initial := AcquireWriterSize(nil, size)
	ReleaseWriter(initial, size)

	var (
		w     *bufio.Writer
		dest  io.Writer
		flush bool
	)
	for i, j := 0, 0; j < len(str); i++ {
		// Detect which action we should perform next.
		var inc int
		switch i % 3 {
		case 0:
			dest = buf1
			flush = true
		case 1:
			dest = buf2
			flush = true
		default:
			dest = io.Discard
			flush = false
			inc = 1
		}
		// Get the writer. Expect that we reuse initial.
		if w = AcquireWriterSize(dest, size); w != initial {
			t.Errorf("%dth AcquireWriterSize did not returned initial writer", i)

		}
		// Write byte to the writer.
		_ = w.WriteByte(str[j])
		if flush {
			w.Flush()
		}
		// Put writer back to be resued in next iteration.
		ReleaseWriter(w, size)

		// Maybe take the next char in str.
		j += inc
	}

	if buf1.String() != str {
		t.Errorf("unexpected contents of buf1: %s; want %s", buf1.String(), str)
	}
	if buf2.String() != str {
		t.Errorf("unexpected contents of buf2: %s; want %s", buf2.String(), str)
	}
}

func TestCeilToPowerOfTwo(t *testing.T) {
	for _, test := range []struct {
		in, out int
	}{
		{
			in:  1,
			out: 1,
		},
		{
			in:  0,
			out: 0,
		},
		{
			in:  3,
			out: 4,
		},
		{
			in:  5,
			out: 8,
		},
		{
			in:  math.MaxInt32 >> 1,
			out: math.MaxInt32>>1 + 1,
		},
	} {
		t.Run(fmt.Sprintf("%v=>%v", test.in, test.out), func(t *testing.T) {
			if out := ceilToPowerOfTwo(test.in); out != test.out {
				t.Errorf("ceilToPowerOfTwo(%v) = %v; want %v", test.in, out, test.out)
			}
		})
	}
}
