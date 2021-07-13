package packfile

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"math"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/utils/ioutil"
)

// See https://github.com/git/git/blob/49fa3dc76179e04b0833542fa52d0f287a4955ac/delta.h
// https://github.com/git/git/blob/c2c5f6b1e479f2c38e0e01345350620944e3527f/patch-delta.c,
// and https://github.com/tarruda/node-git-core/blob/master/src/js/delta.js
// for details about the delta format.

const deltaSizeMin = 4

// ApplyDelta writes to target the result of applying the modification deltas in delta to base.
func ApplyDelta(target, base plumbing.EncodedObject, delta []byte) (err error) {
	r, err := base.Reader()
	if err != nil {
		return err
	}

	defer ioutil.CheckClose(r, &err)

	w, err := target.Writer()
	if err != nil {
		return err
	}

	defer ioutil.CheckClose(w, &err)

	buf := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)
	buf.Reset()
	_, err = buf.ReadFrom(r)
	if err != nil {
		return err
	}
	src := buf.Bytes()

	dst := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(dst)
	dst.Reset()
	err = patchDelta(dst, src, delta)
	if err != nil {
		return err
	}

	target.SetSize(int64(dst.Len()))

	b := byteSlicePool.Get().([]byte)
	_, err = io.CopyBuffer(w, dst, b)
	byteSlicePool.Put(b)
	return err
}

var (
	ErrInvalidDelta = errors.New("invalid delta")
	ErrDeltaCmd     = errors.New("wrong delta command")
)

