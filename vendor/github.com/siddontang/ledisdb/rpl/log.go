package rpl

import (
	"bytes"
	"encoding/binary"
	"io"
	"sync"
)

const LogHeadSize = 17

type Log struct {
	ID          uint64
	CreateTime  uint32
	Compression uint8

	Data []byte
}

func (l *Log) HeadSize() int {
	return LogHeadSize
}

func (l *Log) Size() int {
	return l.HeadSize() + len(l.Data)
}

func (l *Log) Marshal() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, l.Size()))
	buf.Reset()

	if err := l.Encode(buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (l *Log) Unmarshal(b []byte) error {
	buf := bytes.NewBuffer(b)

	return l.Decode(buf)
}

var headPool = sync.Pool{
	New: func() interface{} { return make([]byte, LogHeadSize) },
}

func (l *Log) Encode(w io.Writer) error {
	b := headPool.Get().([]byte)
	pos := 0

	binary.BigEndian.PutUint64(b[pos:], l.ID)
	pos += 8
	binary.BigEndian.PutUint32(b[pos:], uint32(l.CreateTime))
	pos += 4
	b[pos] = l.Compression
	pos++
	binary.BigEndian.PutUint32(b[pos:], uint32(len(l.Data)))

	n, err := w.Write(b)
	headPool.Put(b)

	if err != nil {
		return err
	} else if n != LogHeadSize {
		return io.ErrShortWrite
	}

	if n, err = w.Write(l.Data); err != nil {
		return err
	} else if n != len(l.Data) {
		return io.ErrShortWrite
	}
	return nil
}

func (l *Log) Decode(r io.Reader) error {
	length, err := l.DecodeHead(r)
	if err != nil {
		return err
	}

	l.growData(int(length))

	if _, err := io.ReadFull(r, l.Data); err != nil {
		return err
	}

	return nil
}

func (l *Log) DecodeHead(r io.Reader) (uint32, error) {
	buf := headPool.Get().([]byte)

	if _, err := io.ReadFull(r, buf); err != nil {
		headPool.Put(buf)
		return 0, err
	}

	length := l.decodeHeadBuf(buf)

	headPool.Put(buf)

	return length, nil
}

func (l *Log) DecodeAt(r io.ReaderAt, pos int64) error {
	length, err := l.DecodeHeadAt(r, pos)
	if err != nil {
		return err
	}

	l.growData(int(length))
	var n int
	n, err = r.ReadAt(l.Data, pos+int64(LogHeadSize))
	if err == io.EOF && n == len(l.Data) {
		err = nil
	}

	return err
}

func (l *Log) growData(length int) {
	l.Data = l.Data[0:0]

	if cap(l.Data) >= length {
		l.Data = l.Data[0:length]
	} else {
		l.Data = make([]byte, length)
	}
}

func (l *Log) DecodeHeadAt(r io.ReaderAt, pos int64) (uint32, error) {
	buf := headPool.Get().([]byte)

	n, err := r.ReadAt(buf, pos)
	if err != nil && err != io.EOF {
		headPool.Put(buf)

		return 0, err
	}

	length := l.decodeHeadBuf(buf)
	headPool.Put(buf)

	if err == io.EOF && (length != 0 || n != len(buf)) {
		return 0, err
	}

	return length, nil
}

func (l *Log) decodeHeadBuf(buf []byte) uint32 {
	pos := 0
	l.ID = binary.BigEndian.Uint64(buf[pos:])
	pos += 8

	l.CreateTime = binary.BigEndian.Uint32(buf[pos:])
	pos += 4

	l.Compression = uint8(buf[pos])
	pos++

	length := binary.BigEndian.Uint32(buf[pos:])
	return length
}
