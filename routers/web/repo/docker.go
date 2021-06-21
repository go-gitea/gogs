// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/auth/oauth2"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/packages/docker"
	"code.gitea.io/gitea/modules/setting"
	"github.com/dgrijalva/jwt-go"
)

// DockerTokenAuth token service for container registry
func DockerTokenAuth(ctx *context.Context) {
	if !setting.Package.EnableRegistry {
		ctx.NotFound("MustEnableDocker", nil)
		return
	}

	var tokenResp struct {
		Token        string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
		ExpiresIn    int    `json:"expires_in,omitempty"`
	}

	signingKey := oauth2.DefaultSigningKey
	if signingKey.IsSymmetric() {
		ctx.ServerError("SymmetricKey", nil)
		return
	}

	authRequest := docker.ResolveScopeList(ctx.Query("scope"))
	if len(authRequest) == 0 {
		if ctx.User == nil {
			ctx.Status(http.StatusUnauthorized)
			return
		}

		idToken := &docker.ClaimSet{
			StandardClaims: jwt.StandardClaims{
				Subject: ctx.User.Name,
			},
		}
		// Authentication-only request ("docker login"), pass through.
		tokenResp.Token, _ = idToken.SignToken(signingKey)
		ctx.JSON(http.StatusOK, tokenResp)
		return
	}

	authResult, err := docker.Authorized(ctx.User, authRequest)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, map[string]string{
			"details": fmt.Sprintf("authorized failed %v", err),
		})
		return
	}

	if len(authResult) == 0 {
		ctx.JSON(http.StatusUnauthorized, map[string]string{
			"details": "requested access to the resource is denied",
		})
		return
	}

	idToken := &docker.ClaimSet{
		Access: authResult,
	}
	if ctx.User != nil {
		idToken.Subject = ctx.User.Name
	}
	if tokenResp.Token, err = idToken.SignToken(signingKey); err != nil {
		ctx.JSON(http.StatusUnauthorized, map[string]string{
			"details": fmt.Sprintf("generate token failed %v", err),
		})
		return
	}
	ctx.JSON(http.StatusOK, tokenResp)
}
