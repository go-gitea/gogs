// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sso

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	gouuid "github.com/google/uuid"
)

// Ensure the struct implements the interface.
var (
	_ SingleSignOn = &ReverseProxy{}
)

// ReverseProxy implements the SingleSignOn interface, but actually relies on
// a reverse proxy for authentication of users.
// On successful authentication the proxy is expected to populate the username in the
// "setting.ReverseProxyAuthUser" header. Optionally it can also populate the email of the
// user in the "setting.ReverseProxyAuthEmail" header.
type ReverseProxy struct{}

// getUserName extracts the username from the "setting.ReverseProxyAuthUser" header
func (r *ReverseProxy) getUserName(req *http.Request) string {
	webAuthUser := strings.TrimSpace(req.Header.Get(setting.ReverseProxyAuthUser))
	if len(webAuthUser) == 0 {
		return ""
	}
	return webAuthUser
}

// Init does nothing as the ReverseProxy implementation does not need initialization
func (r *ReverseProxy) Init() error {
	return nil
}

// Free does nothing as the ReverseProxy implementation does not have to release resources
func (r *ReverseProxy) Free() error {
	return nil
}

// IsEnabled checks if EnableReverseProxyAuth setting is true
func (r *ReverseProxy) IsEnabled() bool {
	return setting.Service.EnableReverseProxyAuth
}

// VerifyAuthData extracts the username from the "setting.ReverseProxyAuthUser" header
// of the request and returns the corresponding user object for that name.
// Verification of header data is not performed as it should have already been done by
// the revese proxy.
// If a username is available in the "setting.ReverseProxyAuthUser" header an existing
// user object is returned (populated with username or email found in header).
// Returns nil if header is empty.
func (r *ReverseProxy) VerifyAuthData(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) *models.User {
	username := r.getUserName(req)
	if len(username) == 0 {
		return nil
	}

	user, err := models.GetUserByName(username)
	if err != nil {
		if models.IsErrUserNotExist(err) && r.isAutoRegisterAllowed() {
			return r.newUser(req)
		}
		log.Error("GetUserByName: %v", err)
		return nil
	}

	return user
}

// isAutoRegisterAllowed checks if EnableReverseProxyAutoRegister setting is true
func (r *ReverseProxy) isAutoRegisterAllowed() bool {
	return setting.Service.EnableReverseProxyAutoRegister
}

// newUser creates a new user object for the purpose of automatic registration
// and populates its name and email with the information present in request headers.
func (r *ReverseProxy) newUser(req *http.Request) *models.User {
	username := r.getUserName(req)
	if len(username) == 0 {
		return nil
	}

	email := gouuid.New().String() + "@localhost"
	if setting.Service.EnableReverseProxyEmail {
		webAuthEmail := req.Header.Get(setting.ReverseProxyAuthEmail)
		if len(webAuthEmail) > 0 {
			email = webAuthEmail
		}
	}

	user := &models.User{
		Name:     username,
		Email:    email,
		Passwd:   username,
		IsActive: true,
	}
	if err := models.CreateUser(user); err != nil {
		// FIXME: should I create a system notice?
		log.Error("CreateUser: %v", err)
		return nil
	}
	return user
}
