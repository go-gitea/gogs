// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// ListTrackedTimes list all the tracked times of an issue
func ListTrackedTimes(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/issues/{index}/times issue issueTrackedTimes
	// ---
	// summary: List an issue's tracked times
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
	// - name: user
	//   in: query
	//   description: optional filter by user (available for issue managers)
	//   type: string
	// - name: since
	//   in: query
	//   description: Only show times updated after the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
	// - name: before
	//   in: query
	//   description: Only show times updated before the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
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
	//     "$ref": "#/responses/TrackedTimeList"
	//   "404":
	//     "$ref": "#/responses/notFound"

	if !ctx.Repo.Repository.IsTimetrackerEnabled() {
		ctx.NotFound("Timetracker is disabled")
		return
	}
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	opts := models.FindTrackedTimesOptions{
		ListOptions:  utils.GetListOptions(ctx),
		RepositoryID: ctx.Repo.Repository.ID,
		IssueID:      issue.ID,
	}

	qUser := strings.Trim(ctx.Query("user"), " ")
	if qUser != "" {
		user, err := models.GetUserByName(qUser)
		if models.IsErrUserNotExist(err) {
			ctx.Error(http.StatusNotFound, "User does not exist", err)
		} else if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
			return
		}
		opts.UserID = user.ID
	}

	if opts.CreatedBeforeUnix, opts.CreatedAfterUnix, err = utils.GetQueryBeforeSince(ctx); err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "GetQueryBeforeSince", err)
		return
	}

	cantSetUser := !ctx.User.IsAdmin &&
		opts.UserID != ctx.User.ID &&
		!ctx.IsUserRepoWriter([]models.UnitType{models.UnitTypeIssues})

	if cantSetUser {
		if opts.UserID == 0 {
			opts.UserID = ctx.User.ID
		} else {
			ctx.Error(http.StatusForbidden, "", fmt.Errorf("query by user not allowed; not enough rights"))
			return
		}
	}

	trackedTimes, err := models.GetTrackedTimes(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTrackedTimes", err)
		return
	}
	if err = trackedTimes.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToTrackedTimeList(trackedTimes))
}

// AddTime add time manual to the given issue
func AddTime(ctx *context.APIContext) {
	// swagger:operation Post /repos/{owner}/{repo}/issues/{index}/times issue issueAddTime
	// ---
	// summary: Add tracked time to a issue
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
	//     "$ref": "#/definitions/AddTimeOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/TrackedTime"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	form := web.GetForm(ctx).(*api.AddTimeOption)
	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if !ctx.Repo.CanUseTimetracker(issue, ctx.User) {
		if !ctx.Repo.Repository.IsTimetrackerEnabled() {
			ctx.Error(http.StatusBadRequest, "", "time tracking disabled")
			return
		}
		ctx.Status(http.StatusForbidden)
		return
	}

	user := ctx.User
	if form.User != "" {
		if (ctx.IsUserRepoAdmin() && ctx.User.Name != form.User) || ctx.User.IsAdmin {
			//allow only RepoAdmin, Admin and User to add time
			user, err = models.GetUserByName(form.User)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
			}
		}
	}

	created := time.Time{}
	if !form.Created.IsZero() {
		created = form.Created
	}

	trackedTime, err := models.AddTime(user, issue, form.Time, created)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "AddTime", err)
		return
	}
	if err = trackedTime.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToTrackedTime(trackedTime))
}

// ResetIssueTime reset time manual to the given issue
func ResetIssueTime(ctx *context.APIContext) {
	// swagger:operation Delete /repos/{owner}/{repo}/issues/{index}/times issue issueResetTime
	// ---
	// summary: Reset a tracked time of an issue
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
	//   description: index of the issue to add tracked time to
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if !ctx.Repo.CanUseTimetracker(issue, ctx.User) {
		if !ctx.Repo.Repository.IsTimetrackerEnabled() {
			ctx.JSON(http.StatusBadRequest, struct{ Message string }{Message: "time tracking disabled"})
			return
		}
		ctx.Status(http.StatusForbidden)
		return
	}

	err = models.DeleteIssueUserTimes(issue, ctx.User)
	if err != nil {
		if models.IsErrNotExist(err) {
			ctx.Error(http.StatusNotFound, "DeleteIssueUserTimes", err)
		} else {
			ctx.Error(http.StatusInternalServerError, "DeleteIssueUserTimes", err)
		}
		return
	}
	ctx.Status(204)
}

