// Copyright 2014 Wandoujia Inc. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package rdb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strconv"
)

const (
	Version = 6
)

const (
	rdbTypeString = 0
	rdbTypeList   = 1
	rdbTypeSet    = 2
	rdbTypeZSet   = 3
	rdbTypeHash   = 4

	rdbTypeHashZipmap  = 9
	rdbTypeListZiplist = 10
	rdbTypeSetIntset   = 11
	rdbTypeZSetZiplist = 12
	rdbTypeHashZiplist = 13

	rdbFlagExpiryMS = 0xfc
	rdbFlagExpiry   = 0xfd
	rdbFlagSelectDB = 0xfe
	rdbFlagEOF      = 0xff
)

const (
	rdb6bitLen  = 0
	rdb14bitLen = 1
	rdb32bitLen = 2
	rdbEncVal   = 3

	rdbEncInt8  = 0
	rdbEncInt16 = 1
	rdbEncInt32 = 2
	rdbEncLZF   = 3

	rdbZiplist6bitlenString  = 0
	rdbZiplist14bitlenString = 1
	rdbZiplist32bitlenString = 2

	rdbZiplistInt16 = 0xc0
	rdbZiplistInt32 = 0xd0
	rdbZiplistInt64 = 0xe0
	rdbZiplistInt24 = 0xf0
	rdbZiplistInt8  = 0xfe
	rdbZiplistInt4  = 15
)

type rdbReader struct {
	raw   io.Reader
	buf   [8]byte
	nread int64
}

func newRdbReader(r io.Reader) *rdbReader {
	return &rdbReader{raw: r}
}

func (r *rdbReader) Read(p []byte) (int, error) {
	n, err := r.raw.Read(p)
	r.nread += int64(n)
	return n, err
}

func (r *rdbReader) offset() int64 {
	return r.nread
}

func (r *rdbReader) readObject(otype byte) ([]byte, error) {
	var b bytes.Buffer
	r = newRdbReader(io.TeeReader(r, &b))
	switch otype {
	default:
		return nil, fmt.Errorf("unknown object-type %02x", otype)
	case rdbTypeHashZipmap:
		fallthrough
	case rdbTypeListZiplist:
		fallthrough
	case rdbTypeSetIntset:
		fallthrough
	case rdbTypeZSetZiplist:
		fallthrough
	case rdbTypeHashZiplist:
		fallthrough
	case rdbTypeString:
		if _, err := r.readString(); err != nil {
			return nil, err
		}
	case rdbTypeList, rdbTypeSet:
		if n, err := r.readLength(); err != nil {
			return nil, err
		} else {
			for i := 0; i < int(n); i++ {
				if _, err := r.readString(); err != nil {
					return nil, err
				}
			}
		}
	case rdbTypeZSet:
		if n, err := r.readLength(); err != nil {
			return nil, err
		} else {
			for i := 0; i < int(n); i++ {
				if _, err := r.readString(); err != nil {
					return nil, err
				}
				if _, err := r.readFloat(); err != nil {
					return nil, err
				}
			}
		}
	case rdbTypeHash:
		if n, err := r.readLength(); err != nil {
			return nil, err
		} else {
			for i := 0; i < int(n); i++ {
				if _, err := r.readString(); err != nil {
					return nil, err
				}
				if _, err := r.readString(); err != nil {
					return nil, err
				}
			}
		}
	}
	return b.Bytes(), nil
}

func (r *rdbReader) readString() ([]byte, error) {
	length, encoded, err := r.readEncodedLength()
	if err != nil {
		return nil, err
	}
	if !encoded {
		return r.readBytes(int(length))
	}
	switch t := uint8(length); t {
	default:
		return nil, fmt.Errorf("invalid encoded-string %02x", t)
	case rdbEncInt8:
		i, err := r.readInt8()
		return []byte(strconv.FormatInt(int64(i), 10)), err
	case rdbEncInt16:
		i, err := r.readInt16()
		return []byte(strconv.FormatInt(int64(i), 10)), err
	case rdbEncInt32:
		i, err := r.readInt32()
		return []byte(strconv.FormatInt(int64(i), 10)), err
	case rdbEncLZF:
		var inlen, outlen uint32
		if inlen, err = r.readLength(); err != nil {
			return nil, err
		}
		if outlen, err = r.readLength(); err != nil {
			return nil, err
		}
		if in, err := r.readBytes(int(inlen)); err != nil {
			return nil, err
		} else {
			return lzfDecompress(in, int(outlen))
		}
	}
}

