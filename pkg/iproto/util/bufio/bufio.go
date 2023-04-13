// Package bufio contains tools for reusing bufio.Reader and bufio.Writer.
package bufio

import (
	"bufio"
	"io"
	"sync"
)

const (
	minPooledSize = 256
	maxPooledSize = 65536

	defaultBufSize = 4096
)

var (
	writers = map[int]*sync.Pool{}
	readers = map[int]*sync.Pool{}
)

//nolint:gochecknoinits
func init() {
	for n := minPooledSize; n <= maxPooledSize; n <<= 1 {
		writers[n] = new(sync.Pool)
		readers[n] = new(sync.Pool)
	}
}

// AcquireWriter returns bufio.Writer with default buffer size.
func AcquireWriter(w io.Writer) *bufio.Writer {
	return AcquireWriterSize(w, defaultBufSize)
}

// AcquireWriterSize returns bufio.Writer with given buffer size.
// Note that size is rounded up to nearest highest power of two.
func AcquireWriterSize(w io.Writer, size int) *bufio.Writer {
	if size == 0 {
		size = defaultBufSize
	}

	n := ceilToPowerOfTwo(size)

	if p, ok := writers[n]; ok {
		if v := p.Get(); v != nil {
			ret := v.(*bufio.Writer)
			ret.Reset(w)

			return ret
		}
	}

	return bufio.NewWriterSize(w, size)
}

// ReleaseWriterSize takses bufio.Writer for future reuse.
// Note that size should be the same as used to acquire writer.
// If you have acquired writer from AcquireWriter function, set size to 0.
// If size == 0 then default buffer size is used.
func ReleaseWriter(w *bufio.Writer, size int) {
	if size == 0 {
		size = defaultBufSize
	}

	n := ceilToPowerOfTwo(size)

	if p, ok := writers[n]; ok {
		w.Reset(nil)
		p.Put(w)
	}
}

// AcquireWriter returns bufio.Writer with default buffer size.
func AcquireReader(r io.Reader) *bufio.Reader {
	return AcquireReaderSize(r, defaultBufSize)
}

// AcquireReaderSize returns bufio.Reader with given buffer size.
// Note that size is rounded up to nearest highest power of two.
func AcquireReaderSize(r io.Reader, size int) *bufio.Reader {
	if size == 0 {
		size = defaultBufSize
	}

	n := ceilToPowerOfTwo(size)

	if p, ok := readers[n]; ok {
		if v := p.Get(); v != nil {
			ret := v.(*bufio.Reader)
			ret.Reset(r)

			return ret
		}
	}

	return bufio.NewReaderSize(r, size)
}

// ReleaseReaderSize takes bufio.Reader for future reuse.
// Note that size should be the same as used to acquire reader.
// If you have acquired reader from AcquireReader function, set size to 0.
// If size == 0 then default buffer size is used.
func ReleaseReader(r *bufio.Reader, size int) {
	if size == 0 {
		size = defaultBufSize
	}

	n := ceilToPowerOfTwo(size)
	if p, ok := readers[n]; ok {
		r.Reset(nil)
		p.Put(r)
	}
}

// ceilToPowerOfTwo rounds n to the highest power of two integer.
func ceilToPowerOfTwo(n int) int {
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++

	return n
}
