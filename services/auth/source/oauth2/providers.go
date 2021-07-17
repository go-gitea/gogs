// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"net/url"
	"sort"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/markbates/goth"
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
	"github.com/markbates/goth/providers/twitter"
	"github.com/markbates/goth/providers/yandex"
)

// Provider describes the display values of a single OAuth2 provider
type Provider struct {
	Name             string
	DisplayName      string
	Image            string
	CustomURLMapping *CustomURLMapping
}

// Providers contains the map of registered OAuth2 providers in Gitea (based on goth)
// key is used to map the OAuth2Provider with the goth provider type (also in LoginSource.OAuth2Config.Provider)
// value is used to store display data
var Providers = map[string]Provider{
	"bitbucket": {Name: "bitbucket", DisplayName: "Bitbucket", Image: "/assets/img/auth/bitbucket.png"},
	"dropbox":   {Name: "dropbox", DisplayName: "Dropbox", Image: "/assets/img/auth/dropbox.png"},
	"facebook":  {Name: "facebook", DisplayName: "Facebook", Image: "/assets/img/auth/facebook.png"},
	"github": {
		Name: "github", DisplayName: "GitHub", Image: "/assets/img/auth/github.png",
		CustomURLMapping: &CustomURLMapping{
			TokenURL:   github.TokenURL,
			AuthURL:    github.AuthURL,
			ProfileURL: github.ProfileURL,
			EmailURL:   github.EmailURL,
		},
	},
	"gitlab": {
		Name: "gitlab", DisplayName: "GitLab", Image: "/assets/img/auth/gitlab.png",
		CustomURLMapping: &CustomURLMapping{
			TokenURL:   gitlab.TokenURL,
			AuthURL:    gitlab.AuthURL,
			ProfileURL: gitlab.ProfileURL,
		},
	},
	"gplus":         {Name: "gplus", DisplayName: "Google", Image: "/assets/img/auth/google.png"},
	"openidConnect": {Name: "openidConnect", DisplayName: "OpenID Connect", Image: "/assets/img/auth/openid_connect.svg"},
	"twitter":       {Name: "twitter", DisplayName: "Twitter", Image: "/assets/img/auth/twitter.png"},
	"discord":       {Name: "discord", DisplayName: "Discord", Image: "/assets/img/auth/discord.png"},
	"gitea": {
		Name: "gitea", DisplayName: "Gitea", Image: "/assets/img/auth/gitea.png",
		CustomURLMapping: &CustomURLMapping{
			TokenURL:   gitea.TokenURL,
			AuthURL:    gitea.AuthURL,
			ProfileURL: gitea.ProfileURL,
		},
	},
	"nextcloud": {
		Name: "nextcloud", DisplayName: "Nextcloud", Image: "/assets/img/auth/nextcloud.png",
		CustomURLMapping: &CustomURLMapping{
			TokenURL:   nextcloud.TokenURL,
			AuthURL:    nextcloud.AuthURL,
			ProfileURL: nextcloud.ProfileURL,
		},
	},
	"yandex": {Name: "yandex", DisplayName: "Yandex", Image: "/assets/img/auth/yandex.png"},
	"mastodon": {
		Name: "mastodon", DisplayName: "Mastodon", Image: "/assets/img/auth/mastodon.png",
		CustomURLMapping: &CustomURLMapping{
			AuthURL: mastodon.InstanceURL,
		},
	},
}

// GetActiveOAuth2Providers returns the map of configured active OAuth2 providers
// key is used as technical name (like in the callbackURL)
// values to display
func GetActiveOAuth2Providers() ([]string, map[string]Provider, error) {
	// Maybe also separate used and unused providers so we can force the registration of only 1 active provider for each type

	loginSources, err := models.GetActiveOAuth2ProviderLoginSources()
	if err != nil {
		return nil, nil, err
	}

	var orderedKeys []string
	providers := make(map[string]Provider)
	for _, source := range loginSources {
		prov := Providers[source.Cfg.(*Source).Provider]
		if source.Cfg.(*Source).IconURL != "" {
			prov.Image = source.Cfg.(*Source).IconURL
		}
		providers[source.Name] = prov
		orderedKeys = append(orderedKeys, source.Name)
	}

	sort.Strings(orderedKeys)

	return orderedKeys, providers, nil
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
	}

	// always set the name if provider is created so we can support multiple setups of 1 provider
	if err == nil && provider != nil {
		provider.SetName(providerName)
	}

	return provider, err
}
