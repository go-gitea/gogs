// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sso

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/macaron"
	"gitea.com/macaron/session"
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
type ReverseProxy struct {
}

// getUserName extracts the username from the "setting.ReverseProxyAuthUser" header
func (r *ReverseProxy) getUserName(ctx *macaron.Context) string {
	webAuthUser := strings.TrimSpace(ctx.Req.Header.Get(setting.ReverseProxyAuthUser))
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
// Returns nil if header is empty or internal API is being called.
func (r *ReverseProxy) VerifyAuthData(ctx *macaron.Context, sess session.Store) *models.User {

	// Internal API should not use this auth method.
	if isInternalPath(ctx) {
		return nil
	}

	// Just return user if session is estabilshed already.
	user := SessionUser(sess)
	if user != nil {
		return user
	}

	// If no session established, get username from header.
	username := r.getUserName(ctx)
	if len(username) == 0 {
		return nil
	}

	var err error

	if r.isAutoRegisterAllowed() {
		// Use auto registration from reverse proxy if ENABLE_REVERSE_PROXY_AUTO_REGISTRATION enabled.
		if user, err = models.GetUserByName(username); err != nil {
			if models.IsErrUserNotExist(err) && r.isAutoRegisterAllowed() {
				if user = r.newUser(ctx); user == nil {
					return nil
				}
			} else {
				log.Error("GetUserByName: %v", err)
				return nil
			}
		}
	} else {
		// Use auto registration from other backends if ENABLE_REVERSE_PROXY_AUTO_REGISTRATION not enabled.
		if user, err = models.UserSignIn(username, "", true); err != nil {
			if !models.IsErrUserNotExist(err) {
				log.Error("UserSignIn: %v", err)
			}
			return nil
		}
	}

	// Make sure requests to API paths and PWA resources do not create a new session.
	if !isAPIPath(ctx) && !isAttachmentDownload(ctx) {

		// Update last user login timestamp.
		user.SetLastLogin()
		if err = models.UpdateUserCols(user, false, "last_login_unix"); err != nil {
			log.Error(fmt.Sprintf("VerifyAuthData: error updating user last login time [user: %d]", user.ID))
		}

		// Initialize new session. Will set lang and CSRF cookies.
		handleSignIn(ctx, sess, user)

		// Unfortunatelly we cannot do redirect here (would break git HTTP requests) to
		// reload page with user locale so first page after login may be displayed in
		// wrong language. Language handling in SSO mode should be reconsidered
		// in future gitea versions.
	}

	return user
}

// isAutoRegisterAllowed checks if EnableReverseProxyAutoRegister setting is true
func (r *ReverseProxy) isAutoRegisterAllowed() bool {
	return setting.Service.EnableReverseProxyAutoRegister
}

// newUser creates a new user object for the purpose of automatic registration
// and populates its name and email with the information present in request headers.
func (r *ReverseProxy) newUser(ctx *macaron.Context) *models.User {
	username := r.getUserName(ctx)
	if len(username) == 0 {
		return nil
	}

	email := gouuid.New().String() + "@localhost"
	if setting.Service.EnableReverseProxyEmail {
		webAuthEmail := ctx.Req.Header.Get(setting.ReverseProxyAuthEmail)
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
