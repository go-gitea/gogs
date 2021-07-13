// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"io/ioutil"
	"net/http"

	myCtx "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/private"
	jsoniter "github.com/json-iterator/go"
)

// RestoreRepo restore a repository from data
func RestoreRepo(ctx *myCtx.PrivateContext) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	bs, err := ioutil.ReadAll(ctx.Req.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
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
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
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
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
	} else {
		ctx.Status(http.StatusOK)
	}
}
