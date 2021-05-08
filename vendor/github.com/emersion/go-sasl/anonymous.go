package sasl

// The ANONYMOUS mechanism name.
const Anonymous = "ANONYMOUS"

type anonymousClient struct {
	Trace string
}

func (c *anonymousClient) Start() (mech string, ir []byte, err error) {
	mech = Anonymous
	ir = []byte(c.Trace)
	return
}

func (c *anonymousClient) Next(challenge []byte) (response []byte, err error) {
	return nil, ErrUnexpectedServerChallenge
}

// A client implementation of the ANONYMOUS authentication mechanism, as
// described in RFC 4505.
func NewAnonymousClient(trace string) Client {
	return &anonymousClient{trace}
}

// Get trace information from clients logging in anonymously.
type AnonymousAuthenticator func(trace string) error

type anonymousServer struct {
	done bool
	authenticate AnonymousAuthenticator
}

func (s *anonymousServer) Next(response []byte) (challenge []byte, done bool, err error) {
	if s.done {
		err = ErrUnexpectedClientResponse
		return
	}

	// No initial response, send an empty challenge
	if response == nil {
		return []byte{}, false, nil
	}

	s.done = true

	err = s.authenticate(string(response))
	done = true
	return
}

// A server implementation of the ANONYMOUS authentication mechanism, as
// described in RFC 4505.
func NewAnonymousServer(authenticator AnonymousAuthenticator) Server {
	return &anonymousServer{authenticate: authenticator}
}
