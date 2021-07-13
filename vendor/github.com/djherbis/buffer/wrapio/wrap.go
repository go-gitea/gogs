package wrapio

import "io"

// DoerAt is a common interface for wrappers WriteAt or ReadAt functions
type DoerAt interface {
	DoAt([]byte, int64) (int, error)
}

// DoAtFunc is implemented by ReadAt/WriteAt
type DoAtFunc func([]byte, int64) (int, error)

type wrapper struct {
	off    int64
	wrapAt int64
	doat   DoAtFunc
}

func (w *wrapper) Offset() int64 {
	return w.off
}

func (w *wrapper) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		w.off = offset
	case 1:
		w.off += offset
	case 2:
		w.off = (w.wrapAt + offset)
	}
	w.off %= w.wrapAt
	return w.off, nil
}

func (w *wrapper) DoAt(p []byte, off int64) (n int, err error) {
	return w.doat(p, off)
}

// WrapWriter wraps writes around a section of data.
type WrapWriter struct {
	*wrapper
}

// NewWrapWriter creates a WrapWriter starting at offset off, and wrapping at offset wrapAt.
func NewWrapWriter(w io.WriterAt, off int64, wrapAt int64) *WrapWriter {
	return &WrapWriter{
		&wrapper{
			doat:   w.WriteAt,
			off:    (off % wrapAt),
			wrapAt: wrapAt,
		},
	}
}

// Write writes p starting at the current offset, wrapping when it reaches the end.
// The current offset is shifted forward by the amount written.
func (w *WrapWriter) Write(p []byte) (n int, err error) {
	n, err = Wrap(w, p, w.off, w.wrapAt)
	w.off = (w.off + int64(n)) % w.wrapAt
	return n, err
}

// WriteAt writes p starting at offset off, wrapping when it reaches the end.
func (w *WrapWriter) WriteAt(p []byte, off int64) (n int, err error) {
	return Wrap(w, p, off, w.wrapAt)
}

// WrapReader wraps reads around a section of data.
type WrapReader struct {
	*wrapper
}

// NewWrapReader creates a WrapReader starting at offset off, and wrapping at offset wrapAt.
func NewWrapReader(r io.ReaderAt, off int64, wrapAt int64) *WrapReader {
	return &WrapReader{
		&wrapper{
			doat:   r.ReadAt,
			off:    (off % wrapAt),
			wrapAt: wrapAt,
		},
	}
}

// Read reads into p starting at the current offset, wrapping if it reaches the end.
// The current offset is shifted forward by the amount read.
func (r *WrapReader) Read(p []byte) (n int, err error) {
	n, err = Wrap(r, p, r.off, r.wrapAt)
	r.off = (r.off + int64(n)) % r.wrapAt
	return n, err
}

// ReadAt reads into p starting at the current offset, wrapping when it reaches the end.
func (r *WrapReader) ReadAt(p []byte, off int64) (n int, err error) {
	return Wrap(r, p, off, r.wrapAt)
}

// maxConsecutiveEmptyActions determines how many consecutive empty reads/writes can occur before giving up
const maxConsecutiveEmptyActions = 100

// Wrap causes an action on an array of bytes (like read/write) to be done from an offset off,
// wrapping at offset wrapAt.
func Wrap(w DoerAt, p []byte, off int64, wrapAt int64) (n int, err error) {
	var m, fails int

	off %= wrapAt

	for len(p) > 0 {

		if off+int64(len(p)) < wrapAt {
			m, err = w.DoAt(p, off)
		} else {
			space := wrapAt - off
			m, err = w.DoAt(p[:space], off)
		}

		if err != nil && err != io.EOF {
			return n + m, err
		}

		switch m {
		case 0:
			fails++
		default:
			fails = 0
		}

		if fails > maxConsecutiveEmptyActions {
			return n + m, io.ErrNoProgress
		}

		n += m
		p = p[m:]
		off += int64(m)
		off %= wrapAt
	}

	return n, err
}
