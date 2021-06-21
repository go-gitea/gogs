// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth/oauth2"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/web"
	"github.com/stretchr/testify/assert"
)

func Test_DockerAuth(t *testing.T) {
	models.PrepareTestEnv(t)

	oauth2.InitSigningKey()

	ctx := test.MockContext(t, "api/docker/token")
	web.SetForm(ctx, map[string]string{
		"service": "gitea-token-service",
		"scope":   "registry:catalog:* repository:library/busybox:pull,push",
	})
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	DockerTokenAuth(ctx)
	assert.True(t, ctx.Written())
}
