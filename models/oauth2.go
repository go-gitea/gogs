// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"sort"

	"code.gitea.io/gitea/modules/auth/oauth2"
	"code.gitea.io/gitea/modules/log"
)

// OAuth2Provider describes the display values of a single OAuth2 provider
type OAuth2Provider struct {
	Name             string
	DisplayName      string
	Image            string
	CustomURLMapping *oauth2.CustomURLMapping
}

// OAuth2Providers contains the map of registered OAuth2 providers in Gitea (based on goth)
// key is used to map the OAuth2Provider with the goth provider type (also in LoginSource.OAuth2Config.Provider)
// value is used to store display data
var OAuth2Providers = map[string]OAuth2Provider{
	"bitbucket": {Name: "bitbucket", DisplayName: "Bitbucket", Image: "/assets/img/auth/bitbucket.png"},
	"dropbox":   {Name: "dropbox", DisplayName: "Dropbox", Image: "/assets/img/auth/dropbox.png"},
	"facebook":  {Name: "facebook", DisplayName: "Facebook", Image: "/assets/img/auth/facebook.png"},
	"github": {
		Name: "github", DisplayName: "GitHub", Image: "/assets/img/auth/github.png",
		CustomURLMapping: &oauth2.CustomURLMapping{
			TokenURL:   oauth2.GetDefaultTokenURL("github"),
			AuthURL:    oauth2.GetDefaultAuthURL("github"),
			ProfileURL: oauth2.GetDefaultProfileURL("github"),
			EmailURL:   oauth2.GetDefaultEmailURL("github"),
		},
	},
	"gitlab": {
		Name: "gitlab", DisplayName: "GitLab", Image: "/assets/img/auth/gitlab.png",
		CustomURLMapping: &oauth2.CustomURLMapping{
			TokenURL:   oauth2.GetDefaultTokenURL("gitlab"),
			AuthURL:    oauth2.GetDefaultAuthURL("gitlab"),
			ProfileURL: oauth2.GetDefaultProfileURL("gitlab"),
		},
	},
	"gplus":         {Name: "gplus", DisplayName: "Google", Image: "/assets/img/auth/google.png"},
	"openidConnect": {Name: "openidConnect", DisplayName: "OpenID Connect", Image: "/assets/img/auth/openid_connect.svg"},
	"twitter":       {Name: "twitter", DisplayName: "Twitter", Image: "/assets/img/auth/twitter.png"},
	"discord":       {Name: "discord", DisplayName: "Discord", Image: "/assets/img/auth/discord.png"},
	"gitea": {
		Name: "gitea", DisplayName: "Gitea", Image: "/assets/img/auth/gitea.png",
		CustomURLMapping: &oauth2.CustomURLMapping{
			TokenURL:   oauth2.GetDefaultTokenURL("gitea"),
			AuthURL:    oauth2.GetDefaultAuthURL("gitea"),
			ProfileURL: oauth2.GetDefaultProfileURL("gitea"),
		},
	},
	"nextcloud": {
		Name: "nextcloud", DisplayName: "Nextcloud", Image: "/assets/img/auth/nextcloud.png",
		CustomURLMapping: &oauth2.CustomURLMapping{
			TokenURL:   oauth2.GetDefaultTokenURL("nextcloud"),
			AuthURL:    oauth2.GetDefaultAuthURL("nextcloud"),
			ProfileURL: oauth2.GetDefaultProfileURL("nextcloud"),
		},
	},
	"yandex": {Name: "yandex", DisplayName: "Yandex", Image: "/assets/img/auth/yandex.png"},
	"mastodon": {
		Name: "mastodon", DisplayName: "Mastodon", Image: "/assets/img/auth/mastodon.png",
		CustomURLMapping: &oauth2.CustomURLMapping{
			AuthURL: oauth2.GetDefaultAuthURL("mastodon"),
		},
	},
}

