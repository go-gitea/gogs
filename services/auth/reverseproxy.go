// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"

	gouuid "github.com/google/uuid"
)

// Ensure the struct implements the interface.
var (
	_ Method = &ReverseProxy{}
	_ Named  = &ReverseProxy{}
)

// ReverseProxy implements the Auth interface, but actually relies on
// a reverse proxy for authentication of users.
// On successful authentication the proxy is expected to populate the username in the
// "setting.ReverseProxyAuthUser" header. Optionally it can also populate the email of the
// user in the "setting.ReverseProxyAuthEmail" header.
type ReverseProxy struct {
}

// getUserName extracts the username from the "setting.ReverseProxyAuthUser" header
func (r *ReverseProxy) getUserName(req *http.Request) string {
	webAuthUser := strings.TrimSpace(req.Header.Get(setting.ReverseProxyAuthUser))
	if len(webAuthUser) == 0 {
		return ""
	}
	return webAuthUser
}

// Name represents the name of auth method
func (r *ReverseProxy) Name() string {
	return "reverse_proxy"
}

// Verify extracts the username from the "setting.ReverseProxyAuthUser" header
// of the request and returns the corresponding user object for that name.
// Verification of header data is not performed as it should have already been done by
// the revese proxy.
// If a username is available in the "setting.ReverseProxyAuthUser" header an existing
// user object is returned (populated with username or email found in header).
// Returns nil if header is empty.
func (r *ReverseProxy) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) *models.User {
	username := r.getUserName(req)
	if len(username) == 0 {
		return nil
	}
	log.Trace("ReverseProxy Authorization: Found username: %s", username)

	user, err := models.GetUserByName(username)
	if err != nil {
		if !models.IsErrUserNotExist(err) || !r.isAutoRegisterAllowed() {
			log.Error("GetUserByName: %v", err)
			return nil
		}
		user = r.newUser(req)
	}

	// Make sure requests to API paths, attachment downloads, git and LFS do not create a new session
	if !middleware.IsAPIPath(req) && !isAttachmentDownload(req) && !isGitRawOrLFSPath(req) {
		if sess != nil && (sess.Get("uid") == nil || sess.Get("uid").(int64) != user.ID) {
			handleSignIn(w, req, sess, user)
		}
	}
	store.GetData()["IsReverseProxy"] = true

	log.Trace("ReverseProxy Authorization: Logged in user %-v", user)
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
		IsActive: true,
	}
	if err := models.CreateUser(user); err != nil {
		// FIXME: should I create a system notice?
		log.Error("CreateUser: %v", err)
		return nil
	}

	return user
}
