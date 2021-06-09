// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"io/ioutil"

	myCtx "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/migrations"
	"github.com/goccy/go-json"
)

// RestoreRepo restore a repository from data
func RestoreRepo(ctx *myCtx.PrivateContext) {
	bs, err := ioutil.ReadAll(ctx.Req.Body)
	if err != nil {
		ctx.JSON(500, map[string]string{
			"err": err.Error(),
		})
		return
	}
	var params = struct {
		RepoDir   string
		OwnerName string
		RepoName  string
		Units     []string
	}{}
	if err = json.Unmarshal(bs, &params); err != nil {
		ctx.JSON(500, map[string]string{
			"err": err.Error(),
		})
		return
	}

	if err := migrations.RestoreRepository(
		ctx,
		params.RepoDir,
		params.OwnerName,
		params.RepoName,
		params.Units,
	); err != nil {
		ctx.JSON(500, map[string]string{
			"err": err.Error(),
		})
	} else {
		ctx.Status(200)
	}
}
