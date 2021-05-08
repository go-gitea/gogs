package imap

import (
	"io"
)

// A literal, as defined in RFC 3501 section 4.3.
type Literal interface {
	io.Reader

	// Len returns the number of bytes of the literal.
	Len() int
}
