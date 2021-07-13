// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	"code.gitea.io/gitea/services/webhook"
)

// ListHooks list all hooks of a repository
func ListHooks(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/hooks repository repoListHooks
	// ---
	// summary: List the hooks in a repository
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
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/HookList"

	hooks, err := models.GetWebhooksByRepoID(ctx.Repo.Repository.ID, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetWebhooksByRepoID", err)
		return
	}

	apiHooks := make([]*api.Hook, len(hooks))
	for i := range hooks {
		apiHooks[i] = convert.ToHook(ctx.Repo.RepoLink, hooks[i])
	}
	ctx.JSON(http.StatusOK, &apiHooks)
}

// GetHook get a repo's hook by id
func GetHook(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/hooks/{id} repository repoGetHook
	// ---
	// summary: Get a hook
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
	// - name: id
	//   in: path
	//   description: id of the hook to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Hook"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo
	hookID := ctx.ParamsInt64(":id")
	hook, err := utils.GetRepoHook(ctx, repo.Repository.ID, hookID)
	if err != nil {
		return
	}
	ctx.JSON(http.StatusOK, convert.ToHook(repo.RepoLink, hook))
}

// TestHook tests a hook
func TestHook(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/hooks/{id}/tests repository repoTestHook
	// ---
	// summary: Test a push webhook
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
	// - name: id
	//   in: path
	//   description: id of the hook to test
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"

	if ctx.Repo.Commit == nil {
		// if repo does not have any commits, then don't send a webhook
		ctx.Status(http.StatusNoContent)
		return
	}

	hookID := ctx.ParamsInt64(":id")
	hook, err := utils.GetRepoHook(ctx, ctx.Repo.Repository.ID, hookID)
	if err != nil {
		return
	}

	commit := convert.ToPayloadCommit(ctx.Repo.Repository, ctx.Repo.Commit)

	if err := webhook.PrepareWebhook(hook, ctx.Repo.Repository, models.HookEventPush, &api.PushPayload{
		Ref:        git.BranchPrefix + ctx.Repo.Repository.DefaultBranch,
		Before:     ctx.Repo.Commit.ID.String(),
		After:      ctx.Repo.Commit.ID.String(),
		Commits:    []*api.PayloadCommit{commit},
		HeadCommit: commit,
		Repo:       convert.ToRepo(ctx.Repo.Repository, models.AccessModeNone),
		Pusher:     convert.ToUserWithAccessMode(ctx.User, models.AccessModeNone),
		Sender:     convert.ToUserWithAccessMode(ctx.User, models.AccessModeNone),
	}); err != nil {
		ctx.Error(http.StatusInternalServerError, "PrepareWebhook: ", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CreateHook create a hook for a repository
func CreateHook(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/hooks repository repoCreateHook
	// ---
	// summary: Create a hook
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
	//   schema:
	//     "$ref": "#/definitions/CreateHookOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Hook"
	form := web.GetForm(ctx).(*api.CreateHookOption)

	if !utils.CheckCreateHookOption(ctx, form) {
		return
	}
	utils.AddRepoHook(ctx, form)
}

// EditHook modify a hook of a repository
func EditHook(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/hooks/{id} repository repoEditHook
	// ---
	// summary: Edit a hook in a repository
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
	// - name: id
	//   in: path
	//   description: index of the hook
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditHookOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Hook"
	form := web.GetForm(ctx).(*api.EditHookOption)
	hookID := ctx.ParamsInt64(":id")
	utils.EditRepoHook(ctx, form, hookID)
}

// DeleteHook delete a hook of a repository
func DeleteHook(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/hooks/{id} repository repoDeleteHook
	// ---
	// summary: Delete a hook in a repository
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
	// - name: id
	//   in: path
	//   description: id of the hook to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"
	if err := models.DeleteWebhookByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id")); err != nil {
		if models.IsErrWebhookNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteWebhookByRepoID", err)
		}
		return
	}
	ctx.Status(http.StatusNoContent)
}
