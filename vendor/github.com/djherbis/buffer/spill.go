package buffer

import (
	"encoding/gob"
	"io"
	"io/ioutil"
	"math"
)

type spill struct {
	Buffer
	Spiller io.Writer
}

// NewSpill returns a Buffer which writes data to w when there's an error
// writing to buf. Such as when buf is full, or the disk is full, etc.
func NewSpill(buf Buffer, w io.Writer) Buffer {
	if w == nil {
		w = ioutil.Discard
	}
	return &spill{
		Buffer:  buf,
		Spiller: w,
	}
}

func (buf *spill) Cap() int64 {
	return math.MaxInt64
}

func (buf *spill) Write(p []byte) (n int, err error) {
	if n, err = buf.Buffer.Write(p); err != nil {
		m, err := buf.Spiller.Write(p[n:])
		return m + n, err
	}
	return len(p), nil
}

func init() {
	gob.Register(&spill{})
}
