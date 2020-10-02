// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/repofiles"
	api "code.gitea.io/gitea/modules/structs"
)

// ApplyDiffPatch handles API call for applying a patch
func ApplyDiffPatch(ctx *context.APIContext, apiOpts api.ApplyDiffPatchFileOptions) {
	// swagger:operation POST /repos/{owner}/{repo}/diffpatch repository repoApplyDiffPatch
	// ---
	// summary: Apply diff patch to repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/UpdateFileOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/FileResponse"
	opts := &repofiles.ApplyDiffPatchOptions{
		Content:   apiOpts.Content,
		SHA:       apiOpts.SHA,
		Message:   apiOpts.Message,
		OldBranch: apiOpts.BranchName,
		NewBranch: apiOpts.NewBranchName,
		Committer: &repofiles.IdentityOptions{
			Name:  apiOpts.Committer.Name,
			Email: apiOpts.Committer.Email,
		},
		Author: &repofiles.IdentityOptions{
			Name:  apiOpts.Author.Name,
			Email: apiOpts.Author.Email,
		},
		Dates: &repofiles.CommitDateOptions{
			Author:    apiOpts.Dates.Author,
			Committer: apiOpts.Dates.Committer,
		},
	}
	if opts.Dates.Author.IsZero() {
		opts.Dates.Author = time.Now()
	}
	if opts.Dates.Committer.IsZero() {
		opts.Dates.Committer = time.Now()
	}

	if opts.Message == "" {
		opts.Message = "apply-patch"
	}

	if !canWriteFiles(ctx.Repo) {
		ctx.Error(http.StatusInternalServerError, "ApplyPatch", models.ErrUserDoesNotHaveAccessToRepo{
			UserID:   ctx.User.ID,
			RepoName: ctx.Repo.Repository.LowerName,
		})
	}

	if fileResponse, err := repofiles.ApplyDiffPatch(ctx.Repo.Repository, ctx.User, opts); err != nil {
		ctx.Error(http.StatusInternalServerError, "ApplyPatch", err)
	} else {
		ctx.JSON(http.StatusCreated, fileResponse)
	}
}
