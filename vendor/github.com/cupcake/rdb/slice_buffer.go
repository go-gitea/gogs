package rdb

import (
	"errors"
	"io"
)

type sliceBuffer struct {
	s []byte
	i int
}

func newSliceBuffer(s []byte) *sliceBuffer {
	return &sliceBuffer{s, 0}
}

func (s *sliceBuffer) Slice(n int) ([]byte, error) {
	if s.i+n > len(s.s) {
		return nil, io.EOF
	}
	b := s.s[s.i : s.i+n]
	s.i += n
	return b, nil
}

func (s *sliceBuffer) ReadByte() (byte, error) {
	if s.i >= len(s.s) {
		return 0, io.EOF
	}
	b := s.s[s.i]
	s.i++
	return b, nil
}

func (s *sliceBuffer) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	if s.i >= len(s.s) {
		return 0, io.EOF
	}
	n := copy(b, s.s[s.i:])
	s.i += n
	return n, nil
}

func (s *sliceBuffer) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case 0:
		abs = offset
	case 1:
		abs = int64(s.i) + offset
	case 2:
		abs = int64(len(s.s)) + offset
	default:
		return 0, errors.New("invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("negative position")
	}
	if abs >= 1<<31 {
		return 0, errors.New("position out of range")
	}
	s.i = int(abs)
	return abs, nil
}
