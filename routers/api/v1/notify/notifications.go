// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package notify

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/utils"
)

// NewAvailable check if unread notifications exist
func NewAvailable(ctx *context.APIContext) {
	// swagger:operation GET /notifications/new notification notifyNewAvailable
	// ---
	// summary: Check if unread notifications exist
	// responses:
	//   "200":
	//     "$ref": "#/responses/NotificationCount"
	ctx.JSON(http.StatusOK, api.NotificationCount{New: models.CountUnread(ctx.User)})
}

func getFindNotificationOptions(ctx *context.APIContext) *models.FindNotificationOptions {
	before, since, err := utils.GetQueryBeforeSince(ctx)
	if err != nil {
		ctx.Error(http.StatusUnprocessableEntity, "GetQueryBeforeSince", err)
		return nil
	}
	opts := &models.FindNotificationOptions{
		ListOptions:       utils.GetListOptions(ctx),
		UserID:            ctx.User.ID,
		UpdatedBeforeUnix: before,
		UpdatedAfterUnix:  since,
	}
	if !ctx.QueryBool("all") {
		statuses := ctx.QueryStrings("status-types")
		opts.Status = statusStringsToNotificationStatuses(statuses, []string{"unread", "pinned"})
	}

	subjectTypes := ctx.QueryStrings("subject-type")
	if len(subjectTypes) != 0 {
		opts.Source = subjectToSource(subjectTypes)
	}

	return opts
}

func subjectToSource(value []string) (result []models.NotificationSource) {
	for _, v := range value {
		switch strings.ToLower(v) {
		case "issue":
			result = append(result, models.NotificationSourceIssue)
		case "pull":
			result = append(result, models.NotificationSourcePullRequest)
		case "commit":
			result = append(result, models.NotificationSourceCommit)
		case "repository":
			result = append(result, models.NotificationSourceRepository)
		}
	}
	return
}
