package buffer

import (
	"bytes"
	"encoding/gob"
	"io"
	"math"
)

type chain struct {
	Buf  BufferAt
	Next BufferAt
}

type nopBufferAt struct {
	Buffer
}

func (buf *nopBufferAt) ReadAt(p []byte, off int64) (int, error) {
	panic("ReadAt not implemented")
}

func (buf *nopBufferAt) WriteAt(p []byte, off int64) (int, error) {
	panic("WriteAt not implemented")
}

// toBufferAt converts a Buffer to a BufferAt with nop ReadAt and WriteAt funcs
func toBufferAt(buf Buffer) BufferAt {
	return &nopBufferAt{Buffer: buf}
}

// NewMultiAt returns a BufferAt which is the logical concatenation of the passed BufferAts.
// The data in the buffers is shifted such that there is no non-empty buffer following
// a non-full buffer, this process is also run after every Read.
// If no buffers are passed, the returned Buffer is nil.
func NewMultiAt(buffers ...BufferAt) BufferAt {
	if len(buffers) == 0 {
		return nil
	} else if len(buffers) == 1 {
		return buffers[0]
	}

	buf := &chain{
		Buf:  buffers[0],
		Next: NewMultiAt(buffers[1:]...),
	}

	buf.Defrag()

	return buf
}

// NewMulti returns a Buffer which is the logical concatenation of the passed buffers.
// The data in the buffers is shifted such that there is no non-empty buffer following
// a non-full buffer, this process is also run after every Read.
// If no buffers are passed, the returned Buffer is nil.
func NewMulti(buffers ...Buffer) Buffer {
	bufAt := make([]BufferAt, len(buffers))
	for i, buf := range buffers {
		bufAt[i] = toBufferAt(buf)
	}
	return NewMultiAt(bufAt...)
}

func (buf *chain) Reset() {
	buf.Next.Reset()
	buf.Buf.Reset()
}

func (buf *chain) Cap() (n int64) {
	Next := buf.Next.Cap()
	if buf.Buf.Cap() > math.MaxInt64-Next {
		return math.MaxInt64
	}
	return buf.Buf.Cap() + Next
}

func (buf *chain) Len() (n int64) {
	Next := buf.Next.Len()
	if buf.Buf.Len() > math.MaxInt64-Next {
		return math.MaxInt64
	}
	return buf.Buf.Len() + Next
}

func (buf *chain) Defrag() {
	for !Full(buf.Buf) && !Empty(buf.Next) {
		r := io.LimitReader(buf.Next, Gap(buf.Buf))
		if _, err := io.Copy(buf.Buf, r); err != nil && err != io.EOF {
			return
		}
	}
}

func (buf *chain) Read(p []byte) (n int, err error) {
	n, err = buf.Buf.Read(p)
	if len(p[n:]) > 0 && (err == nil || err == io.EOF) {
		m, err := buf.Next.Read(p[n:])
		n += m
		if err != nil {
			return n, err
		}
	}

	buf.Defrag()

	return n, err
}

func (buf *chain) ReadAt(p []byte, off int64) (n int, err error) {
	if buf.Buf.Len() < off {
		return buf.Next.ReadAt(p, off-buf.Buf.Len())
	}

	n, err = buf.Buf.ReadAt(p, off)
	if len(p[n:]) > 0 && (err == nil || err == io.EOF) {
		var m int
		m, err = buf.Next.ReadAt(p[n:], 0)
		n += m
	}
	return n, err
}

func (buf *chain) Write(p []byte) (n int, err error) {
	if n, err = buf.Buf.Write(p); err == io.ErrShortWrite {
		err = nil
	}
	p = p[n:]
	if len(p) > 0 && err == nil {
		m, err := buf.Next.Write(p)
		n += m
		if err != nil {
			return n, err
		}
	}
	return n, err
}

func (buf *chain) WriteAt(p []byte, off int64) (n int, err error) {
	switch {
	case buf.Buf.Cap() <= off: // past the end
		return buf.Next.WriteAt(p, off-buf.Buf.Cap())

	case buf.Buf.Cap() >= off+int64(len(p)): // fits in
		return buf.Buf.WriteAt(p, off)

	default: // partial fit
		n, err = buf.Buf.WriteAt(p, off)
		if len(p[n:]) > 0 && (err == nil || err == io.ErrShortWrite) {
			var m int
			m, err = buf.Next.WriteAt(p[n:], 0)
			n += m
		}
		return n, err
	}
}

func init() {
	gob.Register(&chain{})
	gob.Register(&nopBufferAt{})
}

func (buf *chain) MarshalBinary() ([]byte, error) {
	b := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(b)
	if err := enc.Encode(&buf.Buf); err != nil {
		return nil, err
	}
	if err := enc.Encode(&buf.Next); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (buf *chain) UnmarshalBinary(data []byte) error {
	b := bytes.NewBuffer(data)
	dec := gob.NewDecoder(b)
	if err := dec.Decode(&buf.Buf); err != nil {
		return err
	}
	if err := dec.Decode(&buf.Next); err != nil {
		return err
	}
	return nil
}
