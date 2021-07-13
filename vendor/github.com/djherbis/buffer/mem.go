package buffer

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"

	"github.com/djherbis/buffer/limio"
)

type memory struct {
	N int64
	*bytes.Buffer
}

// New returns a new in memory BufferAt with max size N.
// It's backed by a bytes.Buffer.
func New(n int64) BufferAt {
	return &memory{
		N:      n,
		Buffer: bytes.NewBuffer(nil),
	}
}

func (buf *memory) Cap() int64 {
	return buf.N
}

func (buf *memory) Len() int64 {
	return int64(buf.Buffer.Len())
}

func (buf *memory) Write(p []byte) (n int, err error) {
	return limio.LimitWriter(buf.Buffer, Gap(buf)).Write(p)
}

func (buf *memory) WriteAt(p []byte, off int64) (n int, err error) {
	if off > buf.Len() {
		return 0, io.ErrShortWrite
	} else if len64(p)+off <= buf.Len() {
		d := buf.Bytes()[off:]
		return copy(d, p), nil
	} else {
		d := buf.Bytes()[off:]
		n = copy(d, p)
		m, err := buf.Write(p[n:])
		return n + m, err
	}
}

func (buf *memory) ReadAt(p []byte, off int64) (n int, err error) {
	return bytes.NewReader(buf.Bytes()).ReadAt(p, off)
}

func (buf *memory) Read(p []byte) (n int, err error) {
	return io.LimitReader(buf.Buffer, buf.Len()).Read(p)
}

func (buf *memory) ReadFrom(r io.Reader) (n int64, err error) {
	return buf.Buffer.ReadFrom(io.LimitReader(r, Gap(buf)))
}

func init() {
	gob.Register(&memory{})
}

func (buf *memory) MarshalBinary() ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintln(&b, buf.N)
	b.Write(buf.Bytes())
	return b.Bytes(), nil
}

func (buf *memory) UnmarshalBinary(bindata []byte) error {
	data := make([]byte, len(bindata))
	copy(data, bindata)
	b := bytes.NewBuffer(data)
	_, err := fmt.Fscanln(b, &buf.N)
	buf.Buffer = bytes.NewBuffer(b.Bytes())
	return err
}
