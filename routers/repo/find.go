// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
)

const (
	tplFindFiles base.TplName = "repo/find/files"
)

// FindFiles render the page to find repository files
func FindFiles(ctx *context.Context) {
	ctx.Data["PageIsFindFiles"] = true
	ctx.Data["PageIsViewCode"] = true

	branchLink := ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()
	treeLink := branchLink

	if len(ctx.Repo.TreePath) > 0 {
		treeLink += "/" + ctx.Repo.TreePath
	}

	ctx.Data["BranchName"] = ctx.Repo.BranchName
	ctx.Data["OwnerName"] = ctx.Repo.Owner.Name
	ctx.Data["RepoName"] = ctx.Repo.Repository.Name

	ctx.Data["RepoLink"] = ctx.Repo.RepoLink
	ctx.Data["TreeLink"] = treeLink

	ctx.HTML(200, tplFindFiles)
}
