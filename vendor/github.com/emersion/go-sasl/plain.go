package sasl

import (
	"bytes"
	"errors"
)

// The PLAIN mechanism name.
const Plain = "PLAIN"

type plainClient struct {
	Identity string
	Username string
	Password string
}

func (a *plainClient) Start() (mech string, ir []byte, err error) {
	mech = "PLAIN"
	ir = []byte(a.Identity + "\x00" + a.Username + "\x00" + a.Password)
	return
}

func (a *plainClient) Next(challenge []byte) (response []byte, err error) {
	return nil, ErrUnexpectedServerChallenge
}

// A client implementation of the PLAIN authentication mechanism, as described
// in RFC 4616. Authorization identity may be left blank to indicate that it is
// the same as the username.
func NewPlainClient(identity, username, password string) Client {
	return &plainClient{identity, username, password}
}

// Authenticates users with an identity, a username and a password. If the
// identity is left blank, it indicates that it is the same as the username.
// If identity is not empty and the server doesn't support it, an error must be
// returned.
type PlainAuthenticator func(identity, username, password string) error

type plainServer struct {
	done bool
	authenticate PlainAuthenticator
}

func (a *plainServer) Next(response []byte) (challenge []byte, done bool, err error) {
	if a.done {
		err = ErrUnexpectedClientResponse
		return
	}

	// No initial response, send an empty challenge
	if response == nil {
		return []byte{}, false, nil
	}

	a.done = true

	parts := bytes.Split(response, []byte("\x00"))
	if len(parts) != 3 {
		err = errors.New("Invalid response")
		return
	}

	identity := string(parts[0])
	username := string(parts[1])
	password := string(parts[2])

	err = a.authenticate(identity, username, password)
	done = true
	return
}

// A server implementation of the PLAIN authentication mechanism, as described
// in RFC 4616.
func NewPlainServer(authenticator PlainAuthenticator) Server {
	return &plainServer{authenticate: authenticator}
}
