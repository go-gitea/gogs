// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/auth/source/oauth2"
)

// Ensure the struct implements the interface.
var (
	_ Method = &OAuth2{}
	_ Named  = &OAuth2{}
)

// CheckOAuthAccessToken returns uid of user from oauth token
func CheckOAuthAccessToken(accessToken string) int64 {
	// JWT tokens require a "."
	if !strings.Contains(accessToken, ".") {
		return 0
	}
	token, err := oauth2.ParseToken(accessToken)
	if err != nil {
		log.Trace("ParseOAuth2Token: %v", err)
		return 0
	}
	var grant *models.OAuth2Grant
	if grant, err = models.GetOAuth2GrantByID(token.GrantID); err != nil || grant == nil {
		return 0
	}
	if token.Type != oauth2.TypeAccessToken {
		return 0
	}
	if token.ExpiresAt < time.Now().Unix() || token.IssuedAt > time.Now().Unix() {
		return 0
	}
	return grant.UserID
}

// OAuth2 implements the Auth interface and authenticates requests
// (API requests only) by looking for an OAuth token in query parameters or the
// "Authorization" header.
type OAuth2 struct {
}

// Name represents the name of auth method
func (o *OAuth2) Name() string {
	return "oauth2"
}

// userIDFromToken returns the user id corresponding to the OAuth token.
func (o *OAuth2) userIDFromToken(req *http.Request, store DataStore) int64 {
	_ = req.ParseForm()

	// Check access token.
	tokenSHA := req.Form.Get("token")
	if len(tokenSHA) == 0 {
		tokenSHA = req.Form.Get("access_token")
	}
	if len(tokenSHA) == 0 {
		// Well, check with header again.
		auHead := req.Header.Get("Authorization")
		if len(auHead) > 0 {
			auths := strings.Fields(auHead)
			if len(auths) == 2 && (auths[0] == "token" || strings.ToLower(auths[0]) == "bearer") {
				tokenSHA = auths[1]
			}
		}
	}
	if len(tokenSHA) == 0 {
		return 0
	}

	// Let's see if token is valid.
	if strings.Contains(tokenSHA, ".") {
		uid := CheckOAuthAccessToken(tokenSHA)
		if uid != 0 {
			store.GetData()["IsApiToken"] = true
		}
		return uid
	}
	t, err := models.GetAccessTokenBySHA(tokenSHA)
	if err != nil {
		if !models.IsErrAccessTokenNotExist(err) && !models.IsErrAccessTokenEmpty(err) {
			log.Error("GetAccessTokenBySHA: %v", err)
		}
		return 0
	}
	t.UpdatedUnix = timeutil.TimeStampNow()
	if err = models.UpdateAccessToken(t); err != nil {
		log.Error("UpdateAccessToken: %v", err)
	}
	store.GetData()["IsApiToken"] = true
	return t.UID
}

// Verify extracts the user ID from the OAuth token in the query parameters
// or the "Authorization" header and returns the corresponding user object for that ID.
// If verification is successful returns an existing user object.
// Returns nil if verification fails.
func (o *OAuth2) Verify(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) *models.User {
	if !models.HasEngine {
		return nil
	}

	if !middleware.IsAPIPath(req) && !isAttachmentDownload(req) {
		return nil
	}

	id := o.userIDFromToken(req, store)
	if id <= 0 {
		return nil
	}
	log.Trace("OAuth2 Authorization: Found token for user[%d]", id)

	user, err := models.GetUserByID(id)
	if err != nil {
		if !models.IsErrUserNotExist(err) {
			log.Error("GetUserByName: %v", err)
		}
		return nil
	}

	log.Trace("OAuth2 Authorization: Logged in user %-v", user)
	return user
}
