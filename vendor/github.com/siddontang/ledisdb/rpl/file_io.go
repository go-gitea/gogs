package rpl

import (
	"fmt"
	"io"
	"os"

	"github.com/edsrzf/mmap-go"
	"github.com/siddontang/go/log"
)

//like leveldb or rocksdb file interface, haha!

type writeFile interface {
	Sync() error
	Write(b []byte) (n int, err error)
	Close() error
	ReadAt(buf []byte, offset int64) (int, error)
	Truncate(size int64) error
	SetOffset(o int64)
	Name() string
	Size() int
	Offset() int64
}

type readFile interface {
	ReadAt(buf []byte, offset int64) (int, error)
	Close() error
	Size() int
	Name() string
}

type rawWriteFile struct {
	writeFile
	f      *os.File
	offset int64
	name   string
}

func newRawWriteFile(name string, size int64) (writeFile, error) {
	m := new(rawWriteFile)
	var err error

	m.name = name

	m.f, err = os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *rawWriteFile) Close() error {
	if err := m.f.Truncate(m.offset); err != nil {
		return fmt.Errorf("close truncate %s error %s", m.name, err.Error())
	}

	if err := m.f.Close(); err != nil {
		return fmt.Errorf("close %s error %s", m.name, err.Error())
	}

	return nil
}

func (m *rawWriteFile) Sync() error {
	return m.f.Sync()
}

func (m *rawWriteFile) Write(b []byte) (n int, err error) {
	n, err = m.f.WriteAt(b, m.offset)
	if err != nil {
		return
	} else if n != len(b) {
		err = io.ErrShortWrite
		return
	}

	m.offset += int64(n)
	return
}

func (m *rawWriteFile) ReadAt(buf []byte, offset int64) (int, error) {
	return m.f.ReadAt(buf, offset)
}

func (m *rawWriteFile) Truncate(size int64) error {
	var err error
	if err = m.f.Truncate(size); err != nil {
		return err
	}

	if m.offset > size {
		m.offset = size
	}
	return nil
}

func (m *rawWriteFile) SetOffset(o int64) {
	m.offset = o
}

func (m *rawWriteFile) Offset() int64 {
	return m.offset
}

func (m *rawWriteFile) Name() string {
	return m.name
}

func (m *rawWriteFile) Size() int {
	st, _ := m.f.Stat()
	return int(st.Size())
}

type rawReadFile struct {
	readFile

	f    *os.File
	name string
}

func newRawReadFile(name string) (readFile, error) {
	m := new(rawReadFile)

	var err error
	m.f, err = os.Open(name)
	m.name = name

	if err != nil {
		return nil, err
	}

	return m, err
}

func (m *rawReadFile) Close() error {
	return m.f.Close()
}

func (m *rawReadFile) Size() int {
	st, _ := m.f.Stat()
	return int(st.Size())
}

func (m *rawReadFile) ReadAt(b []byte, offset int64) (int, error) {
	return m.f.ReadAt(b, offset)
}

func (m *rawReadFile) Name() string {
	return m.name
}

/////////////////////////////////////////////////

type mmapWriteFile struct {
	writeFile

	f      *os.File
	m      mmap.MMap
	name   string
	size   int64
	offset int64
}

func newMmapWriteFile(name string, size int64) (writeFile, error) {
	m := new(mmapWriteFile)

	m.name = name

	var err error

	m.f, err = os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	if size == 0 {
		st, _ := m.f.Stat()
		size = st.Size()
	}

	if err = m.f.Truncate(size); err != nil {
		return nil, err
	}

	if m.m, err = mmap.Map(m.f, mmap.RDWR, 0); err != nil {
		return nil, err
	}

	m.size = size
	m.offset = 0
	return m, nil
}

func (m *mmapWriteFile) Size() int {
	return int(m.size)
}

func (m *mmapWriteFile) Sync() error {
	return m.m.Flush()
}

func (m *mmapWriteFile) Close() error {
	if err := m.m.Unmap(); err != nil {
		return fmt.Errorf("unmap %s error %s", m.name, err.Error())
	}

	if err := m.f.Truncate(m.offset); err != nil {
		return fmt.Errorf("close truncate %s error %s", m.name, err.Error())
	}

	if err := m.f.Close(); err != nil {
		return fmt.Errorf("close %s error %s", m.name, err.Error())
	}

	return nil
}

func (m *mmapWriteFile) Write(b []byte) (n int, err error) {
	extra := int64(len(b)) - (m.size - m.offset)
	if extra > 0 {
		newSize := m.size + extra + m.size/10
		if err = m.Truncate(newSize); err != nil {
			return
		}
		m.size = newSize
	}

	n = copy(m.m[m.offset:], b)
	if n != len(b) {
		return 0, io.ErrShortWrite
	}

	m.offset += int64(len(b))
	return len(b), nil
}

func (m *mmapWriteFile) ReadAt(buf []byte, offset int64) (int, error) {
	if offset > m.offset {
		return 0, fmt.Errorf("invalid offset %d", offset)
	}

	n := copy(buf, m.m[offset:m.offset])
	if n != len(buf) {
		return n, io.ErrUnexpectedEOF
	}

	return n, nil
}

func (m *mmapWriteFile) Truncate(size int64) error {
	var err error
	if err = m.m.Unmap(); err != nil {
		return err
	}

	if err = m.f.Truncate(size); err != nil {
		return err
	}

	if m.m, err = mmap.Map(m.f, mmap.RDWR, 0); err != nil {
		return err
	}

	m.size = size
	if m.offset > m.size {
		m.offset = m.size
	}
	return nil
}

func (m *mmapWriteFile) SetOffset(o int64) {
	m.offset = o
}

func (m *mmapWriteFile) Offset() int64 {
	return m.offset
}

func (m *mmapWriteFile) Name() string {
	return m.name
}

type mmapReadFile struct {
	readFile

	f    *os.File
	m    mmap.MMap
	name string
}

func newMmapReadFile(name string) (readFile, error) {
	m := new(mmapReadFile)

	m.name = name

	var err error
	m.f, err = os.Open(name)
	if err != nil {
		return nil, err
	}

	m.m, err = mmap.Map(m.f, mmap.RDONLY, 0)
	return m, err
}

func (m *mmapReadFile) ReadAt(buf []byte, offset int64) (int, error) {
	if int64(offset) > int64(len(m.m)) {
		return 0, fmt.Errorf("invalid offset %d", offset)
	}

	n := copy(buf, m.m[offset:])
	if n != len(buf) {
		return n, io.ErrUnexpectedEOF
	}

	return n, nil
}

func (m *mmapReadFile) Close() error {
	if m.m != nil {
		if err := m.m.Unmap(); err != nil {
			log.Errorf("unmap %s error %s", m.name, err.Error())
		}
		m.m = nil
	}

	if m.f != nil {
		if err := m.f.Close(); err != nil {
			log.Errorf("close %s error %s", m.name, err.Error())
		}
		m.f = nil
	}

	return nil
}

func (m *mmapReadFile) Size() int {
	return len(m.m)
}

func (m *mmapReadFile) Name() string {
	return m.name
}

/////////////////////////////////////

func newWriteFile(useMmap bool, name string, size int64) (writeFile, error) {
	if useMmap {
		return newMmapWriteFile(name, size)
	} else {
		return newRawWriteFile(name, size)
	}
}

func newReadFile(useMmap bool, name string) (readFile, error) {
	if useMmap {
		return newMmapReadFile(name)
	} else {
		return newRawReadFile(name)
	}
}
