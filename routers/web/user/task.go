// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	jsoniter "github.com/json-iterator/go"
)

// TaskStatus returns task's status
func TaskStatus(ctx *context.Context) {
	task, opts, err := models.GetMigratingTaskByID(ctx.ParamsInt64("task"), ctx.User.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, map[string]interface{}{
			"err": err,
		})
		return
	}

	message := task.Message

	if task.Message != "" && task.Message[0] == '{' {
		// assume message is actually a translatable string
		json := jsoniter.ConfigCompatibleWithStandardLibrary
		var translatableMessage models.TranslatableMessage
		if err := json.Unmarshal([]byte(message), &translatableMessage); err != nil {
			translatableMessage = models.TranslatableMessage{
				Format: "migrate.migrating_failed.error",
				Args:   []interface{}{task.Message},
			}
		}
		message = ctx.Tr(translatableMessage.Format, translatableMessage.Args...)
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"status":    task.Status,
		"message":   message,
		"repo-id":   task.RepoID,
		"repo-name": opts.RepoName,
		"start":     task.StartTime,
		"end":       task.EndTime,
	})
}
