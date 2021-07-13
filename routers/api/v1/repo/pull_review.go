// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	issue_service "code.gitea.io/gitea/services/issue"
	pull_service "code.gitea.io/gitea/services/pull"
)

// ListPullReviews lists all reviews of a pull request
func ListPullReviews(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/reviews repository repoListPullReviews
	// ---
	// summary: List all reviews for a pull request
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
	//   description: index of the pull request
	//   type: integer
	//   format: int64
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
	//     "$ref": "#/responses/PullReviewList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if err = pr.LoadIssue(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return
	}

	if err = pr.Issue.LoadRepo(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadRepo", err)
		return
	}

	allReviews, err := models.FindReviews(models.FindReviewOptions{
		ListOptions: utils.GetListOptions(ctx),
		Type:        models.ReviewTypeUnknown,
		IssueID:     pr.IssueID,
	})

	if err != nil {
		ctx.Error(http.StatusInternalServerError, "FindReviews", err)
		return
	}

	apiReviews, err := convert.ToPullReviewList(allReviews, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convertToPullReviewList", err)
		return
	}

	ctx.JSON(http.StatusOK, &apiReviews)
}

// GetPullReview gets a specific review of a pull request
func GetPullReview(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/reviews/{id} repository repoGetPullReview
	// ---
	// summary: Get a specific review for a pull request
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
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the review
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReview"
	//   "404":
	//     "$ref": "#/responses/notFound"

	review, _, statusSet := prepareSingleReview(ctx)
	if statusSet {
		return
	}

	apiReview, err := convert.ToPullReview(review, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convertToPullReview", err)
		return
	}

	ctx.JSON(http.StatusOK, apiReview)
}

// GetPullReviewComments lists all comments of a pull request review
func GetPullReviewComments(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/pulls/{index}/reviews/{id}/comments repository repoGetPullReviewComments
	// ---
	// summary: Get a specific review for a pull request
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
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the review
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReviewCommentList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	review, _, statusSet := prepareSingleReview(ctx)
	if statusSet {
		return
	}

	apiComments, err := convert.ToPullReviewCommentList(review, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convertToPullReviewCommentList", err)
		return
	}

	ctx.JSON(http.StatusOK, apiComments)
}

// DeletePullReview delete a specific review from a pull request
func DeletePullReview(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/pulls/{index}/reviews/{id} repository repoDeletePullReview
	// ---
	// summary: Delete a specific review from a pull request
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
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the review
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

	review, _, statusSet := prepareSingleReview(ctx)
	if statusSet {
		return
	}

	if ctx.User == nil {
		ctx.NotFound()
		return
	}
	if !ctx.User.IsAdmin && ctx.User.ID != review.ReviewerID {
		ctx.Error(http.StatusForbidden, "only admin and user itself can delete a review", nil)
		return
	}

	if err := models.DeleteReview(review); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteReview", fmt.Errorf("can not delete ReviewID: %d", review.ID))
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CreatePullReview create a review to an pull request
func CreatePullReview(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/reviews repository repoCreatePullReview
	// ---
	// summary: Create a review to an pull request
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
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/CreatePullReviewOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReview"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	opts := web.GetForm(ctx).(*api.CreatePullReviewOptions)
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	// determine review type
	reviewType, isWrong := preparePullReviewType(ctx, pr, opts.Event, opts.Body, len(opts.Comments) > 0)
	if isWrong {
		return
	}

	if err := pr.Issue.LoadRepo(); err != nil {
		ctx.Error(http.StatusInternalServerError, "pr.Issue.LoadRepo", err)
		return
	}

	// if CommitID is empty, set it as lastCommitID
	if opts.CommitID == "" {
		gitRepo, err := git.OpenRepository(pr.Issue.Repo.RepoPath())
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "git.OpenRepository", err)
			return
		}
		defer gitRepo.Close()

		headCommitID, err := gitRepo.GetRefCommitID(pr.GetGitRefName())
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetRefCommitID", err)
			return
		}

		opts.CommitID = headCommitID
	}

	// create review comments
	for _, c := range opts.Comments {
		line := c.NewLineNum
		if c.OldLineNum > 0 {
			line = c.OldLineNum * -1
		}

		if _, err := pull_service.CreateCodeComment(
			ctx.User,
			ctx.Repo.GitRepo,
			pr.Issue,
			line,
			c.Body,
			c.Path,
			true, // is review
			0,    // no reply
			opts.CommitID,
		); err != nil {
			ctx.Error(http.StatusInternalServerError, "CreateCodeComment", err)
			return
		}
	}

	// create review and associate all pending review comments
	review, _, err := pull_service.SubmitReview(ctx.User, ctx.Repo.GitRepo, pr.Issue, reviewType, opts.Body, opts.CommitID, nil)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SubmitReview", err)
		return
	}

	// convert response
	apiReview, err := convert.ToPullReview(review, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convertToPullReview", err)
		return
	}
	ctx.JSON(http.StatusOK, apiReview)
}

