// Copyright 2019 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package task

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations"
	migration "code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"
	jsoniter "github.com/json-iterator/go"
)

func handleCreateError(owner *models.User, err error) error {
	switch {
	case models.IsErrReachLimitOfRepo(err):
		return fmt.Errorf("You have already reached your limit of %d repositories", owner.MaxCreationLimit())
	case models.IsErrRepoAlreadyExist(err):
		return errors.New("The repository name is already used")
	case models.IsErrNameReserved(err):
		return fmt.Errorf("The repository name '%s' is reserved", err.(models.ErrNameReserved).Name)
	case models.IsErrNamePatternNotAllowed(err):
		return fmt.Errorf("The pattern '%s' is not allowed in a repository name", err.(models.ErrNamePatternNotAllowed).Pattern)
	default:
		return err
	}
}

func runMigrateTask(t *models.Task) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("PANIC whilst trying to do migrate task: %v", e)
			log.Critical("PANIC during runMigrateTask[%d] by DoerID[%d] to RepoID[%d] for OwnerID[%d]: %v\nStacktrace: %v", t.ID, t.DoerID, t.RepoID, t.OwnerID, e, log.Stack(2))
		}

		if err == nil {
			err = models.FinishMigrateTask(t)
			if err == nil {
				notification.NotifyMigrateRepository(t.Doer, t.Owner, t.Repo)
				return
			}

			log.Error("FinishMigrateTask[%d] by DoerID[%d] to RepoID[%d] for OwnerID[%d] failed: %v", t.ID, t.DoerID, t.RepoID, t.OwnerID, err)
		}

		t.EndTime = timeutil.TimeStampNow()
		t.Status = structs.TaskStatusFailed
		t.Message = err.Error()
		t.RepoID = 0
		if err := t.UpdateCols("status", "errors", "repo_id", "end_time"); err != nil {
			log.Error("Task UpdateCols failed: %v", err)
		}

		if t.Repo != nil {
			if errDelete := models.DeleteRepository(t.Doer, t.OwnerID, t.Repo.ID); errDelete != nil {
				log.Error("DeleteRepository: %v", errDelete)
			}
		}
	}()

	if err = t.LoadRepo(); err != nil {
		return
	}

	// if repository is ready, then just finsih the task
	if t.Repo.Status == models.RepositoryReady {
		return nil
	}

	if err = t.LoadDoer(); err != nil {
		return
	}
	if err = t.LoadOwner(); err != nil {
		return
	}

	var opts *migration.MigrateOptions
	opts, err = t.MigrateConfig()
	if err != nil {
		return
	}

	opts.MigrateToRepoID = t.RepoID
	var repo *models.Repository

	ctx, cancel := context.WithCancel(graceful.GetManager().ShutdownContext())
	defer cancel()
	pm := process.GetManager()
	pid := pm.Add(fmt.Sprintf("MigrateTask: %s/%s", t.Owner.Name, opts.RepoName), cancel)
	defer pm.Remove(pid)

	t.StartTime = timeutil.TimeStampNow()
	t.Status = structs.TaskStatusRunning
	if err = t.UpdateCols("start_time", "status"); err != nil {
		return
	}

	repo, err = migrations.MigrateRepository(ctx, t.Doer, t.Owner.Name, *opts, func(format string, args ...interface{}) {
		message := models.TranslatableMessage{
			Format: format,
			Args:   args,
		}
		json := jsoniter.ConfigCompatibleWithStandardLibrary
		bs, _ := json.Marshal(message)
		t.Message = string(bs)
		_ = t.UpdateCols("message")
	})
	if err == nil {
		log.Trace("Repository migrated [%d]: %s/%s", repo.ID, t.Owner.Name, repo.Name)
		return
	}

	if models.IsErrRepoAlreadyExist(err) {
		err = errors.New("The repository name is already used")
		return
	}

	// remoteAddr may contain credentials, so we sanitize it
	err = util.NewStringURLSanitizedError(err, opts.CloneAddr, true)
	if strings.Contains(err.Error(), "Authentication failed") ||
		strings.Contains(err.Error(), "could not read Username") {
		return fmt.Errorf("Authentication failed: %v", err.Error())
	} else if strings.Contains(err.Error(), "fatal:") {
		return fmt.Errorf("Migration failed: %v", err.Error())
	}

	// do not be tempted to coalesce this line with the return
	err = handleCreateError(t.Owner, err)
	return
}
