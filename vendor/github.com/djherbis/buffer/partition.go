package buffer

import (
	"encoding/gob"
	"io"
	"math"
)

type partition struct {
	List
	Pool
}

// NewPartition returns a Buffer which uses a Pool to extend or shrink its size as needed.
// It automatically allocates new buffers with pool.Get() to extend is length, and
// pool.Put() to release unused buffers as it shrinks.
func NewPartition(pool Pool, buffers ...Buffer) Buffer {
	return &partition{
		Pool: pool,
		List: buffers,
	}
}

func (buf *partition) Cap() int64 {
	return math.MaxInt64
}

func (buf *partition) Read(p []byte) (n int, err error) {
	for len(p) > 0 {

		if len(buf.List) == 0 {
			return n, io.EOF
		}

		buffer := buf.List[0]

		if Empty(buffer) {
			buf.Pool.Put(buf.Pop())
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

func (buf *partition) grow() error {
	next, err := buf.Pool.Get()
	if err != nil {
		return err
	}
	buf.Push(next)
	return nil
}

func (buf *partition) Write(p []byte) (n int, err error) {
	for len(p) > 0 {

		if len(buf.List) == 0 {
			if err := buf.grow(); err != nil {
				return n, err
			}
		}

		buffer := buf.List[len(buf.List)-1]

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

func (buf *partition) Reset() {
	for len(buf.List) > 0 {
		buf.Pool.Put(buf.Pop())
	}
}

func init() {
	gob.Register(&partition{})
}
