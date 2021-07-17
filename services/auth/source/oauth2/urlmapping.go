// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

// CustomURLMapping describes the urls values to use when customizing OAuth2 provider URLs
type CustomURLMapping struct {
	AuthURL    string
	TokenURL   string
	ProfileURL string
	EmailURL   string
}

// DefaultCustomURLMappings contains the map of default URL's for OAuth2 providers that are allowed to have custom urls
// key is used to map the OAuth2Provider
// value is the mapping as defined for the OAuth2Provider
var DefaultCustomURLMappings = map[string]*CustomURLMapping{
	"github":    Providers["github"].CustomURLMapping,
	"gitlab":    Providers["gitlab"].CustomURLMapping,
	"gitea":     Providers["gitea"].CustomURLMapping,
	"nextcloud": Providers["nextcloud"].CustomURLMapping,
	"mastodon":  Providers["mastodon"].CustomURLMapping,
}
