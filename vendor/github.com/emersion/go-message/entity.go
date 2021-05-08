package message

import (
	"bufio"
	"io"
	"strings"

	"github.com/emersion/go-message/textproto"
)

// An Entity is either a whole message or a one of the parts in the body of a
// multipart entity.
type Entity struct {
	Header Header    // The entity's header.
	Body   io.Reader // The decoded entity's body.

	mediaType   string
	mediaParams map[string]string
}

// New makes a new message with the provided header and body. The entity's
// transfer encoding and charset are automatically decoded to UTF-8.
//
// If the message uses an unknown transfer encoding or charset, New returns an
// error that verifies IsUnknownCharset, but also returns an Entity that can
// be read.
func New(header Header, body io.Reader) (*Entity, error) {
	var err error

	mediaType, mediaParams, _ := header.ContentType()

	// QUIRK: RFC 2045 section 6.4 specifies that multipart messages can't have
	// a Content-Transfer-Encoding other than "7bit", "8bit" or "binary".
	// However some messages in the wild are non-conformant and have it set to
	// e.g. "quoted-printable". So we just ignore it for multipart.
	// See https://github.com/emersion/go-message/issues/48
	if !strings.HasPrefix(mediaType, "multipart/") {
		enc := header.Get("Content-Transfer-Encoding")
		if decoded, encErr := encodingReader(enc, body); encErr != nil {
			err = UnknownEncodingError{encErr}
		} else {
			body = decoded
		}
	}

	// RFC 2046 section 4.1.2: charset only applies to text/*
	if strings.HasPrefix(mediaType, "text/") {
		if ch, ok := mediaParams["charset"]; ok {
			if converted, charsetErr := charsetReader(ch, body); charsetErr != nil {
				err = UnknownCharsetError{charsetErr}
			} else {
				body = converted
			}
		}
	}

	return &Entity{
		Header:      header,
		Body:        body,
		mediaType:   mediaType,
		mediaParams: mediaParams,
	}, err
}

// NewMultipart makes a new multipart message with the provided header and
// parts. The Content-Type header must begin with "multipart/".
//
// If the message uses an unknown transfer encoding, NewMultipart returns an
// error that verifies IsUnknownCharset, but also returns an Entity that can
// be read.
func NewMultipart(header Header, parts []*Entity) (*Entity, error) {
	r := &multipartBody{
		header: header,
		parts:  parts,
	}

	return New(header, r)
}

// Read reads a message from r. The message's encoding and charset are
// automatically decoded to raw UTF-8. Note that this function only reads the
// message header.
//
// If the message uses an unknown transfer encoding or charset, Read returns an
// error that verifies IsUnknownCharset or IsUnknownEncoding, but also returns
// an Entity that can be read.
func Read(r io.Reader) (*Entity, error) {
	br := bufio.NewReader(r)
	h, err := textproto.ReadHeader(br)
	if err != nil {
		return nil, err
	}

	return New(Header{h}, br)
}

// MultipartReader returns a MultipartReader that reads parts from this entity's
// body. If this entity is not multipart, it returns nil.
func (e *Entity) MultipartReader() MultipartReader {
	if !strings.HasPrefix(e.mediaType, "multipart/") {
		return nil
	}
	if mb, ok := e.Body.(*multipartBody); ok {
		return mb
	}
	return &multipartReader{textproto.NewMultipartReader(e.Body, e.mediaParams["boundary"])}
}

// writeBodyTo writes this entity's body to w (without the header).
func (e *Entity) writeBodyTo(w *Writer) error {
	var err error
	if mb, ok := e.Body.(*multipartBody); ok {
		err = mb.writeBodyTo(w)
	} else {
		_, err = io.Copy(w, e.Body)
	}
	return err
}

// WriteTo writes this entity's header and body to w.
func (e *Entity) WriteTo(w io.Writer) error {
	ew, err := CreateWriter(w, e.Header)
	if err != nil {
		return err
	}
	defer ew.Close()

	return e.writeBodyTo(ew)
}
