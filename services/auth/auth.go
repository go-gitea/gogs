// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"
)

// authMethods contains the list of authentication plugins in the order they are expected to be
// executed.
//
// The OAuth2 plugin is expected to be executed first, as it must ignore the user id stored
// in the session (if there is a user id stored in session other plugins might return the user
// object for that id).
//
// The Session plugin is expected to be executed second, in order to skip authentication
// for users that have already signed in.
var authMethods = []Method{
	&OAuth2{},
	&Basic{},
	&Session{},
}

// The purpose of the following three function variables is to let the linter know that
// those functions are not dead code and are actually being used
var (
	_ = handleSignIn
)

// Methods returns the instances of all registered methods
func Methods() []Method {
	return authMethods
}

// Register adds the specified instance to the list of available methods
func Register(method Method) {
	authMethods = append(authMethods, method)
}

// Init should be called exactly once when the application starts to allow plugins
// to allocate necessary resources
func Init() {
	if setting.Service.EnableReverseProxyAuth {
		Register(&ReverseProxy{})
	}
	specialInit()
	for _, method := range Methods() {
		initializable, ok := method.(Initializable)
		if !ok {
			continue
		}

		err := initializable.Init()
		if err != nil {
			log.Error("Could not initialize '%s' auth method, error: %s", reflect.TypeOf(method).String(), err)
		}
	}
}

// Free should be called exactly once when the application is terminating to allow Auth plugins
// to release necessary resources
func Free() {
	for _, method := range Methods() {
		freeable, ok := method.(Freeable)
		if !ok {
			continue
		}

		err := freeable.Free()
		if err != nil {
			log.Error("Could not free '%s' auth method, error: %s", reflect.TypeOf(method).String(), err)
		}
	}
}

// isAttachmentDownload check if request is a file download (GET) with URL to an attachment
func isAttachmentDownload(req *http.Request) bool {
	return strings.HasPrefix(req.URL.Path, "/attachments/") && req.Method == "GET"
}

var gitRawPathRe = regexp.MustCompile(`^/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+/(?:(?:git-(?:(?:upload)|(?:receive))-pack$)|(?:info/refs$)|(?:HEAD$)|(?:objects/)|raw/)`)
var lfsPathRe = regexp.MustCompile(`^/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+/info/lfs/`)

func isGitRawOrLFSPath(req *http.Request) bool {
	if gitRawPathRe.MatchString(req.URL.Path) {
		return true
	}
	if setting.LFS.StartServer {
		return lfsPathRe.MatchString(req.URL.Path)
	}
	return false
}

// handleSignIn clears existing session variables and stores new ones for the specified user object
func handleSignIn(resp http.ResponseWriter, req *http.Request, sess SessionStore, user *models.User) {
	_ = sess.Delete("openid_verified_uri")
	_ = sess.Delete("openid_signin_remember")
	_ = sess.Delete("openid_determined_email")
	_ = sess.Delete("openid_determined_username")
	_ = sess.Delete("twofaUid")
	_ = sess.Delete("twofaRemember")
	_ = sess.Delete("u2fChallenge")
	_ = sess.Delete("linkAccount")
	err := sess.Set("uid", user.ID)
	if err != nil {
		log.Error(fmt.Sprintf("Error setting session: %v", err))
	}
	err = sess.Set("uname", user.Name)
	if err != nil {
		log.Error(fmt.Sprintf("Error setting session: %v", err))
	}

	// Language setting of the user overwrites the one previously set
	// If the user does not have a locale set, we save the current one.
	if len(user.Language) == 0 {
		lc := middleware.Locale(resp, req)
		user.Language = lc.Language()
		if err := models.UpdateUserCols(user, "language"); err != nil {
			log.Error(fmt.Sprintf("Error updating user language [user: %d, locale: %s]", user.ID, user.Language))
			return
		}
	}

	middleware.SetLocaleCookie(resp, user.Language, 0)

	// Clear whatever CSRF has right now, force to generate a new one
	middleware.DeleteCSRFCookie(resp)
}
