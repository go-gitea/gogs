// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	issue_service "code.gitea.io/gitea/services/issue"
)

// ListIssueLabels list all the labels of an issue
func ListIssueLabels(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/labels issue issueGetLabels
	// ---
	// summary: Get an issue's labels
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/LabelList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if err := issue.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}

	repoCache := make(map[int64]*models.Repository)
	repoCache[ctx.Repo.Repository.ID] = ctx.Repo.Repository
	ctx.JSON(http.StatusOK, convert.ToLabelList(issue.Labels, repoCache, nil))
}

// AddIssueLabels add labels for an issue
func AddIssueLabels(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/labels issue issueAddLabel
	// ---
	// summary: Add a label to an issue
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/IssueLabelsOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/LabelList"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	form := web.GetForm(ctx).(*api.IssueLabelsOption)
	issue, labels, err := prepareForReplaceOrAdd(ctx, *form)
	if err != nil {
		return
	}

	if err = issue_service.AddLabels(issue, ctx.User, labels); err != nil {
		ctx.Error(http.StatusInternalServerError, "AddLabels", err)
		return
	}

	labels, err = models.GetLabelsByIssueID(issue.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByIssueID", err)
		return
	}

	repoCache := make(map[int64]*models.Repository)
	repoCache[ctx.Repo.Repository.ID] = ctx.Repo.Repository
	ctx.JSON(http.StatusOK, convert.ToLabelList(labels, repoCache, nil))
}

// DeleteIssueLabel delete a label for an issue
func DeleteIssueLabel(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/labels/{id} issue issueRemoveLabel
	// ---
	// summary: Remove a label from an issue
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the label to remove
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Status(http.StatusForbidden)
		return
	}

	label, err := models.GetLabelByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrLabelNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetLabelByID", err)
		}
		return
	}

	if err := issue_service.RemoveLabel(issue, ctx.User, label); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteIssueLabel", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

// ReplaceIssueLabels replace labels for an issue
func ReplaceIssueLabels(ctx *context.APIContext) {
	// swagger:operation PUT /repos/{owner}/{repo}/issues/{index}/labels issue issueReplaceLabels
	// ---
	// summary: Replace an issue's labels
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/IssueLabelsOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/LabelList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	form := web.GetForm(ctx).(*api.IssueLabelsOption)
	issue, labels, err := prepareForReplaceOrAdd(ctx, *form)
	if err != nil {
		return
	}

	if err := issue_service.ReplaceLabels(issue, ctx.User, labels); err != nil {
		ctx.Error(http.StatusInternalServerError, "ReplaceLabels", err)
		return
	}

	labels, err = models.GetLabelsByIssueID(issue.ID)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByIssueID", err)
		return
	}

	repoCache := make(map[int64]*models.Repository)
	repoCache[ctx.Repo.Repository.ID] = ctx.Repo.Repository
	ctx.JSON(http.StatusOK, convert.ToLabelList(labels, repoCache, nil))
}

// ClearIssueLabels delete all the labels for an issue
func ClearIssueLabels(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/labels issue issueClearLabels
	// ---
	// summary: Remove all labels from an issue
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
	// - name: index
	//   in: path
	//   description: index of the issue
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Status(http.StatusForbidden)
		return
	}

	if err := issue_service.ClearLabels(issue, ctx.User); err != nil {
		ctx.Error(http.StatusInternalServerError, "ClearLabels", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func prepareForReplaceOrAdd(ctx *context.APIContext, form api.IssueLabelsOption) (issue *models.Issue, labels []*models.Label, err error) {
	issue, err = models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	labels, err = models.GetLabelsByIDs(form.Labels)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetLabelsByIDs", err)
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Status(http.StatusForbidden)
		return
	}

	return
}
