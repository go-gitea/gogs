package buffer

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"io/ioutil"
	"os"
	"sync"
)

// PoolAt provides a way to Allocate and Release BufferAt objects
// PoolAt's mut be concurrent-safe for calls to Get() and Put().
type PoolAt interface {
	Get() (BufferAt, error) // Allocate a BufferAt
	Put(buf BufferAt) error // Release or Reuse a BufferAt
}

type poolAt struct {
	poolAt sync.Pool
}

// NewPoolAt returns a PoolAt(), it's backed by a sync.Pool so its safe for concurrent use.
// Get() and Put() errors will always be nil.
// It will not work with gob.
func NewPoolAt(New func() BufferAt) PoolAt {
	return &poolAt{
		poolAt: sync.Pool{
			New: func() interface{} {
				return New()
			},
		},
	}
}

func (p *poolAt) Get() (BufferAt, error) {
	return p.poolAt.Get().(BufferAt), nil
}

func (p *poolAt) Put(buf BufferAt) error {
	buf.Reset()
	p.poolAt.Put(buf)
	return nil
}

type memPoolAt struct {
	N int64
	PoolAt
}

// NewMemPoolAt returns a PoolAt, Get() returns an in memory buffer of max size N.
// Put() returns the buffer to the pool after resetting it.
// Get() and Put() errors will always be nil.
func NewMemPoolAt(N int64) PoolAt {
	return &memPoolAt{
		N: N,
		PoolAt: NewPoolAt(func() BufferAt {
			return New(N)
		}),
	}
}

func (m *memPoolAt) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := binary.Write(buf, binary.LittleEndian, m.N)
	return buf.Bytes(), err
}

func (m *memPoolAt) UnmarshalBinary(data []byte) error {
	buf := bytes.NewReader(data)
	err := binary.Read(buf, binary.LittleEndian, &m.N)
	m.PoolAt = NewPoolAt(func() BufferAt {
		return New(m.N)
	})
	return err
}

type filePoolAt struct {
	N         int64
	Directory string
}

// NewFilePoolAt returns a PoolAt, Get() returns a file-based buffer of max size N.
// Put() closes and deletes the underlying file for the buffer.
// Get() may return an error if it fails to create a file for the buffer.
// Put() may return an error if it fails to delete the file.
func NewFilePoolAt(N int64, dir string) PoolAt {
	return &filePoolAt{N: N, Directory: dir}
}

func (p *filePoolAt) Get() (BufferAt, error) {
	file, err := ioutil.TempFile(p.Directory, "buffer")
	if err != nil {
		return nil, err
	}
	return NewFile(p.N, file), nil
}

func (p *filePoolAt) Put(buf BufferAt) (err error) {
	buf.Reset()
	if fileBuf, ok := buf.(*fileBuffer); ok {
		fileBuf.file.Close()
		err = os.Remove(fileBuf.file.Name())
	}
	return err
}

func init() {
	gob.Register(&memPoolAt{})
	gob.Register(&filePoolAt{})
}
