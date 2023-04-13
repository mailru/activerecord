// Package io contains utility for working with golang's io objects.
package io

import (
	"io"
	"time"
)

// Stat represents statistics about underlying io.{Reader,Writer} usage.
type Stat struct {
	Bytes uint32 // Bytes sent/read from underlying object.
	Calls uint32 // Read/Write calls made to the underlying object.
}

// reader is a wrapper around io.Reader.
// Underlying reader should not be *bufio.Reader.
// It used to calculate stats of reading from underlying reader.
type Reader struct {
	r     io.Reader
	bytes uint32 // bytes read
	calls uint32 // calls made
}

// WrapReader wraps r into Reader to calculate usage stats of r.
// Note that Reader is not goroutine safe.
func WrapReader(r io.Reader) *Reader {
	ret := &Reader{
		r: r,
	}

	return ret
}

// Read implements io.Reader interface.
func (r *Reader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.bytes += uint32(n)
	r.calls++

	return n, err
}

// Stat returns underlying io.Reader usage statistics.
func (r *Reader) Stat() Stat {
	return Stat{
		Bytes: r.bytes,
		Calls: r.calls,
	}
}

// Writer is a wrapper around io.Writer.
// Underlying writer should not be *bufio.Writer.
// It used to calculate stats of writing to underlying writer.
type Writer struct {
	w     io.Writer
	bytes uint32 // bytes written
	calls uint32 // calls made
}

// WrapWriter wraps w into Writer to calculate usage stats of w.
// Note that Writer is not goroutine safe.
func WrapWriter(w io.Writer) *Writer {
	ret := &Writer{
		w: w,
	}

	return ret
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.bytes += uint32(n)
	w.calls++

	return n, err
}

// Stat returns underlying io.Writer usage statistics.
func (w *Writer) Stat() Stat {
	return Stat{
		Bytes: w.bytes,
		Calls: w.calls,
	}
}

// DeadlineWriter describes object that could prepare io.Writer methods with
// some deadline.
type DeadlineWriter interface {
	io.Writer
	SetWriteDeadline(time.Time) error
}

// DeadlineWriter describes object that could prepare io.Reader methods with
// some deadline.
type DeadlineReader interface {
	io.Reader
	SetReadDeadline(time.Time) error
}

// TimeoutWriter is a wrapper around DeadlineWriter that sets write deadline on
// each Write() call. It is useful as destination for bufio.Writer, when you do
// not exactly know, when Write() will occure, but want to control timeout of
// such calls.
type TimeoutWriter struct {
	Dest    DeadlineWriter
	Timeout time.Duration
}

// Write implements io.Writer interface.
func (w TimeoutWriter) Write(p []byte) (int, error) {
	if err := w.Dest.SetWriteDeadline(time.Now().Add(w.Timeout)); err != nil {
		return 0, err
	}

	return w.Dest.Write(p)
}

// TimeoutReader is a wrapper around DeadlineReader that sets read deadline on
// each Read() call. It is useful as destination for bufio.Reader, when you do
// not exactly know, when Read() will occure, but want to control timeout of
// such calls.
type TimeoutReader struct {
	Dest    DeadlineReader
	Timeout time.Duration
}

// Read implements io.Reader interface.
func (w TimeoutReader) Read(p []byte) (int, error) {
	if err := w.Dest.SetReadDeadline(time.Now().Add(w.Timeout)); err != nil {
		return 0, err
	}

	return w.Dest.Read(p)
}
