// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"encoding/json"
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/packages/docker"
	"code.gitea.io/gitea/modules/setting"

	"github.com/docker/distribution/notifications"
)

// DockerPluginLogin token service for docker registry
func DockerPluginLogin(ctx *context.Context) {
	if !setting.HasDockerPlugin() {
		ctx.Status(404)
		return
	}

	// handle params
	service := ctx.Query("service")
	if service != setting.Docker.ServiceName {
		ctx.Status(http.StatusBadRequest)
		return
	}

	scope := ctx.Query("scope")
	if len(scope) == 0 {
		if ctx.User == nil {
			ctx.Status(http.StatusUnauthorized)
			return
		}
		// Authentication-only request ("docker login"), pass through.
		handleResult(ctx, []docker.AuthzResult{})
		return
	}

	scops, err := docker.SplitScopes(scope)
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}

	results, err := docker.PermissionCheck(ctx.User, scops)
	if err != nil {
		ctx.Error(500, "PermissionCheck")
		log.Error("docker.PermissionCheck: %v", err)
		return
	}

	handleResult(ctx, results)
}

func handleResult(ctx *context.Context, ares []docker.AuthzResult) {
	account := ""
	if ctx.User != nil {
		account = ctx.User.Name
	}
	token, err := docker.GenerateToken(docker.GenerateTokenOptions{
		Account:      account,
		IssuerName:   setting.Docker.IssuerName,
		AuthzResults: ares,
		PublicKey:    &setting.Docker.PublicKey,
		PrivateKey:   &setting.Docker.PrivateKey,
		ServiceName:  setting.Docker.ServiceName,
		Expiration:   setting.Docker.Expiration,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "")
		log.Error("Failed to generate token: %v", err)
		return
	}

	result, _ := json.Marshal(&map[string]string{"access_token": token, "token": token})
	ctx.Header().Set("Content-Type", "application/json")
	_, err = ctx.Write(result)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "generateToken")
		log.Error("Failed to response token: %v", err)
	}
}

// DockerPluginEvent event service for docker registry
func DockerPluginEvent(ctx *context.Context) {
	if !setting.HasDockerPlugin() {
		ctx.Status(404)
		return
	}

	token := ctx.Req.Header.Get("X-Token")
	if token != setting.Docker.NotifyToken {
		ctx.Error(http.StatusUnauthorized)
		return
	}

	decoder := json.NewDecoder(ctx.Req.Body)
	decoder.DisallowUnknownFields()

	data := new(notifications.Envelope)

	err := decoder.Decode(data)
	if err != nil {
		ctx.Error(http.StatusInternalServerError)
		log.Error("Failed to unmarshal events: %v", err)
		return
	}

	if err := docker.HandleEvents(data); err != nil {
		ctx.Error(http.StatusInternalServerError)
		log.Error("Failed to handle events: %v", err)
		return
	}

	// handle events
	ctx.JSON(200, map[string]string{
		"result": "ok",
	})
}