// SubmitPullReview submit a pending review to an pull request
func SubmitPullReview(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/reviews/{id} repository repoSubmitPullReview
	// ---
	// summary: Submit a pending review to an pull request
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
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the review
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/SubmitPullReviewOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReview"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	opts := web.GetForm(ctx).(*api.SubmitPullReviewOptions)
	review, pr, isWrong := prepareSingleReview(ctx)
	if isWrong {
		return
	}

	if review.Type != models.ReviewTypePending {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("only a pending review can be submitted"))
		return
	}

	// determine review type
	reviewType, isWrong := preparePullReviewType(ctx, pr, opts.Event, opts.Body, len(review.Comments) > 0)
	if isWrong {
		return
	}

	// if review stay pending return
	if reviewType == models.ReviewTypePending {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("review stay pending"))
		return
	}

	headCommitID, err := ctx.Repo.GitRepo.GetRefCommitID(pr.GetGitRefName())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GitRepo: GetRefCommitID", err)
		return
	}

	// create review and associate all pending review comments
	review, _, err = pull_service.SubmitReview(ctx.User, ctx.Repo.GitRepo, pr.Issue, reviewType, opts.Body, headCommitID, nil)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "SubmitReview", err)
		return
	}

	// convert response
	apiReview, err := convert.ToPullReview(review, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convertToPullReview", err)
		return
	}
	ctx.JSON(http.StatusOK, apiReview)
}

// preparePullReviewType return ReviewType and false or nil and true if an error happen
func preparePullReviewType(ctx *context.APIContext, pr *models.PullRequest, event api.ReviewStateType, body string, hasComments bool) (models.ReviewType, bool) {
	if err := pr.LoadIssue(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadIssue", err)
		return -1, true
	}

	needsBody := true
	hasBody := len(strings.TrimSpace(body)) > 0

	var reviewType models.ReviewType
	switch event {
	case api.ReviewStateApproved:
		// can not approve your own PR
		if pr.Issue.IsPoster(ctx.User.ID) {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("approve your own pull is not allowed"))
			return -1, true
		}
		reviewType = models.ReviewTypeApprove
		needsBody = false

	case api.ReviewStateRequestChanges:
		// can not reject your own PR
		if pr.Issue.IsPoster(ctx.User.ID) {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("reject your own pull is not allowed"))
			return -1, true
		}
		reviewType = models.ReviewTypeReject

	case api.ReviewStateComment:
		reviewType = models.ReviewTypeComment
		needsBody = false
		// if there is no body we need to ensure that there are comments
		if !hasBody && !hasComments {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("review event %s requires a body or a comment", event))
			return -1, true
		}
	default:
		reviewType = models.ReviewTypePending
	}

	// reject reviews with empty body if a body is required for this call
	if needsBody && !hasBody {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Errorf("review event %s requires a body", event))
		return -1, true
	}

	return reviewType, false
}