// DeleteTime delete a specific time by id
func DeleteTime(ctx *context.APIContext) {
	// swagger:operation Delete /repos/{owner}/{repo}/issues/{index}/times/{id} issue issueDeleteTime
	// ---
	// summary: Delete specific tracked time
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
	// - name: id
	//   in: path
	//   description: id of time to delete
	//   type: integer
	//   format: int64
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	issue, err := models.GetIssueByIndex(ctx.Repo.Repository.ID, ctx.ParamsInt64(":index"))
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetIssueByIndex", err)
		}
		return
	}

	if !ctx.Repo.CanUseTimetracker(issue, ctx.User) {
		if !ctx.Repo.Repository.IsTimetrackerEnabled() {
			ctx.JSON(http.StatusBadRequest, struct{ Message string }{Message: "time tracking disabled"})
			return
		}
		ctx.Status(http.StatusForbidden)
		return
	}

	time, err := models.GetTrackedTimeByID(ctx.ParamsInt64(":id"))
	if err != nil {
		if models.IsErrNotExist(err) {
			ctx.NotFound(err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetTrackedTimeByID", err)
		return
	}
	if time.Deleted {
		ctx.NotFound(fmt.Errorf("tracked time [%d] already deleted", time.ID))
		return
	}

	if !ctx.User.IsAdmin && time.UserID != ctx.User.ID {
		//Only Admin and User itself can delete their time
		ctx.Status(http.StatusForbidden)
		return
	}

	err = models.DeleteTime(time)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteTime", err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// ListTrackedTimesByUser  lists all tracked times of the user
func ListTrackedTimesByUser(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/times/{user} repository userTrackedTimes
	// ---
	// summary: List a user's tracked times in a repo
	// deprecated: true
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
	// - name: user
	//   in: path
	//   description: username of user
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/TrackedTimeList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	if !ctx.Repo.Repository.IsTimetrackerEnabled() {
		ctx.Error(http.StatusBadRequest, "", "time tracking disabled")
		return
	}
	user, err := models.GetUserByName(ctx.Params(":timetrackingusername"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
		}
		return
	}
	if user == nil {
		ctx.NotFound()
		return
	}

	if !ctx.IsUserRepoAdmin() && !ctx.User.IsAdmin && ctx.User.ID != user.ID {
		ctx.Error(http.StatusForbidden, "", fmt.Errorf("query by user not allowed; not enough rights"))
		return
	}

	opts := models.FindTrackedTimesOptions{
		UserID:       user.ID,
		RepositoryID: ctx.Repo.Repository.ID,
	}

	trackedTimes, err := models.GetTrackedTimes(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTrackedTimes", err)
		return
	}
	if err = trackedTimes.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToTrackedTimeList(trackedTimes))
}

// ListTrackedTimesByRepository lists all tracked times of the repository
func ListTrackedTimesByRepository(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/times repository repoTrackedTimes
	// ---
	// summary: List a repo's tracked times
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
	// - name: user
	//   in: query
	//   description: optional filter by user (available for issue managers)
	//   type: string
	// - name: since
	//   in: query
	//   description: Only show times updated after the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
	// - name: before
	//   in: query
	//   description: Only show times updated before the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
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
	//     "$ref": "#/responses/TrackedTimeList"
	//   "400":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"

	if !ctx.Repo.Repository.IsTimetrackerEnabled() {
		ctx.Error(http.StatusBadRequest, "", "time tracking disabled")
		return
	}

	opts := models.FindTrackedTimesOptions{
		ListOptions:  utils.GetListOptions(ctx),
		RepositoryID: ctx.Repo.Repository.ID,
	}

	// Filters
	qUser := strings.Trim(ctx.Query("user"), " ")
	if qUser != "" {
		user, err := models.GetUserByName(qUser)
		if models.IsErrUserNotExist(err) {
			ctx.Error(http.StatusNotFound, "User does not exist", err)
		} else if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
			return
		}
		opts.UserID = user.ID
	}

	var err error
	if opts.CreatedBeforeUnix, opts.CreatedAfterUnix, err = utils.GetQueryBeforeSince(ctx); err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "GetQueryBeforeSince", err)
		return
	}

	cantSetUser := !ctx.User.IsAdmin &&
		opts.UserID != ctx.User.ID &&
		!ctx.IsUserRepoWriter([]models.UnitType{models.UnitTypeIssues})

	if cantSetUser {
		if opts.UserID == 0 {
			opts.UserID = ctx.User.ID
		} else {
			ctx.Error(http.StatusForbidden, "", fmt.Errorf("query by user not allowed; not enough rights"))
			return
		}
	}

	trackedTimes, err := models.GetTrackedTimes(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTrackedTimes", err)
		return
	}
	if err = trackedTimes.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}
	ctx.JSON(http.StatusOK, convert.ToTrackedTimeList(trackedTimes))
}

// ListMyTrackedTimes lists all tracked times of the current user
func ListMyTrackedTimes(ctx *context.APIContext) {
	// swagger:operation GET /user/times user userCurrentTrackedTimes
	// ---
	// summary: List the current user's tracked times
	// parameters:
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// produces:
	// - application/json
	// parameters:
	// - name: since
	//   in: query
	//   description: Only show times updated after the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
	// - name: before
	//   in: query
	//   description: Only show times updated before the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
	// responses:
	//   "200":
	//     "$ref": "#/responses/TrackedTimeList"

	opts := models.FindTrackedTimesOptions{
		ListOptions: utils.GetListOptions(ctx),
		UserID:      ctx.User.ID,
	}

	var err error
	if opts.CreatedBeforeUnix, opts.CreatedAfterUnix, err = utils.GetQueryBeforeSince(ctx); err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "GetQueryBeforeSince", err)
		return
	}

	trackedTimes, err := models.GetTrackedTimes(opts)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetTrackedTimesByUser", err)
		return
	}

	if err = trackedTimes.LoadAttributes(); err != nil {
		ctx.Error(http.StatusInternalServerError, "LoadAttributes", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToTrackedTimeList(trackedTimes))
}
