package buffer

import (
	"encoding/gob"
	"io"
)

type swap struct {
	A BufferAt
	B BufferAt
}

// NewSwap creates a Buffer which writes to a until you write past a.Cap()
// then it io.Copy's from a to b and writes to b.
// Once the Buffer is empty again, it starts over writing to a.
// Note that if b.Cap() <= a.Cap() it will cause a panic, b is expected
// to be larger in order to accommodate writes past a.Cap().
func NewSwap(a, b Buffer) Buffer {
	return NewSwapAt(toBufferAt(a), toBufferAt(b))
}

// NewSwapAt creates a BufferAt which writes to a until you write past a.Cap()
// then it io.Copy's from a to b and writes to b.
// Once the Buffer is empty again, it starts over writing to a.
// Note that if b.Cap() <= a.Cap() it will cause a panic, b is expected
// to be larger in order to accommodate writes past a.Cap().
func NewSwapAt(a, b BufferAt) BufferAt {
	if b.Cap() <= a.Cap() {
		panic("Buffer b must be larger than a.")
	}
	return &swap{A: a, B: b}
}

func (buf *swap) Len() int64 {
	return buf.A.Len() + buf.B.Len()
}

func (buf *swap) Cap() int64 {
	return buf.B.Cap()
}

func (buf *swap) Read(p []byte) (n int, err error) {
	if buf.A.Len() > 0 {
		return buf.A.Read(p)
	}
	return buf.B.Read(p)
}

func (buf *swap) ReadAt(p []byte, off int64) (n int, err error) {
	if buf.A.Len() > 0 {
		return buf.A.ReadAt(p, off)
	}
	return buf.B.ReadAt(p, off)
}

func (buf *swap) Write(p []byte) (n int, err error) {
	switch {
	case buf.B.Len() > 0:
		n, err = buf.B.Write(p)

	case buf.A.Len()+int64(len(p)) > buf.A.Cap():
		_, err = io.Copy(buf.B, buf.A)
		if err == nil {
			n, err = buf.B.Write(p)
		}

	default:
		n, err = buf.A.Write(p)
	}

	return n, err
}

func (buf *swap) WriteAt(p []byte, off int64) (n int, err error) {
	switch {
	case buf.B.Len() > 0:
		n, err = buf.B.WriteAt(p, off)

	case off+int64(len(p)) > buf.A.Cap():
		_, err = io.Copy(buf.B, buf.A)
		if err == nil {
			n, err = buf.B.WriteAt(p, off)
		}

	default:
		n, err = buf.A.WriteAt(p, off)
	}

	return n, err
}

func (buf *swap) Reset() {
	buf.A.Reset()
	buf.B.Reset()
}

func init() {
	gob.Register(&swap{})
}
