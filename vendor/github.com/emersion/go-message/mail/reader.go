package mail

import (
	"container/list"
	"io"
	"strings"

	"github.com/emersion/go-message"
)

// A PartHeader is a mail part header. It contains convenience functions to get
// and set header fields.
type PartHeader interface {
	// Add adds the key, value pair to the header.
	Add(key, value string)
	// Del deletes the values associated with key.
	Del(key string)
	// Get gets the first value associated with the given key. If there are no
	// values associated with the key, Get returns "".
	Get(key string) string
	// Set sets the header entries associated with key to the single element
	// value. It replaces any existing values associated with key.
	Set(key, value string)
}

// A Part is either a mail text or an attachment. Header is either a InlineHeader
// or an AttachmentHeader.
type Part struct {
	Header PartHeader
	Body   io.Reader
}

// A Reader reads a mail message.
type Reader struct {
	Header Header

	e       *message.Entity
	readers *list.List
}

// NewReader creates a new mail reader.
func NewReader(e *message.Entity) *Reader {
	mr := e.MultipartReader()
	if mr == nil {
		// Artificially create a multipart entity
		// With this header, no error will be returned by message.NewMultipart
		var h message.Header
		h.Set("Content-Type", "multipart/mixed")
		me, _ := message.NewMultipart(h, []*message.Entity{e})
		mr = me.MultipartReader()
	}

	l := list.New()
	l.PushBack(mr)

	return &Reader{Header{e.Header}, e, l}
}

// CreateReader reads a mail header from r and returns a new mail reader.
//
// If the message uses an unknown transfer encoding or charset, CreateReader
// returns an error that verifies message.IsUnknownCharset, but also returns a
// Reader that can be used.
func CreateReader(r io.Reader) (*Reader, error) {
	e, err := message.Read(r)
	if err != nil && !message.IsUnknownCharset(err) {
		return nil, err
	}

	return NewReader(e), err
}

// NextPart returns the next mail part. If there is no more part, io.EOF is
// returned as error.
//
// The returned Part.Body must be read completely before the next call to
// NextPart, otherwise it will be discarded.
//
// If the part uses an unknown transfer encoding or charset, NextPart returns an
// error that verifies message.IsUnknownCharset, but also returns a Part that
// can be used.
func (r *Reader) NextPart() (*Part, error) {
	for r.readers.Len() > 0 {
		e := r.readers.Back()
		mr := e.Value.(message.MultipartReader)

		p, err := mr.NextPart()
		if err == io.EOF {
			// This whole multipart entity has been read, continue with the next one
			r.readers.Remove(e)
			continue
		} else if err != nil && !message.IsUnknownCharset(err) {
			return nil, err
		}

		if pmr := p.MultipartReader(); pmr != nil {
			// This is a multipart part, read it
			r.readers.PushBack(pmr)
		} else {
			// This is a non-multipart part, return a mail part
			mp := &Part{Body: p.Body}
			t, _, _ := p.Header.ContentType()
			disp, _, _ := p.Header.ContentDisposition()
			if disp == "inline" || (disp != "attachment" && strings.HasPrefix(t, "text/")) {
				mp.Header = &InlineHeader{p.Header}
			} else {
				mp.Header = &AttachmentHeader{p.Header}
			}
			return mp, err
		}
	}

	return nil, io.EOF
}

// Close finishes the reader.
func (r *Reader) Close() error {
	for r.readers.Len() > 0 {
		e := r.readers.Back()
		mr := e.Value.(message.MultipartReader)

		if err := mr.Close(); err != nil {
			return err
		}

		r.readers.Remove(e)
	}

	return nil
}
