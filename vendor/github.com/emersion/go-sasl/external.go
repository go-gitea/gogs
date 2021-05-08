package sasl

// The EXTERNAL mechanism name.
const External = "EXTERNAL"

type externalClient struct {
	Identity string
}

func (a *externalClient) Start() (mech string, ir []byte, err error) {
	mech = External
	ir = []byte(a.Identity)
	return
}

func (a *externalClient) Next(challenge []byte) (response []byte, err error) {
	return nil, ErrUnexpectedServerChallenge
}

// An implementation of the EXTERNAL authentication mechanism, as described in
// RFC 4422. Authorization identity may be left blank to indicate that the
// client is requesting to act as the identity associated with the
// authentication credentials.
func NewExternalClient(identity string) Client {
	return &externalClient{identity}
}
