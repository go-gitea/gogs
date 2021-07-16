// Package steam implements the OpenID protocol for authenticating users through Steam.
package steam

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/url"
	"regexp"
	"strings"

	"github.com/markbates/goth"
)

// Session stores data during the auth process with Steam.
type Session struct {
	AuthURL       string
	CallbackURL   string
	SteamID       string
	ResponseNonce string
}

// GetAuthURL will return the URL set by calling the `BeginAuth` function on the Steam provider.
func (s Session) GetAuthURL() (string, error) {
	if s.AuthURL == "" {
		return "", errors.New(goth.NoAuthUrlErrorMessage)
	}
	return s.AuthURL, nil
}

// Authorize the session with Steam and return the unique response_nonce by OpenID.
func (s *Session) Authorize(provider goth.Provider, params goth.Params) (string, error) {
	p := provider.(*Provider)
	if params.Get("openid.mode") != "id_res" {
		return "", errors.New("Mode must equal to \"id_res\".")
	}

	if params.Get("openid.return_to") != s.CallbackURL {
		return "", errors.New("The \"return_to url\" must match the url of current request.")
	}

	v := make(url.Values)
	v.Set("openid.assoc_handle", params.Get("openid.assoc_handle"))
	v.Set("openid.signed", params.Get("openid.signed"))
	v.Set("openid.sig", params.Get("openid.sig"))
	v.Set("openid.ns", params.Get("openid.ns"))

	split := strings.Split(params.Get("openid.signed"), ",")
	for _, item := range split {
		v.Set("openid."+item, params.Get("openid."+item))
	}
	v.Set("openid.mode", "check_authentication")

	resp, err := p.Client().PostForm(apiLoginEndpoint, v)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	response := strings.Split(string(content), "\n")
	if response[0] != "ns:"+openIDNs {
		return "", errors.New("Wrong ns in the response.")
	}

	if response[1] == "is_valid:false" {
		return "", errors.New("Unable validate openId.")
	}

	openIDURL := params.Get("openid.claimed_id")
	validationRegExp := regexp.MustCompile("^(http|https)://steamcommunity.com/openid/id/[0-9]{15,25}$")
	if !validationRegExp.MatchString(openIDURL) {
		return "", errors.New("Invalid Steam ID pattern.")
	}

	s.SteamID = regexp.MustCompile("\\D+").ReplaceAllString(openIDURL, "")
	s.ResponseNonce = params.Get("openid.response_nonce")

	return s.ResponseNonce, nil
}

// Marshal the session into a string
func (s Session) Marshal() string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (s Session) String() string {
	return s.Marshal()
}

// UnmarshalSession will unmarshal a JSON string into a session.
func (p *Provider) UnmarshalSession(data string) (goth.Session, error) {
	s := &Session{}
	err := json.NewDecoder(strings.NewReader(data)).Decode(s)
	return s, err
}
