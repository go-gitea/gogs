// Package steam implements the OpenID protocol for authenticating users through Steam.
package steam

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/markbates/goth"
	"golang.org/x/oauth2"
)

const (
	// Steam API Endpoints
	apiLoginEndpoint       = "https://steamcommunity.com/openid/login"
	apiUserSummaryEndpoint = "http://api.steampowered.com/ISteamUser/GetPlayerSummaries/v0002/?key=%s&steamids=%s"

	// OpenID settings
	openIDMode       = "checkid_setup"
	openIDNs         = "http://specs.openid.net/auth/2.0"
	openIDIdentifier = "http://specs.openid.net/auth/2.0/identifier_select"
)

// New creates a new Steam provider, and sets up important connection details.
// You should always call `steam.New` to get a new Provider. Never try to create
// one manually.
func New(apiKey string, callbackURL string) *Provider {
	p := &Provider{
		APIKey:       apiKey,
		CallbackURL:  callbackURL,
		providerName: "steam",
	}
	return p
}

// Provider is the implementation of `goth.Provider` for accessing Steam
type Provider struct {
	APIKey       string
	CallbackURL  string
	HTTPClient   *http.Client
	providerName string
}

// Name gets the name used to retrieve this provider.
func (p *Provider) Name() string {
	return p.providerName
}

// SetName is to update the name of the provider (needed in case of multiple providers of 1 type)
func (p *Provider) SetName(name string) {
	p.providerName = name
}

func (p *Provider) Client() *http.Client {
	return goth.HTTPClientWithFallBack(p.HTTPClient)
}

// Debug is no-op for the Steam package.
func (p *Provider) Debug(debug bool) {}

// BeginAuth will return the authentication end-point for Steam.
func (p *Provider) BeginAuth(state string) (goth.Session, error) {
	u, err := p.getAuthURL()
	s := &Session{
		AuthURL:     u.String(),
		CallbackURL: p.CallbackURL,
	}
	return s, err
}

// getAuthURL is an internal function to build the correct
// authentication url to redirect the user to Steam.
func (p *Provider) getAuthURL() (*url.URL, error) {
	callbackURL, err := url.Parse(p.CallbackURL)
	if err != nil {
		return nil, err
	}

	urlValues := map[string]string{
		"openid.claimed_id": openIDIdentifier,
		"openid.identity":   openIDIdentifier,
		"openid.mode":       openIDMode,
		"openid.ns":         openIDNs,
		"openid.realm":      fmt.Sprintf("%s://%s", callbackURL.Scheme, callbackURL.Host),
		"openid.return_to":  callbackURL.String(),
	}

	u, err := url.Parse(apiLoginEndpoint)
	if err != nil {
		return nil, err
	}

	v := u.Query()
	for key, value := range urlValues {
		v.Set(key, value)
	}
	u.RawQuery = v.Encode()

	return u, nil
}

// FetchUser will go to Steam and access basic info about the user.
func (p *Provider) FetchUser(session goth.Session) (goth.User, error) {
	s := session.(*Session)
	u := goth.User{
		Provider:    p.Name(),
		AccessToken: s.ResponseNonce,
	}

	if s.SteamID == "" {
		// data is not yet retrieved since SteamID is still empty
		return u, fmt.Errorf("%s cannot get user information without SteamID", p.providerName)
	}

	apiURL := fmt.Sprintf(apiUserSummaryEndpoint, p.APIKey, s.SteamID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return u, err
	}
	req.Header.Add("Accept", "application/json")
	resp, err := p.Client().Do(req)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return u, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return u, fmt.Errorf("%s responded with a %d trying to fetch user information", p.providerName, resp.StatusCode)
	}

	u, err = buildUserObject(resp.Body, u)

	return u, err
}

// buildUserObject is an internal function to build a goth.User object
// based in the data stored in r
func buildUserObject(r io.Reader, u goth.User) (goth.User, error) {
	// Response object from Steam
	apiResponse := struct {
		Response struct {
			Players []struct {
				UserID              string `json:"steamid"`
				NickName            string `json:"personaname"`
				Name                string `json:"realname"`
				AvatarURL           string `json:"avatarfull"`
				LocationCountryCode string `json:"loccountrycode"`
				LocationStateCode   string `json:"locstatecode"`
			} `json:"players"`
		} `json:"response"`
	}{}

	err := json.NewDecoder(r).Decode(&apiResponse)
	if err != nil {
		return u, err
	}

	if l := len(apiResponse.Response.Players); l != 1 {
		return u, fmt.Errorf("Expected one player in API response. Got %d.", l)
	}

	player := apiResponse.Response.Players[0]
	u.UserID = player.UserID
	u.Name = player.Name
	if len(player.Name) == 0 {
		u.Name = "No name is provided by the Steam API"
	}
	u.NickName = player.NickName
	u.AvatarURL = player.AvatarURL
	u.Email = "No email is provided by the Steam API"
	u.Description = "No description is provided by the Steam API"

	if len(player.LocationStateCode) > 0 && len(player.LocationCountryCode) > 0 {
		u.Location = fmt.Sprintf("%s, %s", player.LocationStateCode, player.LocationCountryCode)
	} else if len(player.LocationCountryCode) > 0 {
		u.Location = player.LocationCountryCode
	} else if len(player.LocationStateCode) > 0 {
		u.Location = player.LocationStateCode
	} else {
		u.Location = "No location is provided by the Steam API"
	}

	return u, nil
}

// RefreshToken refresh token is not provided by Steam
func (p *Provider) RefreshToken(refreshToken string) (*oauth2.Token, error) {
	return nil, nil
}

// RefreshTokenAvailable refresh token is not provided by Steam
func (p *Provider) RefreshTokenAvailable() bool {
	return false
}
