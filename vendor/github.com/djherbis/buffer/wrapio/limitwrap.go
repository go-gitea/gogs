package wrapio

import (
	"encoding/gob"
	"io"

	"github.com/djherbis/buffer/limio"
)

// ReadWriterAt implements io.ReaderAt and io.WriterAt
type ReadWriterAt interface {
	io.ReaderAt
	io.WriterAt
}

// Wrapper implements a io.ReadWriter and ReadWriterAt such that
// when reading/writing goes past N bytes, it "wraps" back to the beginning.
type Wrapper struct {
	// N is the offset at which to "wrap" back to the start
	N int64
	// L is the length of the data written
	L int64
	// O is our offset in the data
	O   int64
	rwa ReadWriterAt
}

// NewWrapper creates a Wrapper based on ReadWriterAt rwa.
// L is the current length, O is the current offset, and N is offset at which we "wrap".
func NewWrapper(rwa ReadWriterAt, L, O, N int64) *Wrapper {
	return &Wrapper{
		L:   L,
		O:   O,
		N:   N,
		rwa: rwa,
	}
}

// Len returns the # of bytes in the Wrapper
func (wpr *Wrapper) Len() int64 {
	return wpr.L
}

// Cap returns the "wrap" offset (max # of bytes)
func (wpr *Wrapper) Cap() int64 {
	return wpr.N
}

// Reset seeks to the start (0 offset), and sets the length to 0.
func (wpr *Wrapper) Reset() {
	wpr.O = 0
	wpr.L = 0
}

// SetReadWriterAt lets you switch the underlying Read/WriterAt
func (wpr *Wrapper) SetReadWriterAt(rwa ReadWriterAt) {
	wpr.rwa = rwa
}

// Read reads from the current offset into p, wrapping at Cap()
func (wpr *Wrapper) Read(p []byte) (n int, err error) {
	n, err = wpr.ReadAt(p, 0)
	wpr.L -= int64(n)
	wpr.O += int64(n)
	wpr.O %= wpr.N
	return n, err
}

// ReadAt reads from the current offset+off into p, wrapping at Cap()
func (wpr *Wrapper) ReadAt(p []byte, off int64) (n int, err error) {
	wrap := NewWrapReader(wpr.rwa, wpr.O+off, wpr.N)
	r := io.LimitReader(wrap, wpr.L-off)
	return r.Read(p)
}

// Write writes p to the end of the Wrapper (at Len()), wrapping at Cap()
func (wpr *Wrapper) Write(p []byte) (n int, err error) {
	return wpr.WriteAt(p, wpr.L)
}

// WriteAt writes p at the current offset+off, wrapping at Cap()
func (wpr *Wrapper) WriteAt(p []byte, off int64) (n int, err error) {
	wrap := NewWrapWriter(wpr.rwa, wpr.O+off, wpr.N)
	w := limio.LimitWriter(wrap, wpr.N-off)
	n, err = w.Write(p)
	if wpr.L < off+int64(n) {
		wpr.L = int64(n) + off
	}
	return n, err
}

func init() {
	gob.Register(&Wrapper{})
}
