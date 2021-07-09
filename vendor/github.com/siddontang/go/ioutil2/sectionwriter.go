package ioutil2

import (
	"errors"
	"io"
)

var ErrExceedLimit = errors.New("write exceed limit")

func NewSectionWriter(w io.WriterAt, off int64, n int64) *SectionWriter {
	return &SectionWriter{w, off, off, off + n}
}

type SectionWriter struct {
	w     io.WriterAt
	base  int64
	off   int64
	limit int64
}

func (s *SectionWriter) Write(p []byte) (n int, err error) {
	if s.off >= s.limit {
		return 0, ErrExceedLimit
	}

	if max := s.limit - s.off; int64(len(p)) > max {
		return 0, ErrExceedLimit
	}

	n, err = s.w.WriteAt(p, s.off)
	s.off += int64(n)
	return
}

var errWhence = errors.New("Seek: invalid whence")
var errOffset = errors.New("Seek: invalid offset")

func (s *SectionWriter) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	default:
		return 0, errWhence
	case 0:
		offset += s.base
	case 1:
		offset += s.off
	case 2:
		offset += s.limit
	}
	if offset < s.base {
		return 0, errOffset
	}
	s.off = offset
	return offset - s.base, nil
}

func (s *SectionWriter) WriteAt(p []byte, off int64) (n int, err error) {
	if off < 0 || off >= s.limit-s.base {
		return 0, errOffset
	}
	off += s.base
	if max := s.limit - off; int64(len(p)) > max {
		return 0, ErrExceedLimit
	}

	return s.w.WriteAt(p, off)
}

// Size returns the size of the section in bytes.
func (s *SectionWriter) Size() int64 { return s.limit - s.base }
