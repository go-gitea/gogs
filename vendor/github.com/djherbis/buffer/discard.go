package buffer

import (
	"encoding/gob"
	"io"
	"io/ioutil"
	"math"
)

type discard struct{}

// Discard is a Buffer which writes to ioutil.Discard and read's return 0, io.EOF.
// All of its methods are concurrent safe.
var Discard Buffer = discard{}

func (buf discard) Len() int64 {
	return 0
}

func (buf discard) Cap() int64 {
	return math.MaxInt64
}

func (buf discard) Reset() {}

func (buf discard) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (buf discard) Write(p []byte) (int, error) {
	return ioutil.Discard.Write(p)
}

func init() {
	gob.Register(&discard{})
}
