package rdb

import (
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"math"
	"strconv"

	"github.com/cupcake/rdb/crc64"
)

const Version = 6

type Encoder struct {
	w   io.Writer
	crc hash.Hash
}

func NewEncoder(w io.Writer) *Encoder {
	e := &Encoder{crc: crc64.New()}
	e.w = io.MultiWriter(w, e.crc)
	return e
}

func (e *Encoder) EncodeHeader() error {
	_, err := fmt.Fprintf(e.w, "REDIS%04d", Version)
	return err
}

func (e *Encoder) EncodeFooter() error {
	e.w.Write([]byte{rdbFlagEOF})
	_, err := e.w.Write(e.crc.Sum(nil))
	return err
}

func (e *Encoder) EncodeDumpFooter() error {
	binary.Write(e.w, binary.LittleEndian, uint16(Version))
	_, err := e.w.Write(e.crc.Sum(nil))
	return err
}

func (e *Encoder) EncodeDatabase(n int) error {
	e.w.Write([]byte{rdbFlagSelectDB})
	return e.EncodeLength(uint32(n))
}

func (e *Encoder) EncodeExpiry(expiry uint64) error {
	b := make([]byte, 9)
	b[0] = rdbFlagExpiryMS
	binary.LittleEndian.PutUint64(b[1:], expiry)
	_, err := e.w.Write(b)
	return err
}

func (e *Encoder) EncodeType(v ValueType) error {
	_, err := e.w.Write([]byte{byte(v)})
	return err
}

func (e *Encoder) EncodeString(s []byte) error {
	written, err := e.encodeIntString(s)
	if written {
		return err
	}
	e.EncodeLength(uint32(len(s)))
	_, err = e.w.Write(s)
	return err
}

func (e *Encoder) EncodeLength(l uint32) (err error) {
	switch {
	case l < 1<<6:
		_, err = e.w.Write([]byte{byte(l)})
	case l < 1<<14:
		_, err = e.w.Write([]byte{byte(l>>8) | rdb14bitLen<<6, byte(l)})
	default:
		b := make([]byte, 5)
		b[0] = rdb32bitLen << 6
		binary.BigEndian.PutUint32(b[1:], l)
		_, err = e.w.Write(b)
	}
	return
}

func (e *Encoder) EncodeFloat(f float64) (err error) {
	switch {
	case math.IsNaN(f):
		_, err = e.w.Write([]byte{253})
	case math.IsInf(f, 1):
		_, err = e.w.Write([]byte{254})
	case math.IsInf(f, -1):
		_, err = e.w.Write([]byte{255})
	default:
		b := []byte(strconv.FormatFloat(f, 'g', 17, 64))
		e.w.Write([]byte{byte(len(b))})
		_, err = e.w.Write(b)
	}
	return
}

func (e *Encoder) encodeIntString(b []byte) (written bool, err error) {
	s := string(b)
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return
	}
	// if the stringified parsed int isn't exactly the same, we can't encode it as an int
	if s != strconv.FormatInt(i, 10) {
		return
	}
	switch {
	case i >= math.MinInt8 && i <= math.MaxInt8:
		_, err = e.w.Write([]byte{rdbEncVal << 6, byte(int8(i))})
	case i >= math.MinInt16 && i <= math.MaxInt16:
		b := make([]byte, 3)
		b[0] = rdbEncVal<<6 | rdbEncInt16
		binary.LittleEndian.PutUint16(b[1:], uint16(int16(i)))
		_, err = e.w.Write(b)
	case i >= math.MinInt32 && i <= math.MaxInt32:
		b := make([]byte, 5)
		b[0] = rdbEncVal<<6 | rdbEncInt32
		binary.LittleEndian.PutUint32(b[1:], uint32(int32(i)))
		_, err = e.w.Write(b)
	default:
		return
	}
	return true, err
}
