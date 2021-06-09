// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/goccy/go-json"
	"xorm.io/builder"
	"xorm.io/xorm"
)

func deleteMigrationCredentials(x *xorm.Engine) (err error) {
	const batchSize = 100

	// only match migration tasks, that are not pending or running
	cond := builder.Eq{
		"type": structs.TaskTypeMigrateRepo,
	}.And(builder.Gte{
		"status": structs.TaskStatusStopped,
	})

	sess := x.NewSession()
	defer sess.Close()

	for start := 0; ; start += batchSize {
		tasks := make([]*models.Task, 0, batchSize)
		if err = sess.Limit(batchSize, start).Where(cond, 0).Find(&tasks); err != nil {
			return
		}
		if len(tasks) == 0 {
			break
		}
		if err = sess.Begin(); err != nil {
			return
		}
		for _, t := range tasks {
			if t.PayloadContent, err = removeCredentials(t.PayloadContent); err != nil {
				return
			}
			if _, err = sess.ID(t.ID).Cols("payload_content").Update(t); err != nil {
				return
			}
		}
		if err = sess.Commit(); err != nil {
			return
		}
	}
	return
}

func removeCredentials(payload string) (string, error) {
	var opts base.MigrateOptions
	err := json.Unmarshal([]byte(payload), &opts)
	if err != nil {
		return "", err
	}

	opts.AuthPassword = ""
	opts.AuthToken = ""
	opts.CloneAddr = util.SanitizeURLCredentials(opts.CloneAddr, true)

	confBytes, err := json.Marshal(opts)
	if err != nil {
		return "", err
	}
	return string(confBytes), nil
}
