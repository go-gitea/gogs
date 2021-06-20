// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	releaseservice "code.gitea.io/gitea/services/release"
)

// GetRelease get a single release of a repository
func GetRelease(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases/{id} repository repoGetRelease
	// ---
	// summary: Get a release
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
	//   description: id of the release to get
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Release"
	//   "404":
	//     "$ref": "#/responses/notFound"

	id := ctx.ParamsInt64(":id")
	release, err := models.GetReleaseByID(id)
	if err != nil && !models.IsErrReleaseNotExist(err) {
		ctx.Error(http.StatusInternalServerError, "GetReleaseByID", err)
		return
	}
	if err != nil && models.IsErrReleaseNotExist(err) ||
		release.IsTag || release.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound()
		return
	}

	if err := release.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToRelease(release))
}

// ListReleases list a repository's releases
func ListReleases(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/releases repository repoListReleases
	// ---
	// summary: List a repo's releases
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
	// - name: draft
	//   in: query
	//   description: filter (exclude / include) drafts, if you dont have repo write access none will show
	//   type: boolean
	// - name: pre-release
	//   in: query
	//   description: filter (exclude / include) pre-releases
	//   type: boolean
	// - name: per_page
	//   in: query
	//   description: page size of results, deprecated - use limit
	//   type: integer
	//   deprecated: true
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
	//     "$ref": "#/responses/ReleaseList"
	listOptions := utils.GetListOptions(ctx)
	if listOptions.PageSize == 0 && ctx.QueryInt("per_page") != 0 {
		listOptions.PageSize = ctx.QueryInt("per_page")
	}

	opts := models.FindReleasesOptions{
		ListOptions:   listOptions,
		IncludeDrafts: ctx.Repo.AccessMode >= models.AccessModeWrite,
		IncludeTags:   false,
		IsDraft:       ctx.QueryOptionalBool("draft"),
		IsPreRelease:  ctx.QueryOptionalBool("pre-release"),
	}

	releases, err := models.GetReleasesByRepoID(ctx.Repo.Repository.ID, opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetReleasesByRepoID", err)
		return
	}
	rels := make([]*api.Release, len(releases))
	for i, release := range releases {
		if err := release.LoadAttributes(); err != nil {
			ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
			return
		}
		rels[i] = convert.ToRelease(release)
	}

	filteredCount, err := models.CountReleasesByRepoID(ctx.Repo.Repository.ID, opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.SetLinkHeader(int(filteredCount), listOptions.PageSize)
	ctx.Header().Set("X-Total-Count", fmt.Sprint(filteredCount))
	ctx.Header().Set("Access-Control-Expose-Headers", "X-Total-Count, Link")
	ctx.JSON(http.StatusOK, rels)
}

// CreateRelease create a release
func CreateRelease(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/releases repository repoCreateRelease
	// ---
	// summary: Create a release
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
	//     "$ref": "#/definitions/CreateReleaseOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Release"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "409":
	//     "$ref": "#/responses/error"
	form := web.GetForm(ctx).(*api.CreateReleaseOption)
	rel, err := models.GetRelease(ctx.Repo.Repository.ID, form.TagName)
	if err != nil {
		if !models.IsErrReleaseNotExist(err) {
			ctx.Error(http.StatusInternalServerError, "GetRelease", err)
			return
		}
		// If target is not provided use default branch
		if len(form.Target) == 0 {
			form.Target = ctx.Repo.Repository.DefaultBranch
		}
		rel = &models.Release{
			RepoID:       ctx.Repo.Repository.ID,
			PublisherID:  ctx.User.ID,
			Publisher:    ctx.User,
			TagName:      form.TagName,
			Target:       form.Target,
			Title:        form.Title,
			Note:         form.Note,
			IsDraft:      form.IsDraft,
			IsPrerelease: form.IsPrerelease,
			IsTag:        false,
			Repo:         ctx.Repo.Repository,
		}
		if err := releaseservice.CreateRelease(ctx.Repo.GitRepo, rel, nil, ""); err != nil {
			if models.IsErrReleaseAlreadyExist(err) {
				ctx.Error(http.StatusConflict, "ReleaseAlreadyExist", err)
			} else {
				ctx.Error(http.StatusInternalServerError, "CreateRelease", err)
			}
			return
		}
	} else {
		if !rel.IsTag {
			ctx.Error(http.StatusConflict, "GetRelease", "Release is has no Tag")
			return
		}

		rel.Title = form.Title
		rel.Note = form.Note
		rel.IsDraft = form.IsDraft
		rel.IsPrerelease = form.IsPrerelease
		rel.PublisherID = ctx.User.ID
		rel.IsTag = false
		rel.Repo = ctx.Repo.Repository
		rel.Publisher = ctx.User

		if err = releaseservice.UpdateRelease(ctx.User, ctx.Repo.GitRepo, rel, nil, nil, nil); err != nil {
			ctx.Error(http.StatusInternalServerError, "UpdateRelease", err)
			return
		}
	}
	ctx.JSON(http.StatusCreated, convert.ToRelease(rel))
}

// EditRelease edit a release
func EditRelease(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/releases/{id} repository repoEditRelease
	// ---
	// summary: Update a release
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
	// - name: id
	//   in: path
	//   description: id of the release to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditReleaseOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Release"
	//   "404":
	//     "$ref": "#/responses/notFound"

	form := web.GetForm(ctx).(*api.EditReleaseOption)
	id := ctx.ParamsInt64(":id")
	rel, err := models.GetReleaseByID(id)
	if err != nil && !models.IsErrReleaseNotExist(err) {
		ctx.Error(http.StatusInternalServerError, "GetReleaseByID", err)
		return
	}
	if err != nil && models.IsErrReleaseNotExist(err) ||
		rel.IsTag || rel.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound()
		return
	}

	if len(form.TagName) > 0 {
		rel.TagName = form.TagName
	}
	if len(form.Target) > 0 {
		rel.Target = form.Target
	}
	if len(form.Title) > 0 {
		rel.Title = form.Title
	}
	if len(form.Note) > 0 {
		rel.Note = form.Note
	}
	if form.IsDraft != nil {
		rel.IsDraft = *form.IsDraft
	}
	if form.IsPrerelease != nil {
		rel.IsPrerelease = *form.IsPrerelease
	}
	if err := releaseservice.UpdateRelease(ctx.User, ctx.Repo.GitRepo, rel, nil, nil, nil); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateRelease", err)
		return
	}

	rel, err = models.GetReleaseByID(id)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetReleaseByID", err)
		return
	}
	if err := rel.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToRelease(rel))
}

// DeleteRelease delete a release from a repository
func DeleteRelease(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/releases/{id} repository repoDeleteRelease
	// ---
	// summary: Delete a release
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
	//   description: id of the release to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	id := ctx.ParamsInt64(":id")
	rel, err := models.GetReleaseByID(id)
	if err != nil && !models.IsErrReleaseNotExist(err) {
		ctx.Error(http.StatusInternalServerError, "GetReleaseByID", err)
		return
	}
	if err != nil && models.IsErrReleaseNotExist(err) ||
		rel.IsTag || rel.RepoID != ctx.Repo.Repository.ID {
		ctx.NotFound()
		return
	}
	if err := releaseservice.DeleteReleaseByID(id, ctx.User, false); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteReleaseByID", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
