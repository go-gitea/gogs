// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
)

// TopicsPost response for creating repository
func TopicsPost(ctx *context.Context) {
	if ctx.User == nil {
		ctx.JSON(403, map[string]interface{}{
			"message": "Only owners could change the topics.",
		})
		return
	}

	topics := make([]string, 0)
	topicsStr := strings.TrimSpace(ctx.Query("topics"))
	if len(topicsStr) > 0 {
		topics = strings.Split(topicsStr, ",")
	}

	validTopics, invalidTopics := models.SanitizeAndValidateTopics(topics)

	if len(validTopics) > 25 {
		ctx.JSON(422, map[string]interface{}{
			"invalidTopics": nil,
			"message":       ctx.Tr("repo.topic.count_prompt"),
		})
		return
	}

	if len(invalidTopics) > 0 {
		ctx.JSON(422, map[string]interface{}{
			"invalidTopics": invalidTopics,
			"message":       ctx.Tr("repo.topic.format_prompt"),
		})
		return
	}

	err := models.SaveTopics(ctx.Repo.Repository.ID, validTopics...)
	if err != nil {
		log.Error("SaveTopics failed: %v", err)
		ctx.JSON(500, map[string]interface{}{
			"message": "Save topics failed.",
		})
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"status": "ok",
	})
}
