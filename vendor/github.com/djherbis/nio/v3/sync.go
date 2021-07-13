package nio

import (
	"io"
	"sync"
)

// PipeReader is the read half of the pipe.
type PipeReader struct {
	*bufpipe
}

// CloseWithError closes the reader; subsequent writes to the write half of the pipe will return the error err.
func (r *PipeReader) CloseWithError(err error) error {
	if err == nil {
		err = io.ErrClosedPipe
	}
	r.bufpipe.l.Lock()
	defer r.bufpipe.l.Unlock()
	if r.bufpipe.rerr == nil {
		r.bufpipe.rerr = err
		r.bufpipe.rwait.Signal()
		r.bufpipe.wwait.Signal()
	}
	return nil
}

// Close closes the reader; subsequent writes to the write half of the pipe will return the error io.ErrClosedPipe.
func (r *PipeReader) Close() error {
	return r.CloseWithError(nil)
}

// A PipeWriter is the write half of a pipe.
type PipeWriter struct {
	*bufpipe
}

// CloseWithError closes the writer; once the buffer is empty subsequent reads from the read half of the pipe will return
// no bytes and the error err, or io.EOF if err is nil. CloseWithError always returns nil.
func (w *PipeWriter) CloseWithError(err error) error {
	if err == nil {
		err = io.EOF
	}
	w.bufpipe.l.Lock()
	defer w.bufpipe.l.Unlock()
	if w.bufpipe.werr == nil {
		w.bufpipe.werr = err
		w.bufpipe.rwait.Signal()
		w.bufpipe.wwait.Signal()
	}
	return nil
}

// Close closes the writer; once the buffer is empty subsequent reads from the read half of the pipe will return
// no bytes and io.EOF after all the buffer has been read. CloseWithError always returns nil.
func (w *PipeWriter) Close() error {
	return w.CloseWithError(nil)
}

type bufpipe struct {
	rl    sync.Mutex
	wl    sync.Mutex
	l     sync.Mutex
	rwait sync.Cond
	wwait sync.Cond
	b     Buffer
	rerr  error // if reader closed, error to give writes
	werr  error // if writer closed, error to give reads
}

func newBufferedPipe(buf Buffer) *bufpipe {
	s := &bufpipe{
		b: buf,
	}
	s.rwait.L = &s.l
	s.wwait.L = &s.l
	return s
}

func empty(buf Buffer) bool {
	return buf.Len() == 0
}

func gap(buf Buffer) int64 {
	return buf.Cap() - buf.Len()
}

func (r *PipeReader) Read(p []byte) (n int, err error) {
	r.rl.Lock()
	defer r.rl.Unlock()

	r.l.Lock()
	defer r.wwait.Signal()
	defer r.l.Unlock()

	for empty(r.b) {
		if r.rerr != nil {
			return 0, io.ErrClosedPipe
		}

		if r.werr != nil {
			return 0, r.werr
		}

		r.wwait.Signal()
		r.rwait.Wait()
	}

	n, err = r.b.Read(p)
	if err == io.EOF {
		err = nil
	}

	return n, err
}

func (w *PipeWriter) Write(p []byte) (int, error) {
	var m int
	var n, space int64
	var err error
	sliceLen := int64(len(p))

	w.wl.Lock()
	defer w.wl.Unlock()

	w.l.Lock()
	defer w.rwait.Signal()
	defer w.l.Unlock()

	if w.werr != nil {
		return 0, io.ErrClosedPipe
	}

	// while there is data to write
	for writeLen := sliceLen; writeLen > 0 && err == nil; writeLen = sliceLen - n {

		// wait for some buffer space to become available (while no errs)
		for space = gap(w.b); space == 0 && w.rerr == nil && w.werr == nil; space = gap(w.b) {
			w.rwait.Signal()
			w.wwait.Wait()
		}

		if w.rerr != nil {
			err = w.rerr
			break
		}

		if w.werr != nil {
			err = io.ErrClosedPipe
			break
		}

		// space > 0, and locked

		var nn int64
		if space < writeLen {
			// => writeLen - space > 0
			// => (sliceLen - n) - space > 0
			// => sliceLen > n + space
			// nn is safe to use for p[:nn]
			nn = n + space
		} else {
			nn = sliceLen
		}

		m, err = w.b.Write(p[n:nn])
		n += int64(m)

		// one of the following cases has occurred:
		// 1. done writing -> writeLen == 0
		// 2. ran out of buffer space -> gap(w.b) == 0
		// 3. an error occurred err != nil
		// all of these cases are handled at the top of this loop
	}

	return int(n), err
}
