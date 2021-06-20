// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package notify

import (
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
)

// ListNotifications list users's notification threads
func ListNotifications(ctx *context.APIContext) {
	// swagger:operation GET /notifications notification notifyGetList
	// ---
	// summary: List users's notification threads
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: all
	//   in: query
	//   description: If true, show notifications marked as read. Default value is false
	//   type: boolean
	// - name: status-types
	//   in: query
	//   description: "Show notifications with the provided status types. Options are: unread, read and/or pinned. Defaults to unread & pinned."
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	// - name: subject-type
	//   in: query
	//   description: "filter notifications by subject type"
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//     enum: [issue,pull,commit,repository]
	// - name: since
	//   in: query
	//   description: Only show notifications updated after the given time. This is a timestamp in RFC 3339 format
	//   type: string
	//   format: date-time
	// - name: before
	//   in: query
	//   description: Only show notifications updated before the given time. This is a timestamp in RFC 3339 format
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
	//     "$ref": "#/responses/NotificationThreadList"
	opts := getFindNotificationOptions(ctx)
	if ctx.Written() {
		return
	}

	nl, err := models.GetNotifications(opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	err = nl.LoadAttributes()
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToNotifications(nl))
}

// ReadNotifications mark notification threads as read, unread, or pinned
func ReadNotifications(ctx *context.APIContext) {
	// swagger:operation PUT /notifications notification notifyReadList
	// ---
	// summary: Mark notification threads as read, pinned or unread
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: last_read_at
	//   in: query
	//   description: Describes the last point that notifications were checked. Anything updated since this time will not be updated.
	//   type: string
	//   format: date-time
	//   required: false
	// - name: all
	//   in: query
	//   description: If true, mark all notifications on this repo. Default value is false
	//   type: string
	//   required: false
	// - name: status-types
	//   in: query
	//   description: "Mark notifications with the provided status types. Options are: unread, read and/or pinned. Defaults to unread."
	//   type: array
	//   collectionFormat: multi
	//   items:
	//     type: string
	//   required: false
	// - name: to-status
	//   in: query
	//   description: Status to mark notifications as, Defaults to read.
	//   type: string
	//   required: false
	// responses:
	//   "205":
	//     "$ref": "#/responses/empty"

	lastRead := int64(0)
	qLastRead := strings.Trim(ctx.Query("last_read_at"), " ")
	if len(qLastRead) > 0 {
		tmpLastRead, err := time.Parse(time.RFC3339, qLastRead)
		if err != nil {
			ctx.InternalServerError(err)
			return
		}
		if !tmpLastRead.IsZero() {
			lastRead = tmpLastRead.Unix()
		}
	}
	opts := &models.FindNotificationOptions{
		UserID:            ctx.User.ID,
		UpdatedBeforeUnix: lastRead,
	}
	if !ctx.QueryBool("all") {
		statuses := ctx.QueryStrings("status-types")
		opts.Status = statusStringsToNotificationStatuses(statuses, []string{"unread"})
	}
	nl, err := models.GetNotifications(opts)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}

	targetStatus := statusStringToNotificationStatus(ctx.Query("to-status"))
	if targetStatus == 0 {
		targetStatus = models.NotificationStatusRead
	}

	for _, n := range nl {
		err := models.SetNotificationStatus(n.ID, ctx.User, targetStatus)
		if err != nil {
			ctx.InternalServerError(err)
			return
		}
		ctx.Status(http.StatusResetContent)
	}

	ctx.Status(http.StatusResetContent)
}
