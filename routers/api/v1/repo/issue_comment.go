// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
	comment_service "code.gitea.io/gitea/services/comments"
)

// ListIssueComments list all the comments of an issue
func ListIssueComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/comments issue issueGetComments
	// ---
	// summary: List all comments on an issue
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
	// - name: since
	//   in: query
	//   description: if provided, only comments updated since the specified time are returned.
	//   type: string
	//   format: date-time
	// - name: before
	//   in: query
	//   description: if provided, only comments updated before the provided time are returned.
	//   type: string
	//   format: date-time
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommentList"

	before, since, err := utils.GetQueryBeforeSince(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetQueryBeforeSince", err)
		return
	}
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetRawIssueByIndex", err)
		return
	}
	issue.Repo = ctx.Repo.Repository

	comments, err := models.FindComments(models.FindCommentsOptions{
		IssueID: issue.ID,
		Since:   since,
		Before:  before,
		Type:    models.CommentTypeComment,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindComments", err)
		return
	}

	if err := models.CommentList(comments).LoadPosters(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadPosters", err)
		return
	}

	apiComments := make([]*api.Comment, len(comments))
	for i, comment := range comments {
		comment.Issue = issue
		apiComments[i] = comments[i].APIFormat()
	}
	ctx.JSON(http.StatusOK, &apiComments)
}

// ListRepoIssueComments returns all issue-comments for a repo
func ListRepoIssueComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/comments issue issueGetRepoComments
	// ---
	// summary: List all comments in a repository
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
	// - name: since
	//   in: query
	//   description: if provided, only comments updated since the provided time are returned.
	//   type: string
	//   format: date-time
	// - name: before
	//   in: query
	//   description: if provided, only comments updated before the provided time are returned.
	//   type: string
	//   format: date-time
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results, maximum page size is 50
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/CommentList"

	before, since, err := utils.GetQueryBeforeSince(ctx)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetQueryBeforeSince", err)
		return
	}

	comments, err := models.FindComments(models.FindCommentsOptions{
		ListOptions: utils.GetListOptions(ctx),
		RepoID:      ctx.Repo.Repository.ID,
		Type:        models.CommentTypeComment,
		Since:       since,
		Before:      before,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindComments", err)
		return
	}

	if err = models.CommentList(comments).LoadPosters(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadPosters", err)
		return
	}

	apiComments := make([]*api.Comment, len(comments))
	if err := models.CommentList(comments).LoadIssues(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssues", err)
		return
	}
	if err := models.CommentList(comments).LoadPosters(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadPosters", err)
		return
	}
	if _, err := models.CommentList(comments).Issues().LoadRepositories(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepositories", err)
		return
	}
	for i := range comments {
		apiComments[i] = comments[i].APIFormat()
	}
	ctx.JSON(http.StatusOK, &apiComments)
}

// CreateIssueComment create a comment for an issue
func CreateIssueComment(ctx *context.APIContext, form api.CreateIssueCommentOption) {
	// swagger:operation POST /repos/{owner}/{repo}/issues/{index}/comments issue issueCreateComment
	// ---
	// summary: Add a comment to an issue
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
	//     "$ref": "#/definitions/CreateIssueCommentOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Comment"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		return
	}

	if issue.IsLocked && !ctx.Repo.CanWriteIssuesOrPulls(issue.IsPull) && !ctx.User.IsAdmin {
		ctx.Error(http.StatusForbidden, "CreateIssueComment", errors.New(ctx.Tr("repo.issues.comment_on_locked")))
		return
	}

	comment, err := comment_service.CreateIssueComment(ctx.User, ctx.Repo.Repository, issue, form.Body, nil)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateIssueComment", err)
		return
	}

	ctx.JSON(http.StatusCreated, comment.APIFormat())
}

// GetIssueComment Get a comment by ID
func GetIssueComment(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/comments/{id} issue issueGetComment
	// ---
	// summary: Get a comment
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
	//   description: id of the comment
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Comment"
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	if err = comment.LoadIssue(); err != nil {
		ctx.InternalServerError(err)
		return
	}
	if comment.Issue.RepoID != ctx.Repo.Repository.ID {
		ctx.Status(http.StatusNotFound)
		return
	}

	if comment.Type != models.CommentTypeComment {
		ctx.Status(http.StatusNoContent)
		return
	}

	if err := comment.LoadPoster(); err != nil {
		ctx.Error(http.StatusInternalServerError, "comment.LoadPoster", err)
		return
	}

	ctx.JSON(http.StatusOK, comment.APIFormat())
}

// EditIssueComment modify a comment of an issue
func EditIssueComment(ctx *context.APIContext, form api.EditIssueCommentOption) {
	// swagger:operation PATCH /repos/{owner}/{repo}/issues/comments/{id} issue issueEditComment
	// ---
	// summary: Edit a comment
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
	//   description: id of the comment to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditIssueCommentOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Comment"
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	editIssueComment(ctx, form)
}

// EditIssueCommentDeprecated modify a comment of an issue
func EditIssueCommentDeprecated(ctx *context.APIContext, form api.EditIssueCommentOption) {
	// swagger:operation PATCH /repos/{owner}/{repo}/issues/{index}/comments/{id} issue issueEditCommentDeprecated
	// ---
	// summary: Edit a comment
	// deprecated: true
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
	//   description: this parameter is ignored
	//   type: integer
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the comment to edit
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditIssueCommentOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/Comment"
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	editIssueComment(ctx, form)
}

func editIssueComment(ctx *context.APIContext, form api.EditIssueCommentOption) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != comment.PosterID && !ctx.Repo.IsAdmin()) {
		ctx.Status(http.StatusForbidden)
		return
	} else if comment.Type != models.CommentTypeComment {
		ctx.Status(http.StatusNoContent)
		return
	}

	oldContent := comment.Content
	comment.Content = form.Body
	if err := comment_service.UpdateComment(comment, ctx.User, oldContent); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateComment", err)
		return
	}
	_ = comment.LoadPoster()

	ctx.JSON(http.StatusOK, comment.APIFormat())
}

// DeleteIssueComment delete a comment from an issue
func DeleteIssueComment(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/comments/{id} issue issueDeleteComment
	// ---
	// summary: Delete a comment
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
	//   description: id of comment to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	deleteIssueComment(ctx)
}

// DeleteIssueCommentDeprecated delete a comment from an issue
func DeleteIssueCommentDeprecated(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/issues/{index}/comments/{id} issue issueDeleteCommentDeprecated
	// ---
	// summary: Delete a comment
	// deprecated: true
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
	//   description: this parameter is ignored
	//   type: integer
	//   required: true
	// - name: id
	//   in: path
	//   description: id of comment to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"

	deleteIssueComment(ctx)
}

func deleteIssueComment(ctx *context.APIContext) {
	comment, err := models.GetCommentByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrCommentNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetCommentByID", err)
		}
		return
	}

	if !ctx.IsSigned || (ctx.User.ID != comment.PosterID && !ctx.Repo.IsAdmin()) {
		ctx.Status(http.StatusForbidden)
		return
	} else if comment.Type != models.CommentTypeComment {
		ctx.Status(http.StatusNoContent)
		return
	}

	if err = comment_service.DeleteComment(comment, ctx.User); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteCommentByID", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
