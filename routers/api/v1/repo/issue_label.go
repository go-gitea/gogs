// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"strconv"

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

	ctx.JSON(http.StatusOK, convert.ToLabelList(issue.Labels))
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

	ctx.JSON(http.StatusOK, convert.ToLabelList(labels))
}

// DeleteIssueLabel delete a label for an issue
func DeleteIssueLabel(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/labels/{name} issue issueRemoveLabel
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
	// - name: name
	//   in: path
	//   description: name or id of the label to remove
	//   type: string
	//   required: true
	// - name: mode
	//   in: query
	//   description: the type of id is id or name
	//   type: string
	//   enum: [id, name, org_name]
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

	var label *models.Label

	switch ctx.Query("mode") {
	case "name":
		label, err = models.GetLabelInRepoByName(ctx.Repo.Repository.ID, ctx.Params(":id"))
		if err != nil {
			if models.IsErrLabelNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "parseLabel", err)
			return
		}
	case "org_name":
		label, err = models.GetLabelInOrgByName(ctx.Repo.Repository.OwnerID, ctx.Params(":id"))
		if err != nil {
			if models.IsErrOrgNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetLabelByIDOrName", err)
			return
		}

	default: // id as default
		label, err = models.GetLabelByID(ctx.ParamsInt64(":id"))
		if err != nil {
			if models.IsErrLabelNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "parseLabel", err)
			return
		}
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

	ctx.JSON(http.StatusOK, convert.ToLabelList(labels))
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

	switch form.Mode {
	case "name_only":
		labels, err = models.GetLabelsInRepoByNames(ctx.Repo.Repository.ID, form.Labels)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetLabelByIDsOrNames", err)
			return
		}
	case "org_name_only":
		labels, err = models.GetLabelsInOrgByNames(ctx.Repo.Repository.OwnerID, form.Labels)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetLabelByIDsOrNames", err)
			return
		}
	default: // id_only as default
		ids := make([]int64, 0, len(form.Labels))
		for _, q := range form.Labels {
			if len(q) == 0 {
				continue
			}

			var id int64
			id, err = strconv.ParseInt(q, 10, 64)
			if err == nil && id > 0 {
				ids = append(ids, id)
				continue
			}
		}

		labels, err = models.GetLabelsByIDs(ids)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetLabelByIDsOrNames", err)
			return
		}
	}

	if len(labels) == 0 {
		ctx.NotFound()
		return
	}

	if !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) {
		ctx.Status(http.StatusForbidden)
		return
	}

	return
}
