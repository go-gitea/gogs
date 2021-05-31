// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/web"
	"github.com/stretchr/testify/assert"
)

func Test_DockerAuth(t *testing.T) {
	models.PrepareTestEnv(t)

	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatal(err)
	}
	setting.Docker.ServiceName = "gitea-token-service"
	setting.Docker.PrivateKey = privKey
	setting.Docker.Expiration = 60

	ctx := test.MockContext(t, "api/docker/token")
	web.SetForm(ctx, map[string]string{
		"service": setting.Docker.ServiceName,
		"scope":   "registry:catalog:* repository:library/busybox:pull,push",
	})
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 1)
	DockerTokenAuth(ctx)
	assert.True(t, ctx.Written())
}
