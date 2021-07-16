// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"net/http"
	"net/url"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	uuid "github.com/google/uuid"
	"github.com/lafriks/xormstore"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/bitbucket"
	"github.com/markbates/goth/providers/discord"
	"github.com/markbates/goth/providers/dropbox"
	"github.com/markbates/goth/providers/facebook"
	"github.com/markbates/goth/providers/gitea"
	"github.com/markbates/goth/providers/github"
	"github.com/markbates/goth/providers/gitlab"
	"github.com/markbates/goth/providers/google"
	"github.com/markbates/goth/providers/mastodon"
	"github.com/markbates/goth/providers/nextcloud"
	"github.com/markbates/goth/providers/openidConnect"
	"github.com/markbates/goth/providers/steam"
	"github.com/markbates/goth/providers/twitter"
	"github.com/markbates/goth/providers/yandex"
	"xorm.io/xorm"
)

var (
	sessionUsersStoreKey = "gitea-oauth2-sessions"
	providerHeaderKey    = "gitea-oauth2-provider"
)

// CustomURLMapping describes the urls values to use when customizing OAuth2 provider URLs
type CustomURLMapping struct {
	AuthURL    string
	TokenURL   string
	ProfileURL string
	EmailURL   string
}

// Init initialize the setup of the OAuth2 library
func Init(x *xorm.Engine) error {
	store, err := xormstore.NewOptions(x, xormstore.Options{
		TableName: "oauth2_session",
	}, []byte(sessionUsersStoreKey))

	if err != nil {
		return err
	}
	// according to the Goth lib:
	// set the maxLength of the cookies stored on the disk to a larger number to prevent issues with:
	// securecookie: the value is too long
	// when using OpenID Connect , since this can contain a large amount of extra information in the id_token

	// Note, when using the FilesystemStore only the session.ID is written to a browser cookie, so this is explicit for the storage on disk
	store.MaxLength(setting.OAuth2.MaxTokenLength)
	gothic.Store = store

	gothic.SetState = func(req *http.Request) string {
		return uuid.New().String()
	}

	gothic.GetProviderName = func(req *http.Request) (string, error) {
		return req.Header.Get(providerHeaderKey), nil
	}

	return nil
}

// Auth OAuth2 auth service
func Auth(provider string, request *http.Request, response http.ResponseWriter) error {
	// not sure if goth is thread safe (?) when using multiple providers
	request.Header.Set(providerHeaderKey, provider)

	// don't use the default gothic begin handler to prevent issues when some error occurs
	// normally the gothic library will write some custom stuff to the response instead of our own nice error page
	//gothic.BeginAuthHandler(response, request)

	url, err := gothic.GetAuthURL(response, request)
	if err == nil {
		http.Redirect(response, request, url, http.StatusTemporaryRedirect)
	}
	return err
}

// ProviderCallback handles OAuth callback, resolve to a goth user and send back to original url
// this will trigger a new authentication request, but because we save it in the session we can use that
func ProviderCallback(provider string, request *http.Request, response http.ResponseWriter) (goth.User, error) {
	// not sure if goth is thread safe (?) when using multiple providers
	request.Header.Set(providerHeaderKey, provider)

	user, err := gothic.CompleteUserAuth(response, request)
	if err != nil {
		return user, err
	}

	return user, nil
}

// RegisterProvider register a OAuth2 provider in goth lib
func RegisterProvider(providerName, providerType, clientID, clientSecret, openIDConnectAutoDiscoveryURL string, customURLMapping *CustomURLMapping) error {
	provider, err := createProvider(providerName, providerType, clientID, clientSecret, openIDConnectAutoDiscoveryURL, customURLMapping)

	if err == nil && provider != nil {
		goth.UseProviders(provider)
	}

	return err
}

// RemoveProvider removes the given OAuth2 provider from the goth lib
func RemoveProvider(providerName string) {
	delete(goth.GetProviders(), providerName)
}

// ClearProviders clears all OAuth2 providers from the goth lib
func ClearProviders() {
	goth.ClearProviders()
}

