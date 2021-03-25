// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sso

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

// Ensure the struct implements the interface.
var (
	_ SingleSignOn = &Basic{}
)

// Basic implements the SingleSignOn interface and authenticates requests (API requests
// only) by looking for Basic authentication data or "x-oauth-basic" token in the "Authorization"
// header.
type Basic struct{}

// Init does nothing as the Basic implementation does not need to allocate any resources
func (b *Basic) Init() error {
	return nil
}

// Free does nothing as the Basic implementation does not have to release any resources
func (b *Basic) Free() error {
	return nil
}

// IsEnabled returns true as this plugin is enabled by default and its not possible to disable
// it from settings.
func (b *Basic) IsEnabled() bool {
	return setting.Service.EnableBasicAuth
}

// VerifyAuthData extracts and validates Basic data (username and password/token) from the
// "Authorization" header of the request and returns the corresponding user object for that
// name/token on successful validation.
// Returns nil if header is empty or validation fails.
func (b *Basic) VerifyAuthData(req *http.Request, w http.ResponseWriter, store DataStore, sess SessionStore) *models.User {
	baHead := req.Header.Get("Authorization")
	if len(baHead) == 0 {
		return nil
	}

	auths := strings.Fields(baHead)
	if len(auths) != 2 || (auths[0] != "Basic" && auths[0] != "basic") {
		return nil
	}

	var u *models.User
	uname, passwd, _ := base.BasicAuthDecode(auths[1])

	// Check if username or password is a token
	isUsernameToken := len(passwd) == 0 || passwd == "x-oauth-basic"
	// Assume username is token
	authToken := uname
	if !isUsernameToken {
		// Assume password is token
		authToken = passwd
	}

	uid := CheckOAuthAccessToken(authToken)
	if uid != 0 {
		var err error
		store.GetData()["IsApiToken"] = true

		u, err = models.GetUserByID(uid)
		if err != nil {
			log.Error("GetUserByID:  %v", err)
			return nil
		}
	}
	token, err := models.GetAccessTokenBySHA(authToken)
	if err == nil {
		u, err = models.GetUserByID(token.UID)
		if err != nil {
			log.Error("GetUserByID:  %v", err)
			return nil
		}

		token.UpdatedUnix = timeutil.TimeStampNow()
		if err = models.UpdateAccessToken(token); err != nil {
			log.Error("UpdateAccessToken:  %v", err)
		}
	} else if !models.IsErrAccessTokenNotExist(err) && !models.IsErrAccessTokenEmpty(err) {
		log.Error("GetAccessTokenBySha: %v", err)
	}

	if u == nil {
		u, err = models.UserSignIn(uname, passwd)
		if err != nil {
			if !models.IsErrUserNotExist(err) {
				log.Error("UserSignIn: %v", err)
			}
			return nil
		}
	} else {
		store.GetData()["IsApiToken"] = true
	}

	return u
}