// prepareSingleReview return review, related pull and false or nil, nil and true if an error happen
func prepareSingleReview(ctx *context.APIContext) (*models.Review, *models.PullRequest, bool) {
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return nil, nil, true
	}

	review, err := models.GetReviewByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrReviewNotExist(err) {
			ctx.NotFound("GetReviewByID", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetReviewByID", err)
		}
		return nil, nil, true
	}

	// validate the the review is for the given PR
	if review.IssueID != pr.IssueID {
		ctx.NotFound("ReviewNotInPR")
		return nil, nil, true
	}

	// make sure that the user has access to this review if it is pending
	if review.Type == models.ReviewTypePending && review.ReviewerID != ctx.User.ID && !ctx.User.IsAdmin {
		ctx.NotFound("GetReviewByID")
		return nil, nil, true
	}

	if err := review.LoadAttributes(); err != nil && !models.IsErrUserNotExist(err) {
		ctx.Error(http.StatusInternalServerError, "ReviewLoadAttributes", err)
		return nil, nil, true
	}

	return review, pr, false
}

// CreateReviewRequests create review requests to an pull request
func CreateReviewRequests(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/requested_reviewers repository repoCreatePullReviewRequests
	// ---
	// summary: create review requests for a pull request
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
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/PullReviewRequestOptions"
	// responses:
	//   "201":
	//     "$ref": "#/responses/PullReviewList"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"

	opts := web.GetForm(ctx).(*api.PullReviewRequestOptions)
	apiReviewRequest(ctx, *opts, true)
}

// DeleteReviewRequests delete review requests to an pull request
func DeleteReviewRequests(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/pulls/{index}/requested_reviewers repository repoDeletePullReviewRequests
	// ---
	// summary: cancel review requests for a pull request
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
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/PullReviewRequestOptions"
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "422":
	//     "$ref": "#/responses/validationError"
	//   "404":
	//     "$ref": "#/responses/notFound"
	opts := web.GetForm(ctx).(*api.PullReviewRequestOptions)
	apiReviewRequest(ctx, *opts, false)
}

func apiReviewRequest(ctx *context.APIContext, opts api.PullReviewRequestOptions, isAdd bool) {
	pr, err := models.GetPullRequestByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrPullRequestNotExist(err) {
			ctx.NotFound("GetPullRequestByIndex", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetPullRequestByIndex", err)
		}
		return
	}

	if err := pr.Issue.LoadRepo(); err != nil {
		ctx.Error(http.StatusInternalServerError, "pr.Issue.LoadRepo", err)
		return
	}

	reviewers := make([]*models.User, 0, len(opts.Reviewers))

	permDoer, err := models.GetUserRepoPermission(pr.Issue.Repo, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
		return
	}

	for _, r := range opts.Reviewers {
		var reviewer *models.User
		if strings.Contains(r, "@") {
			reviewer, err = models.GetUserByEmail(r)
		} else {
			reviewer, err = models.GetUserByName(r)
		}

		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.NotFound("UserNotExist", fmt.Sprintf("User '%s' not exist", r))
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetUser", err)
			return
		}

		err = issue_service.IsValidReviewRequest(reviewer, ctx.User, isAdd, pr.Issue, &permDoer)
		if err != nil {
			if models.IsErrNotValidReviewRequest(err) {
				ctx.Error(http.StatusUnprocessableEntity, "NotValidReviewRequest", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "IsValidReviewRequest", err)
			return
		}

		reviewers = append(reviewers, reviewer)
	}

	var reviews []*models.Review
	if isAdd {
		reviews = make([]*models.Review, 0, len(reviewers))
	}

	for _, reviewer := range reviewers {
		comment, err := issue_service.ReviewRequest(pr.Issue, ctx.User, reviewer, isAdd)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "ReviewRequest", err)
			return
		}

		if comment != nil && isAdd {
			if err = comment.LoadReview(); err != nil {
				ctx.ServerError("ReviewRequest", err)
				return
			}
			reviews = append(reviews, comment.Review)
		}
	}

	if ctx.Repo.Repository.Owner.IsOrganization() && len(opts.TeamReviewers) > 0 {

		teamReviewers := make([]*models.Team, 0, len(opts.TeamReviewers))
		for _, t := range opts.TeamReviewers {
			var teamReviewer *models.Team
			teamReviewer, err = models.GetTeam(ctx.Repo.Owner.ID, t)
			if err != nil {
				if models.IsErrTeamNotExist(err) {
					ctx.NotFound("TeamNotExist", fmt.Sprintf("Team '%s' not exist", t))
					return
				}
				ctx.Error(http.StatusInternalServerError, "ReviewRequest", err)
				return
			}

			err = issue_service.IsValidTeamReviewRequest(teamReviewer, ctx.User, isAdd, pr.Issue)
			if err != nil {
				if models.IsErrNotValidReviewRequest(err) {
					ctx.Error(http.StatusUnprocessableEntity, "NotValidReviewRequest", err)
					return
				}
				ctx.Error(http.StatusInternalServerError, "IsValidTeamReviewRequest", err)
				return
			}

			teamReviewers = append(teamReviewers, teamReviewer)
		}

		for _, teamReviewer := range teamReviewers {
			comment, err := issue_service.TeamReviewRequest(pr.Issue, ctx.User, teamReviewer, isAdd)
			if err != nil {
				ctx.ServerError("TeamReviewRequest", err)
				return
			}

			if comment != nil && isAdd {
				if err = comment.LoadReview(); err != nil {
					ctx.ServerError("ReviewRequest", err)
					return
				}
				reviews = append(reviews, comment.Review)
			}
		}
	}

	if isAdd {
		apiReviews, err := convert.ToPullReviewList(reviews, ctx.User)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "convertToPullReviewList", err)
			return
		}
		ctx.JSON(http.StatusCreated, apiReviews)
	} else {
		ctx.Status(http.StatusNoContent)
		return
	}
}

