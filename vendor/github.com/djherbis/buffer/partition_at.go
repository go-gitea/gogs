package buffer

import (
	"encoding/gob"
	"errors"
	"io"
	"math"
)

type partitionAt struct {
	ListAt
	PoolAt
}

// NewPartitionAt returns a BufferAt which uses a PoolAt to extend or shrink its size as needed.
// It automatically allocates new buffers with pool.Get() to extend is length, and
// pool.Put() to release unused buffers as it shrinks.
func NewPartitionAt(pool PoolAt, buffers ...BufferAt) BufferAt {
	return &partitionAt{
		PoolAt: pool,
		ListAt: buffers,
	}
}

func (buf *partitionAt) Cap() int64 {
	return math.MaxInt64
}

func (buf *partitionAt) Read(p []byte) (n int, err error) {
	for len(p) > 0 {

		if len(buf.ListAt) == 0 {
			return n, io.EOF
		}

		buffer := buf.ListAt[0]

		if Empty(buffer) {
			buf.PoolAt.Put(buf.Pop())
			continue
		}

		m, er := buffer.Read(p)
		n += m
		p = p[m:]

		if er != nil && er != io.EOF {
			return n, er
		}

	}
	return n, nil
}

func (buf *partitionAt) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errors.New("buffer.PartionAt.ReadAt: negative offset")
	}
	for _, buffer := range buf.ListAt {
		// Find the buffer where this offset is found.
		if buffer.Len() <= off {
			off -= buffer.Len()
			continue
		}

		m, er := buffer.ReadAt(p, off)
		n += m
		p = p[m:]

		if er != nil && er != io.EOF {
			return n, er
		}
		if len(p) == 0 {
			return n, er
		}
		// We need to read more, starting from 0 in the next buffer.
		off = 0
	}
	if len(p) > 0 {
		return n, io.EOF
	}
	return n, nil
}

func (buf *partitionAt) grow() error {
	next, err := buf.PoolAt.Get()
	if err != nil {
		return err
	}
	buf.Push(next)
	return nil
}

func (buf *partitionAt) Write(p []byte) (n int, err error) {
	for len(p) > 0 {

		if len(buf.ListAt) == 0 {
			if err := buf.grow(); err != nil {
				return n, err
			}
		}

		buffer := buf.ListAt[len(buf.ListAt)-1]

		if Full(buffer) {
			if err := buf.grow(); err != nil {
				return n, err
			}
			continue
		}

		m, er := buffer.Write(p)
		n += m
		p = p[m:]

		if er != nil && er != io.ErrShortWrite {
			return n, er
		}

	}
	return n, nil
}

func (buf *partitionAt) WriteAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, errors.New("buffer.PartionAt.WriteAt: negative offset")
	}
	if off == buf.Len() { // writing at the end special case
		if err := buf.grow(); err != nil {
			return 0, err
		}
	}
	fitCheck := BufferAt.Len
	for i := 0; i < len(buf.ListAt); i++ {
		buffer := buf.ListAt[i]

		// Find the buffer where this offset is found.
		if fitCheck(buffer) < off {
			off -= fitCheck(buffer)
			continue
		}

		if i+1 == len(buf.ListAt) {
			fitCheck = BufferAt.Cap
		}

		endOff := off + int64(len(p))
		if fitCheck(buffer) >= endOff {
			// Everything should fit.
			return buffer.WriteAt(p, off)
		}

		// Assume it won't all fit, only write what should fit.
		canFit := int(fitCheck(buffer) - off)
		if len(p[:canFit]) > 0 {
			var m int
			m, err = buffer.WriteAt(p[:canFit], off)
			n += m
			p = p[m:]
		}
		off = 0 // All writes are at offset 0 of following buffers now.

		if err != nil || len(p) == 0 {
			return n, err
		}
		if i+1 == len(buf.ListAt) {
			if err := buf.grow(); err != nil {
				return 0, err
			}
			fitCheck = BufferAt.Cap
		}
	}
	if len(p) > 0 {
		err = io.ErrShortWrite
	}
	return n, err
}

func (buf *partitionAt) Reset() {
	for len(buf.ListAt) > 0 {
		buf.PoolAt.Put(buf.Pop())
	}
}

func init() {
	gob.Register(&partitionAt{})
}
