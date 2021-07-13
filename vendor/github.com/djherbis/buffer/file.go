package buffer

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/djherbis/buffer/wrapio"
)

// File is used as the backing resource for a the NewFile BufferAt.
type File interface {
	Name() string
	Stat() (fi os.FileInfo, err error)
	io.ReaderAt
	io.WriterAt
	Close() error
}

type fileBuffer struct {
	file File
	*wrapio.Wrapper
}

// NewFile returns a new BufferAt backed by "file" with max-size N.
func NewFile(N int64, file File) BufferAt {
	return &fileBuffer{
		file:    file,
		Wrapper: wrapio.NewWrapper(file, 0, 0, N),
	}
}

func init() {
	gob.Register(&fileBuffer{})
}

func (buf *fileBuffer) MarshalBinary() ([]byte, error) {
	fullpath, err := filepath.Abs(filepath.Dir(buf.file.Name()))
	if err != nil {
		return nil, err
	}
	base := filepath.Base(buf.file.Name())
	buf.file.Close()

	buffer := bytes.NewBuffer(nil)
	fmt.Fprintln(buffer, filepath.Join(fullpath, base))
	fmt.Fprintln(buffer, buf.Wrapper.N, buf.Wrapper.L, buf.Wrapper.O)
	return buffer.Bytes(), nil
}

func (buf *fileBuffer) UnmarshalBinary(data []byte) error {
	buffer := bytes.NewBuffer(data)
	var filename string
	var N, L, O int64
	_, err := fmt.Fscanln(buffer, &filename)
	if err != nil {
		return err
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	buf.file = file

	_, err = fmt.Fscanln(buffer, &N, &L, &O)
	buf.Wrapper = wrapio.NewWrapper(file, L, O, N)
	return err
}
