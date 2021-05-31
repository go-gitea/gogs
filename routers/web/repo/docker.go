// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/packages/docker"
	"code.gitea.io/gitea/modules/setting"
)

// DockerTokenAuth token service for container registry
func DockerTokenAuth(ctx *context.Context) {
	if !setting.Docker.Enabled {
		ctx.NotFound("MustEnableDocker", nil)
		return
	}

	// handle params
	service := ctx.Query("service")
	if service != setting.Docker.ServiceName {
		ctx.Status(http.StatusBadRequest)
		return
	}

	var tokenResp struct {
		Token        string `json:"access_token"`
		RefreshToken string `json:"refresh_token,omitempty"`
		ExpiresIn    int    `json:"expires_in,omitempty"`
	}

	issuer := &docker.TokenIssuer{
		Issuer:     setting.Docker.IssuerName,
		Audience:   setting.Docker.ServiceName,
		SigningKey: setting.Docker.PrivateKey,
		Expiration: setting.Docker.Expiration,
	}

	authRequest := docker.ResolveScopeList(ctx.Query("scope"))
	if len(authRequest) == 0 {
		if ctx.User == nil {
			ctx.Status(http.StatusUnauthorized)
			return
		}
		// Authentication-only request ("docker login"), pass through.
		tokenResp.Token, _ = issuer.CreateJWT(ctx.User.Name, authRequest)
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

	var account = ""
	if ctx.User != nil {
		account = ctx.User.Name
	}
	if tokenResp.Token, err = issuer.CreateJWT(account, authResult); err != nil {
		ctx.JSON(http.StatusUnauthorized, map[string]string{
			"details": fmt.Sprintf("generate token failed %v", err),
		})
		return
	}

	tokenResp.ExpiresIn = int(issuer.Expiration)
	ctx.JSON(http.StatusOK, tokenResp)
}