// OAuth2DefaultCustomURLMappings contains the map of default URL's for OAuth2 providers that are allowed to have custom urls
// key is used to map the OAuth2Provider
// value is the mapping as defined for the OAuth2Provider
var OAuth2DefaultCustomURLMappings = map[string]*oauth2.CustomURLMapping{
	"github":    OAuth2Providers["github"].CustomURLMapping,
	"gitlab":    OAuth2Providers["gitlab"].CustomURLMapping,
	"gitea":     OAuth2Providers["gitea"].CustomURLMapping,
	"nextcloud": OAuth2Providers["nextcloud"].CustomURLMapping,
	"mastodon":  OAuth2Providers["mastodon"].CustomURLMapping,
}

// GetActiveOAuth2ProviderLoginSources returns all actived LoginOAuth2 sources
func GetActiveOAuth2ProviderLoginSources() ([]*LoginSource, error) {
	sources := make([]*LoginSource, 0, 1)
	if err := x.Where("is_actived = ? and type = ?", true, LoginOAuth2).Find(&sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// GetActiveOAuth2LoginSourceByName returns a OAuth2 LoginSource based on the given name
func GetActiveOAuth2LoginSourceByName(name string) (*LoginSource, error) {
	loginSource := new(LoginSource)
	has, err := x.Where("name = ? and type = ? and is_actived = ?", name, LoginOAuth2, true).Get(loginSource)
	if !has || err != nil {
		return nil, err
	}

	return loginSource, nil
}

// GetActiveOAuth2Providers returns the map of configured active OAuth2 providers
// key is used as technical name (like in the callbackURL)
// values to display
func GetActiveOAuth2Providers() ([]string, map[string]OAuth2Provider, error) {
	// Maybe also separate used and unused providers so we can force the registration of only 1 active provider for each type

	loginSources, err := GetActiveOAuth2ProviderLoginSources()
	if err != nil {
		return nil, nil, err
	}

	var orderedKeys []string
	providers := make(map[string]OAuth2Provider)
	for _, source := range loginSources {
		prov := OAuth2Providers[source.OAuth2().Provider]
		if source.OAuth2().IconURL != "" {
			prov.Image = source.OAuth2().IconURL
		}
		providers[source.Name] = prov
		orderedKeys = append(orderedKeys, source.Name)
	}

	sort.Strings(orderedKeys)

	return orderedKeys, providers, nil
}

// InitOAuth2 initialize the OAuth2 lib and register all active OAuth2 providers in the library
func InitOAuth2() error {
	if err := oauth2.InitSigningKey(); err != nil {
		return err
	}
	if err := oauth2.Init(x); err != nil {
		return err
	}
	return initOAuth2LoginSources()
}

// ResetOAuth2 clears existing OAuth2 providers and loads them from DB
func ResetOAuth2() error {
	oauth2.ClearProviders()
	return initOAuth2LoginSources()
}

// initOAuth2LoginSources is used to load and register all active OAuth2 providers
func initOAuth2LoginSources() error {
	loginSources, _ := GetActiveOAuth2ProviderLoginSources()
	for _, source := range loginSources {
		oAuth2Config := source.OAuth2()
		err := oauth2.RegisterProvider(source.Name, oAuth2Config.Provider, oAuth2Config.ClientID, oAuth2Config.ClientSecret, oAuth2Config.OpenIDConnectAutoDiscoveryURL, oAuth2Config.CustomURLMapping)
		if err != nil {
			log.Critical("Unable to register source: %s due to Error: %v. This source will be disabled.", source.Name, err)
			source.IsActived = false
			if err = UpdateSource(source); err != nil {
				log.Critical("Unable to update source %s to disable it. Error: %v", err)
				return err
			}
		}
	}
	return nil
}

// wrapOpenIDConnectInitializeError is used to wrap the error but this cannot be done in modules/auth/oauth2
// inside oauth2: import cycle not allowed models -> modules/auth/oauth2 -> models
func wrapOpenIDConnectInitializeError(err error, providerName string, oAuth2Config *OAuth2Config) error {
	if err != nil && "openidConnect" == oAuth2Config.Provider {
		err = ErrOpenIDConnectInitialize{ProviderName: providerName, OpenIDConnectAutoDiscoveryURL: oAuth2Config.OpenIDConnectAutoDiscoveryURL, Cause: err}
	}
	return err
}