func (r *rdbReader) readEncodedLength() (length uint32, encoded bool, err error) {
	var u uint8
	if u, err = r.readUint8(); err != nil {
		return
	}
	length = uint32(u & 0x3f)
	switch u >> 6 {
	case rdb6bitLen:
	case rdb14bitLen:
		u, err = r.readUint8()
		length = (length << 8) + uint32(u)
	case rdbEncVal:
		encoded = true
	default:
		length, err = r.readUint32BigEndian()
	}
	return
}

func (r *rdbReader) readLength() (uint32, error) {
	length, encoded, err := r.readEncodedLength()
	if err == nil && encoded {
		err = fmt.Errorf("encoded-length")
	}
	return length, err
}

func (r *rdbReader) readFloat() (float64, error) {
	u, err := r.readUint8()
	if err != nil {
		return 0, err
	}
	switch u {
	case 253:
		return math.NaN(), nil
	case 254:
		return math.Inf(0), nil
	case 255:
		return math.Inf(-1), nil
	default:
		if b, err := r.readBytes(int(u)); err != nil {
			return 0, err
		} else {
			v, err := strconv.ParseFloat(string(b), 64)
			return v, err
		}
	}
}

func (r *rdbReader) readByte() (byte, error) {
	b := r.buf[:1]
	_, err := r.Read(b)
	return b[0], err
}

func (r *rdbReader) readFull(p []byte) error {
	_, err := io.ReadFull(r, p)
	return err
}

func (r *rdbReader) readBytes(n int) ([]byte, error) {
	p := make([]byte, n)
	return p, r.readFull(p)
}

func (r *rdbReader) readUint8() (uint8, error) {
	b, err := r.readByte()
	return uint8(b), err
}

func (r *rdbReader) readUint16() (uint16, error) {
	b := r.buf[:2]
	err := r.readFull(b)
	return binary.LittleEndian.Uint16(b), err
}

func (r *rdbReader) readUint32() (uint32, error) {
	b := r.buf[:4]
	err := r.readFull(b)
	return binary.LittleEndian.Uint32(b), err
}

func (r *rdbReader) readUint64() (uint64, error) {
	b := r.buf[:8]
	err := r.readFull(b)
	return binary.LittleEndian.Uint64(b), err
}

func (r *rdbReader) readUint32BigEndian() (uint32, error) {
	b := r.buf[:4]
	err := r.readFull(b)
	return binary.BigEndian.Uint32(b), err
}

func (r *rdbReader) readInt8() (int8, error) {
	u, err := r.readUint8()
	return int8(u), err
}

func (r *rdbReader) readInt16() (int16, error) {
	u, err := r.readUint16()
	return int16(u), err
}

func (r *rdbReader) readInt32() (int32, error) {
	u, err := r.readUint32()
	return int32(u), err
}

func (r *rdbReader) readInt64() (int64, error) {
	u, err := r.readUint64()
	return int64(u), err
}

func (r *rdbReader) readInt32BigEndian() (int32, error) {
	u, err := r.readUint32BigEndian()
	return int32(u), err
}

func lzfDecompress(in []byte, outlen int) (out []byte, err error) {
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("decompress exception: %v", x)
		}
	}()
	out = make([]byte, outlen)
	i, o := 0, 0
	for i < len(in) {
		ctrl := int(in[i])
		i++
		if ctrl < 32 {
			for x := 0; x <= ctrl; x++ {
				out[o] = in[i]
				i++
				o++
			}
		} else {
			length := ctrl >> 5
			if length == 7 {
				length = length + int(in[i])
				i++
			}
			ref := o - ((ctrl & 0x1f) << 8) - int(in[i]) - 1
			i++
			for x := 0; x <= length+1; x++ {
				out[o] = out[ref]
				ref++
				o++
			}
		}
	}
	if o != outlen {
		return nil, fmt.Errorf("decompress length is %d != expected %d", o, outlen)
	}
	return out, nil
}
