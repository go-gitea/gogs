package sasl

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// The OAUTHBEARER mechanism name.
const OAuthBearer = "OAUTHBEARER"

type OAuthBearerError struct {
	Status  string `json:"status"`
	Schemes string `json:"schemes"`
	Scope   string `json:"scope"`
}

type OAuthBearerOptions struct {
	Username string
	Token    string
	Host     string
	Port     int
}

// Implements error
func (err *OAuthBearerError) Error() string {
	return fmt.Sprintf("OAUTHBEARER authentication error (%v)", err.Status)
}

type oauthBearerClient struct {
	OAuthBearerOptions
}

func (a *oauthBearerClient) Start() (mech string, ir []byte, err error) {
	mech = OAuthBearer
	var str = "n,a=" + a.Username + ","

	if a.Host != "" {
		str += "\x01host=" + a.Host
	}

	if a.Port != 0 {
		str += "\x01port=" + strconv.Itoa(a.Port)
	}
	str += "\x01auth=Bearer " + a.Token + "\x01\x01"
	ir = []byte(str)
	return
}

func (a *oauthBearerClient) Next(challenge []byte) ([]byte, error) {
	authBearerErr := &OAuthBearerError{}
	if err := json.Unmarshal(challenge, authBearerErr); err != nil {
		return nil, err
	} else {
		return nil, authBearerErr
	}
}

// An implementation of the OAUTHBEARER authentication mechanism, as
// described in RFC 7628.
func NewOAuthBearerClient(opt *OAuthBearerOptions) Client {
	return &oauthBearerClient{*opt}
}