// DismissPullReview dismiss a review for a pull request
func DismissPullReview(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/reviews/{id}/dismissals repository repoDismissPullReview
	// ---
	// summary: Dismiss a review for a pull request
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
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the review
	//   type: integer
	//   format: int64
	//   required: true
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     "$ref": "#/definitions/DismissPullReviewOptions"
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReview"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	opts := web.GetForm(ctx).(*api.DismissPullReviewOptions)
	dismissReview(ctx, opts.Message, true)
}

// UnDismissPullReview cancel to dismiss a review for a pull request
func UnDismissPullReview(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/pulls/{index}/reviews/{id}/undismissals repository repoUnDismissPullReview
	// ---
	// summary: Cancel to dismiss a review for a pull request
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
	//   description: index of the pull request
	//   type: integer
	//   format: int64
	//   required: true
	// - name: id
	//   in: path
	//   description: id of the review
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/PullReview"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"
	dismissReview(ctx, "", false)
}

func dismissReview(ctx *context.APIContext, msg string, isDismiss bool) {
	if !ctx.Repo.IsAdmin() {
		ctx.Error(http.StatusForbidden, "", "Must be repo admin")
		return
	}
	review, pr, isWrong := prepareSingleReview(ctx)
	if isWrong {
		return
	}

	if review.Type != models.ReviewTypeApprove && review.Type != models.ReviewTypeReject {
		ctx.Error(http.StatusForbidden, "", "not need to dismiss this review because it's type is not Approve or change request")
		return
	}

	if pr.Issue.IsClosed {
		ctx.Error(http.StatusForbidden, "", "not need to dismiss this review because this pr is closed")
		return
	}

	_, err := pull_service.DismissReview(review.ID, msg, ctx.User, isDismiss)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "pull_service.DismissReview", err)
		return
	}

	if review, err = models.GetReviewByID(review.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "GetReviewByID", err)
		return
	}

	// convert response
	apiReview, err := convert.ToPullReview(review, ctx.User)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convertToPullReview", err)
		return
	}
	ctx.JSON(http.StatusOK, apiReview)
}