// PatchDelta returns the result of applying the modification deltas in delta to src.
// An error will be returned if delta is corrupted (ErrDeltaLen) or an action command
// is not copy from source or copy from delta (ErrDeltaCmd).
func PatchDelta(src, delta []byte) ([]byte, error) {
	b := &bytes.Buffer{}
	if err := patchDelta(b, src, delta); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func ReaderFromDelta(base plumbing.EncodedObject, deltaRC io.Reader) (io.ReadCloser, error) {
	deltaBuf := bufio.NewReaderSize(deltaRC, 1024)
	srcSz, err := decodeLEB128ByteReader(deltaBuf)
	if err != nil {
		if err == io.EOF {
			return nil, ErrInvalidDelta
		}
		return nil, err
	}
	if srcSz != uint(base.Size()) {
		return nil, ErrInvalidDelta
	}

	targetSz, err := decodeLEB128ByteReader(deltaBuf)
	if err != nil {
		if err == io.EOF {
			return nil, ErrInvalidDelta
		}
		return nil, err
	}
	remainingTargetSz := targetSz

	dstRd, dstWr := io.Pipe()

	go func() {
		baseRd, err := base.Reader()
		if err != nil {
			_ = dstWr.CloseWithError(ErrInvalidDelta)
			return
		}
		defer baseRd.Close()

		baseBuf := bufio.NewReader(baseRd)
		basePos := uint(0)

		for {
			cmd, err := deltaBuf.ReadByte()
			if err == io.EOF {
				_ = dstWr.CloseWithError(ErrInvalidDelta)
				return
			}
			if err != nil {
				_ = dstWr.CloseWithError(err)
				return
			}

			if isCopyFromSrc(cmd) {
				offset, err := decodeOffsetByteReader(cmd, deltaBuf)
				if err != nil {
					_ = dstWr.CloseWithError(err)
					return
				}
				sz, err := decodeSizeByteReader(cmd, deltaBuf)
				if err != nil {
					_ = dstWr.CloseWithError(err)
					return
				}

				if invalidSize(sz, targetSz) ||
					invalidOffsetSize(offset, sz, srcSz) {
					_ = dstWr.Close()
					return
				}

				discard := offset - basePos
				if basePos > offset {
					_ = baseRd.Close()
					baseRd, err = base.Reader()
					if err != nil {
						_ = dstWr.CloseWithError(ErrInvalidDelta)
						return
					}
					baseBuf.Reset(baseRd)
					discard = offset
				}
				for discard > math.MaxInt32 {
					n, err := baseBuf.Discard(math.MaxInt32)
					if err != nil {
						_ = dstWr.CloseWithError(err)
						return
					}
					basePos += uint(n)
					discard -= uint(n)
				}
				for discard > 0 {
					n, err := baseBuf.Discard(int(discard))
					if err != nil {
						_ = dstWr.CloseWithError(err)
						return
					}
					basePos += uint(n)
					discard -= uint(n)
				}
				if _, err := io.Copy(dstWr, io.LimitReader(baseBuf, int64(sz))); err != nil {
					_ = dstWr.CloseWithError(err)
					return
				}
				remainingTargetSz -= sz
				basePos += sz
			} else if isCopyFromDelta(cmd) {
				sz := uint(cmd) // cmd is the size itself
				if invalidSize(sz, targetSz) {
					_ = dstWr.CloseWithError(ErrInvalidDelta)
					return
				}
				if _, err := io.Copy(dstWr, io.LimitReader(deltaBuf, int64(sz))); err != nil {
					_ = dstWr.CloseWithError(err)
					return
				}

				remainingTargetSz -= sz
			} else {
				_ = dstWr.CloseWithError(ErrDeltaCmd)
				return
			}
			if remainingTargetSz <= 0 {
				_ = dstWr.Close()
				return
			}
		}
	}()

	return dstRd, nil
}

func patchDelta(dst *bytes.Buffer, src, delta []byte) error {
	if len(delta) < deltaSizeMin {
		return ErrInvalidDelta
	}

	srcSz, delta := decodeLEB128(delta)
	if srcSz != uint(len(src)) {
		return ErrInvalidDelta
	}

	targetSz, delta := decodeLEB128(delta)
	remainingTargetSz := targetSz

	var cmd byte
	dst.Grow(int(targetSz))
	for {
		if len(delta) == 0 {
			return ErrInvalidDelta
		}

		cmd = delta[0]
		delta = delta[1:]
		if isCopyFromSrc(cmd) {
			var offset, sz uint
			var err error
			offset, delta, err = decodeOffset(cmd, delta)
			if err != nil {
				return err
			}

			sz, delta, err = decodeSize(cmd, delta)
			if err != nil {
				return err
			}

			if invalidSize(sz, targetSz) ||
				invalidOffsetSize(offset, sz, srcSz) {
				break
			}
			dst.Write(src[offset : offset+sz])
			remainingTargetSz -= sz
		} else if isCopyFromDelta(cmd) {
			sz := uint(cmd) // cmd is the size itself
			if invalidSize(sz, targetSz) {
				return ErrInvalidDelta
			}

			if uint(len(delta)) < sz {
				return ErrInvalidDelta
			}

			dst.Write(delta[0:sz])
			remainingTargetSz -= sz
			delta = delta[sz:]
		} else {
			return ErrDeltaCmd
		}

		if remainingTargetSz <= 0 {
			break
		}
	}

	return nil
}

// Decodes a number encoded as an unsigned LEB128 at the start of some
// binary data and returns the decoded number and the rest of the
// stream.
//
// This must be called twice on the delta data buffer, first to get the
// expected source buffer size, and again to get the target buffer size.
func decodeLEB128(input []byte) (uint, []byte) {
	var num, sz uint
	var b byte
	for {
		b = input[sz]
		num |= (uint(b) & payload) << (sz * 7) // concats 7 bits chunks
		sz++

		if uint(b)&continuation == 0 || sz == uint(len(input)) {
			break
		}
	}

	return num, input[sz:]
}

func decodeLEB128ByteReader(input io.ByteReader) (uint, error) {
	var num, sz uint
	for {
		b, err := input.ReadByte()
		if err != nil {
			return 0, err
		}

		num |= (uint(b) & payload) << (sz * 7) // concats 7 bits chunks
		sz++

		if uint(b)&continuation == 0 {
			break
		}
	}

	return num, nil
}

const (
	payload      = 0x7f // 0111 1111
	continuation = 0x80 // 1000 0000
)

func isCopyFromSrc(cmd byte) bool {
	return (cmd & 0x80) != 0
}

func isCopyFromDelta(cmd byte) bool {
	return (cmd&0x80) == 0 && cmd != 0
}

func decodeOffsetByteReader(cmd byte, delta io.ByteReader) (uint, error) {
	var offset uint
	if (cmd & 0x01) != 0 {
		next, err := delta.ReadByte()
		if err != nil {
			return 0, err
		}
		offset = uint(next)
	}
	if (cmd & 0x02) != 0 {
		next, err := delta.ReadByte()
		if err != nil {
			return 0, err
		}
		offset |= uint(next) << 8
	}
	if (cmd & 0x04) != 0 {
		next, err := delta.ReadByte()
		if err != nil {
			return 0, err
		}
		offset |= uint(next) << 16
	}
	if (cmd & 0x08) != 0 {
		next, err := delta.ReadByte()
		if err != nil {
			return 0, err
		}
		offset |= uint(next) << 24
	}

	return offset, nil
}

func decodeOffset(cmd byte, delta []byte) (uint, []byte, error) {
	var offset uint
	if (cmd & 0x01) != 0 {
		if len(delta) == 0 {
			return 0, nil, ErrInvalidDelta
		}
		offset = uint(delta[0])
		delta = delta[1:]
	}
	if (cmd & 0x02) != 0 {
		if len(delta) == 0 {
			return 0, nil, ErrInvalidDelta
		}
		offset |= uint(delta[0]) << 8
		delta = delta[1:]
	}
	if (cmd & 0x04) != 0 {
		if len(delta) == 0 {
			return 0, nil, ErrInvalidDelta
		}
		offset |= uint(delta[0]) << 16
		delta = delta[1:]
	}
	if (cmd & 0x08) != 0 {
		if len(delta) == 0 {
			return 0, nil, ErrInvalidDelta
		}
		offset |= uint(delta[0]) << 24
		delta = delta[1:]
	}

	return offset, delta, nil
}

func decodeSizeByteReader(cmd byte, delta io.ByteReader) (uint, error) {
	var sz uint
	if (cmd & 0x10) != 0 {
		next, err := delta.ReadByte()
		if err != nil {
			return 0, err
		}
		sz = uint(next)
	}
	if (cmd & 0x20) != 0 {
		next, err := delta.ReadByte()
		if err != nil {
			return 0, err
		}
		sz |= uint(next) << 8
	}
	if (cmd & 0x40) != 0 {
		next, err := delta.ReadByte()
		if err != nil {
			return 0, err
		}
		sz |= uint(next) << 16
	}
	if sz == 0 {
		sz = 0x10000
	}

	return sz, nil
}

func decodeSize(cmd byte, delta []byte) (uint, []byte, error) {
	var sz uint
	if (cmd & 0x10) != 0 {
		if len(delta) == 0 {
			return 0, nil, ErrInvalidDelta
		}
		sz = uint(delta[0])
		delta = delta[1:]
	}
	if (cmd & 0x20) != 0 {
		if len(delta) == 0 {
			return 0, nil, ErrInvalidDelta
		}
		sz |= uint(delta[0]) << 8
		delta = delta[1:]
	}
	if (cmd & 0x40) != 0 {
		if len(delta) == 0 {
			return 0, nil, ErrInvalidDelta
		}
		sz |= uint(delta[0]) << 16
		delta = delta[1:]
	}
	if sz == 0 {
		sz = 0x10000
	}

	return sz, delta, nil
}

func invalidSize(sz, targetSz uint) bool {
	return sz > targetSz
}

func invalidOffsetSize(offset, sz, srcSz uint) bool {
	return sumOverflows(offset, sz) ||
		offset+sz > srcSz
}

func sumOverflows(a, b uint) bool {
	return a+b < a
}
