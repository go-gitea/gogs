// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"code.gitea.io/gitea/models"
)

// RegisterSource causes an OAuth2 configuration to be registered
func (source *Source) RegisterSource() error {
	err := RegisterProvider(source.loginSource.Name, source.Provider, source.ClientID, source.ClientSecret, source.OpenIDConnectAutoDiscoveryURL, source.CustomURLMapping)
	return wrapOpenIDConnectInitializeError(err, source.loginSource.Name, source)
}

// UnregisterSource causes an OAuth2 configuration to be unregistered
func (source *Source) UnregisterSource() error {
	RemoveProvider(source.loginSource.Name)
	return nil
}

// wrapOpenIDConnectInitializeError is used to wrap the error but this cannot be done in modules/auth/oauth2
// inside oauth2: import cycle not allowed models -> modules/auth/oauth2 -> models
func wrapOpenIDConnectInitializeError(err error, providerName string, source *Source) error {
	if err != nil && source.Provider == "openidConnect" {
		err = models.ErrOpenIDConnectInitialize{ProviderName: providerName, OpenIDConnectAutoDiscoveryURL: source.OpenIDConnectAutoDiscoveryURL, Cause: err}
	}
	return err
}