// used to create different types of goth providers
func createProvider(providerName, providerType, clientID, clientSecret, openIDConnectAutoDiscoveryURL string, customURLMapping *CustomURLMapping) (goth.Provider, error) {
	callbackURL := setting.AppURL + "user/oauth2/" + url.PathEscape(providerName) + "/callback"

	var provider goth.Provider
	var err error

	switch providerType {
	case "bitbucket":
		provider = bitbucket.New(clientID, clientSecret, callbackURL, "account")
	case "dropbox":
		provider = dropbox.New(clientID, clientSecret, callbackURL)
	case "facebook":
		provider = facebook.New(clientID, clientSecret, callbackURL, "email")
	case "github":
		authURL := github.AuthURL
		tokenURL := github.TokenURL
		profileURL := github.ProfileURL
		emailURL := github.EmailURL
		if customURLMapping != nil {
			if len(customURLMapping.AuthURL) > 0 {
				authURL = customURLMapping.AuthURL
			}
			if len(customURLMapping.TokenURL) > 0 {
				tokenURL = customURLMapping.TokenURL
			}
			if len(customURLMapping.ProfileURL) > 0 {
				profileURL = customURLMapping.ProfileURL
			}
			if len(customURLMapping.EmailURL) > 0 {
				emailURL = customURLMapping.EmailURL
			}
		}
		scopes := []string{}
		if setting.OAuth2Client.EnableAutoRegistration {
			scopes = append(scopes, "user:email")
		}
		provider = github.NewCustomisedURL(clientID, clientSecret, callbackURL, authURL, tokenURL, profileURL, emailURL, scopes...)
	case "gitlab":
		authURL := gitlab.AuthURL
		tokenURL := gitlab.TokenURL
		profileURL := gitlab.ProfileURL
		if customURLMapping != nil {
			if len(customURLMapping.AuthURL) > 0 {
				authURL = customURLMapping.AuthURL
			}
			if len(customURLMapping.TokenURL) > 0 {
				tokenURL = customURLMapping.TokenURL
			}
			if len(customURLMapping.ProfileURL) > 0 {
				profileURL = customURLMapping.ProfileURL
			}
		}
		provider = gitlab.NewCustomisedURL(clientID, clientSecret, callbackURL, authURL, tokenURL, profileURL, "read_user")
	case "gplus": // named gplus due to legacy gplus -> google migration (Google killed Google+). This ensures old connections still work
		scopes := []string{"email"}
		if setting.OAuth2Client.UpdateAvatar || setting.OAuth2Client.EnableAutoRegistration {
			scopes = append(scopes, "profile")
		}
		provider = google.New(clientID, clientSecret, callbackURL, scopes...)
	case "openidConnect":
		if provider, err = openidConnect.New(clientID, clientSecret, callbackURL, openIDConnectAutoDiscoveryURL, setting.OAuth2Client.OpenIDConnectScopes...); err != nil {
			log.Warn("Failed to create OpenID Connect Provider with name '%s' with url '%s': %v", providerName, openIDConnectAutoDiscoveryURL, err)
		}
	case "twitter":
		provider = twitter.NewAuthenticate(clientID, clientSecret, callbackURL)
	case "discord":
		provider = discord.New(clientID, clientSecret, callbackURL, discord.ScopeIdentify, discord.ScopeEmail)
	case "gitea":
		authURL := gitea.AuthURL
		tokenURL := gitea.TokenURL
		profileURL := gitea.ProfileURL
		if customURLMapping != nil {
			if len(customURLMapping.AuthURL) > 0 {
				authURL = customURLMapping.AuthURL
			}
			if len(customURLMapping.TokenURL) > 0 {
				tokenURL = customURLMapping.TokenURL
			}
			if len(customURLMapping.ProfileURL) > 0 {
				profileURL = customURLMapping.ProfileURL
			}
		}
		provider = gitea.NewCustomisedURL(clientID, clientSecret, callbackURL, authURL, tokenURL, profileURL)
	case "nextcloud":
		authURL := nextcloud.AuthURL
		tokenURL := nextcloud.TokenURL
		profileURL := nextcloud.ProfileURL
		if customURLMapping != nil {
			if len(customURLMapping.AuthURL) > 0 {
				authURL = customURLMapping.AuthURL
			}
			if len(customURLMapping.TokenURL) > 0 {
				tokenURL = customURLMapping.TokenURL
			}
			if len(customURLMapping.ProfileURL) > 0 {
				profileURL = customURLMapping.ProfileURL
			}
		}
		provider = nextcloud.NewCustomisedURL(clientID, clientSecret, callbackURL, authURL, tokenURL, profileURL)
	case "yandex":
		// See https://tech.yandex.com/passport/doc/dg/reference/response-docpage/
		provider = yandex.New(clientID, clientSecret, callbackURL, "login:email", "login:info", "login:avatar")
	case "mastodon":
		instanceURL := mastodon.InstanceURL
		if customURLMapping != nil && len(customURLMapping.AuthURL) > 0 {
			instanceURL = customURLMapping.AuthURL
		}
		provider = mastodon.NewCustomisedURL(clientID, clientSecret, callbackURL, instanceURL)
	case "steam":
		provider = steam.New(clientSecret, callbackURL)
	}

	// always set the name if provider is created so we can support multiple setups of 1 provider
	if err == nil && provider != nil {
		provider.SetName(providerName)
	}

	return provider, err
}

// GetDefaultTokenURL return the default token url for the given provider
func GetDefaultTokenURL(provider string) string {
	switch provider {
	case "github":
		return github.TokenURL
	case "gitlab":
		return gitlab.TokenURL
	case "gitea":
		return gitea.TokenURL
	case "nextcloud":
		return nextcloud.TokenURL
	}
	return ""
}

// GetDefaultAuthURL return the default authorize url for the given provider
func GetDefaultAuthURL(provider string) string {
	switch provider {
	case "github":
		return github.AuthURL
	case "gitlab":
		return gitlab.AuthURL
	case "gitea":
		return gitea.AuthURL
	case "nextcloud":
		return nextcloud.AuthURL
	case "mastodon":
		return mastodon.InstanceURL
	}
	return ""
}

// GetDefaultProfileURL return the default profile url for the given provider
func GetDefaultProfileURL(provider string) string {
	switch provider {
	case "github":
		return github.ProfileURL
	case "gitlab":
		return gitlab.ProfileURL
	case "gitea":
		return gitea.ProfileURL
	case "nextcloud":
		return nextcloud.ProfileURL
	}
	return ""
}

// GetDefaultEmailURL return the default email url for the given provider
func GetDefaultEmailURL(provider string) string {
	if provider == "github" {
		return github.EmailURL
	}
	return ""
}
