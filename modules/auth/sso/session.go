// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sso

import (
	"net/http"

	"code.gitea.io/gitea/models"
)

// Ensure the struct implements the interface.
var (
	_ SingleSignOn = &Session{}
)

// Session checks if there is a user uid stored in the session and returns the user
// object for that uid.
type Session struct{}

// Init does nothing as the Session implementation does not need to allocate any resources
func (s *Session) Init() error {
	return nil
}

// Free does nothing as the Session implementation does not have to release any resources
func (s *Session) Free() error {
	return nil
}

// IsEnabled returns true as this plugin is enabled by default and its not possible to disable
// it from settings.
func (s *Session) IsEnabled() bool {
	return true
}

// VerifyAuthData checks if there is a user uid stored in the session and returns the user
// object for that uid.
// Returns nil if there is no user uid stored in the session.
func (s *Session) VerifyAuthData(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) *models.User {
	user := SessionUser(sess)
	if user != nil {
		return user
	}
	return nil
}
