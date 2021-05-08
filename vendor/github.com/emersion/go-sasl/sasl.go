// Library for Simple Authentication and Security Layer (SASL) defined in RFC 4422.
package sasl

// Note:
//   Most of this code was copied, with some modifications, from net/smtp. It
//   would be better if Go provided a standard package (e.g. crypto/sasl) that
//   could be shared by SMTP, IMAP, and other packages.

import (
	"errors"
)

// Common SASL errors.
var (
	ErrUnexpectedClientResponse = errors.New("sasl: unexpected client response")
	ErrUnexpectedServerChallenge = errors.New("sasl: unexpected server challenge")
)

// Client interface to perform challenge-response authentication.
type Client interface {
	// Begins SASL authentication with the server. It returns the
	// authentication mechanism name and "initial response" data (if required by
	// the selected mechanism). A non-nil error causes the client to abort the
	// authentication attempt.
	//
	// A nil ir value is different from a zero-length value. The nil value
	// indicates that the selected mechanism does not use an initial response,
	// while a zero-length value indicates an empty initial response, which must
	// be sent to the server.
	Start() (mech string, ir []byte, err error)

	// Continues challenge-response authentication. A non-nil error causes
	// the client to abort the authentication attempt.
	Next(challenge []byte) (response []byte, err error)
}

// Server interface to perform challenge-response authentication.
type Server interface {
	// Begins or continues challenge-response authentication. If the client
	// supplies an initial response, response is non-nil.
	//
	// If the authentication is finished, done is set to true. If the
	// authentication has failed, an error is returned.
	Next(response []byte) (challenge []byte, done bool, err error)
}
