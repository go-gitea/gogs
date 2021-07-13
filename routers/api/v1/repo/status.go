// Copyright 2017 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/repofiles"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// NewCommitStatus creates a new CommitStatus
func NewCommitStatus(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/statuses/{sha} repository repoCreateStatus
	// ---
	// summary: Create a commit status
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
	// - name: sha
	//   in: path
	//   description: sha of the commit
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateStatusOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/CommitStatus"
	//   "400":
	//     "$ref": "#/responses/error"

	form := web.GetForm(ctx).(*api.CreateStatusOption)
	sha := ctx.Params("sha")
	if len(sha) == 0 {
		ctx.Error(http.StatusBadRequest, "sha not given", nil)
		return
	}
	status := &models.CommitStatus{
		State:       api.CommitStatusState(form.State),
		TargetURL:   form.TargetURL,
		Description: form.Description,
		Context:     form.Context,
	}
	if err := repofiles.CreateCommitStatus(ctx.Repo.Repository, ctx.User, sha, status); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateCommitStatus", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToCommitStatus(status))
}

// GetCommitStatuses returns all statuses for any given commit hash
func GetCommitStatuses(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/statuses/{sha} repository repoListStatuses
	// ---
	// summary: Get a commit's statuses
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
	// - name: sha
	//   in: path
	//   description: sha of the commit
	//   type: string
	//   required: true
	// - name: sort
	//   in: query
	//   description: type of sort
	//   type: string
	//   enum: [oldest, recentupdate, leastupdate, leastindex, highestindex]
	//   required: false
	// - name: state
	//   in: query
	//   description: type of state
	//   type: string
	//   enum: [pending, success, error, failure, warning]
	//   required: false
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
	//     "$ref": "#/responses/CommitStatusList"
	//   "400":
	//     "$ref": "#/responses/error"

	getCommitStatuses(ctx, ctx.Params("sha"))
}

// GetCommitStatusesByRef returns all statuses for any given commit ref
func GetCommitStatusesByRef(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{ref}/statuses repository repoListStatusesByRef
	// ---
	// summary: Get a commit's statuses, by branch/tag/commit reference
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
	// - name: ref
	//   in: path
	//   description: name of branch/tag/commit
	//   type: string
	//   required: true
	// - name: sort
	//   in: query
	//   description: type of sort
	//   type: string
	//   enum: [oldest, recentupdate, leastupdate, leastindex, highestindex]
	//   required: false
	// - name: state
	//   in: query
	//   description: type of state
	//   type: string
	//   enum: [pending, success, error, failure, warning]
	//   required: false
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
	//     "$ref": "#/responses/CommitStatusList"
	//   "400":
	//     "$ref": "#/responses/error"

	filter := utils.ResolveRefOrSha(ctx, ctx.Params("ref"))
	if ctx.Written() {
		return
	}

	getCommitStatuses(ctx, filter) //By default filter is maybe the raw SHA
}

func getCommitStatuses(ctx *context.APIContext, sha string) {
	if len(sha) == 0 {
		ctx.Error(http.StatusBadRequest, "ref/sha not given", nil)
		return
	}
	repo := ctx.Repo.Repository

	listOptions := utils.GetListOptions(ctx)

	statuses, maxResults, err := models.GetCommitStatuses(repo, sha, &models.CommitStatusOptions{
		ListOptions: listOptions,
		SortType:    ctx.QueryTrim("sort"),
		State:       ctx.QueryTrim("state"),
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommitStatuses", fmt.Errorf("GetCommitStatuses[%s, %s, %d]: %v", repo.FullName(), sha, ctx.QueryInt("page"), err))
		return
	}

	apiStatuses := make([]*api.CommitStatus, 0, len(statuses))
	for _, status := range statuses {
		apiStatuses = append(apiStatuses, convert.ToCommitStatus(status))
	}

	ctx.SetLinkHeader(int(maxResults), listOptions.PageSize)
	ctx.Header().Set("X-Total-Count", fmt.Sprintf("%d", maxResults))
	ctx.Header().Set("Access-Control-Expose-Headers", "X-Total-Count, Link")

	ctx.JSON(http.StatusOK, apiStatuses)
}

// GetCombinedCommitStatusByRef returns the combined status for any given commit hash
func GetCombinedCommitStatusByRef(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/commits/{ref}/status repository repoGetCombinedStatusByRef
	// ---
	// summary: Get a commit's combined status, by branch/tag/commit reference
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
	// - name: ref
	//   in: path
	//   description: name of branch/tag/commit
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
	//     "$ref": "#/responses/CombinedStatus"
	//   "400":
	//     "$ref": "#/responses/error"

	sha := utils.ResolveRefOrSha(ctx, ctx.Params("ref"))
	if ctx.Written() {
		return
	}

	repo := ctx.Repo.Repository

	statuses, err := models.GetLatestCommitStatus(repo.ID, sha, utils.GetListOptions(ctx))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLatestCommitStatus", fmt.Errorf("GetLatestCommitStatus[%s, %s]: %v", repo.FullName(), sha, err))
		return
	}

	if len(statuses) == 0 {
		ctx.JSON(http.StatusOK, &api.CombinedStatus{})
		return
	}

	combiStatus := convert.ToCombinedStatus(statuses, convert.ToRepo(repo, ctx.Repo.AccessMode))

	ctx.JSON(http.StatusOK, combiStatus)
}
