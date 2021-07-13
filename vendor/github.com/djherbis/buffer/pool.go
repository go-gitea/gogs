package buffer

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"io/ioutil"
	"os"
	"sync"
)

// Pool provides a way to Allocate and Release Buffer objects
// Pools mut be concurrent-safe for calls to Get() and Put().
type Pool interface {
	Get() (Buffer, error) // Allocate a Buffer
	Put(buf Buffer) error // Release or Reuse a Buffer
}

type pool struct {
	pool sync.Pool
}

// NewPool returns a Pool(), it's backed by a sync.Pool so its safe for concurrent use.
// Get() and Put() errors will always be nil.
// It will not work with gob.
func NewPool(New func() Buffer) Pool {
	return &pool{
		pool: sync.Pool{
			New: func() interface{} {
				return New()
			},
		},
	}
}

func (p *pool) Get() (Buffer, error) {
	return p.pool.Get().(Buffer), nil
}

func (p *pool) Put(buf Buffer) error {
	buf.Reset()
	p.pool.Put(buf)
	return nil
}

type memPool struct {
	N int64
	Pool
}

// NewMemPool returns a Pool, Get() returns an in memory buffer of max size N.
// Put() returns the buffer to the pool after resetting it.
// Get() and Put() errors will always be nil.
func NewMemPool(N int64) Pool {
	return &memPool{
		N: N,
		Pool: NewPool(func() Buffer {
			return New(N)
		}),
	}
}

func (m *memPool) MarshalBinary() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := binary.Write(buf, binary.LittleEndian, m.N)
	return buf.Bytes(), err
}

func (m *memPool) UnmarshalBinary(data []byte) error {
	buf := bytes.NewReader(data)
	err := binary.Read(buf, binary.LittleEndian, &m.N)
	m.Pool = NewPool(func() Buffer {
		return New(m.N)
	})
	return err
}

type filePool struct {
	N         int64
	Directory string
}

// NewFilePool returns a Pool, Get() returns a file-based buffer of max size N.
// Put() closes and deletes the underlying file for the buffer.
// Get() may return an error if it fails to create a file for the buffer.
// Put() may return an error if it fails to delete the file.
func NewFilePool(N int64, dir string) Pool {
	return &filePool{N: N, Directory: dir}
}

func (p *filePool) Get() (Buffer, error) {
	file, err := ioutil.TempFile(p.Directory, "buffer")
	if err != nil {
		return nil, err
	}
	return NewFile(p.N, file), nil
}

func (p *filePool) Put(buf Buffer) (err error) {
	buf.Reset()
	if fileBuf, ok := buf.(*fileBuffer); ok {
		fileBuf.file.Close()
		err = os.Remove(fileBuf.file.Name())
	}
	return err
}

func init() {
	gob.Register(&memPool{})
	gob.Register(&filePool{})
}
