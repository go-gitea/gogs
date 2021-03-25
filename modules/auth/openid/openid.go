// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package openid

import (
	"time"

	"github.com/yohcop/openid-go"
)

// For the demo, we use in-memory infinite storage nonce and discovery
// cache. In your app, do not use this as it will eat up memory and
// never
// free it. Use your own implementation, on a better database system.
// If you have multiple servers for example, you may need to share at
// least
// the nonceStore between them.
var (
	nonceStore     = openid.NewSimpleNonceStore()
	discoveryCache = newTimedDiscoveryCache(24 * time.Hour)
)

// Verify handles response from OpenID provider
func Verify(fullURL string) (id string, err error) {
	return openid.Verify(fullURL, discoveryCache, nonceStore)
}

// Normalize normalizes an OpenID URI
func Normalize(url string) (id string, err error) {
	return openid.Normalize(url)
}

// RedirectURL redirects browser
func RedirectURL(id, callbackURL, realm string) (string, error) {
	return openid.RedirectURL(id, callbackURL, realm)
}
