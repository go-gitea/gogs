package sasl

import (
	"encoding/json"
	"fmt"
)

// The XOAUTH2 mechanism name.
const Xoauth2 = "XOAUTH2"

// An XOAUTH2 error.
type Xoauth2Error struct {
	Status string `json:"status"`
	Schemes string `json:"schemes"`
	Scope string `json:"scope"`
}

// Implements error.
func (err *Xoauth2Error) Error() string {
	return fmt.Sprintf("XOAUTH2 authentication error (%v)", err.Status)
}

type xoauth2Client struct {
	Username string
	Token string
}

func (a *xoauth2Client) Start() (mech string, ir []byte, err error) {
	mech = Xoauth2
	ir = []byte("user=" + a.Username + "\x01auth=Bearer " + a.Token + "\x01\x01")
	return
}

func (a *xoauth2Client) Next(challenge []byte) ([]byte, error) {
	// Server sent an error response
	xoauth2Err := &Xoauth2Error{}
	if err := json.Unmarshal(challenge, xoauth2Err); err != nil {
		return nil, err
	} else {
		return nil, xoauth2Err
	}
}

// An implementation of the XOAUTH2 authentication mechanism, as
// described in https://developers.google.com/gmail/xoauth2_protocol.
func NewXoauth2Client(username, token string) Client {
	return &xoauth2Client{username, token}
}
