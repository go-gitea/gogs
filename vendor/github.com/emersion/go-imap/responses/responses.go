// IMAP responses defined in RFC 3501.
package responses

import (
	"errors"

	"github.com/emersion/go-imap"
)

// ErrUnhandled is used when a response hasn't been handled.
var ErrUnhandled = errors.New("imap: unhandled response")

var errNotEnoughFields = errors.New("imap: not enough fields in response")

// Handler handles responses.
type Handler interface {
	// Handle processes a response. If the response cannot be processed,
	// ErrUnhandledResp must be returned.
	Handle(resp imap.Resp) error
}

// HandlerFunc is a function that handles responses.
type HandlerFunc func(resp imap.Resp) error

// Handle implements Handler.
func (f HandlerFunc) Handle(resp imap.Resp) error {
	return f(resp)
}

// Replier is a Handler that needs to send raw data (for instance
// AUTHENTICATE).
type Replier interface {
	Handler
	Replies() <-chan []byte
}
