// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"context"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/session"
	"code.gitea.io/gitea/modules/web/middleware"
)

// DataStore represents a data store
type DataStore middleware.DataStore

// SessionStore represents a session store
type SessionStore session.Store

// Method represents an authentication method (plugin) for HTTP requests.
type Method interface {
	// Verify tries to verify the authentication data contained in the request.
	// If verification is successful returns either an existing user object (with id > 0)
	// or a new user object (with id = 0) populated with the information that was found
	// in the authentication data (username or email).
	// Returns nil if verification fails.
	Verify(http *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) *models.User
}

// Initializable represents a structure that requires initialization
// It usually should only be called once before anything else is called
type Initializable interface {
	// Init should be called exactly once before using any of the other methods,
	// in order to allow the plugin to allocate necessary resources
	Init() error
}

// Named represents a named thing
type Named interface {
	Name() string
}

// Freeable represents a structure that is required to be freed
type Freeable interface {
	// Free should be called exactly once before application closes, in order to
	// give chance to the plugin to free any allocated resources
	Free() error
}

// PasswordAuthenticator represents a source of authentication
type PasswordAuthenticator interface {
	Authenticate(user *models.User, login, password string) (*models.User, error)
}

// SynchronizableSource represents a source that can synchronize users
type SynchronizableSource interface {
	Sync(ctx context.Context, updateExisting bool) error
}
