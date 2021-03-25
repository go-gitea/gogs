// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"fmt"
	"os/exec"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"xorm.io/builder"
)

func iterateRepositories(each func(*models.Repository) error) error {
	err := models.Iterate(
		models.DefaultDBContext(),
		new(models.Repository),
		builder.Gt{"id": 0},
		func(idx int, bean interface{}) error {
			return each(bean.(*models.Repository))
		},
	)
	return err
}

func checkScriptType(logger log.Logger, autofix bool) error {
	path, err := exec.LookPath(setting.ScriptType)
	if err != nil {
		logger.Critical("ScriptType \"%q\" is not on the current PATH. Error: %v", setting.ScriptType, err)
		return fmt.Errorf("ScriptType \"%q\" is not on the current PATH. Error: %v", setting.ScriptType, err)
	}
	logger.Info("ScriptType %s is on the current PATH at %s", setting.ScriptType, path)
	return nil
}

func checkHooks(logger log.Logger, autofix bool) error {
	if err := iterateRepositories(func(repo *models.Repository) error {
		results, err := repository.CheckDelegateHooks(repo.RepoPath())
		if err != nil {
			logger.Critical("Unable to check delegate hooks for repo %-v. ERROR: %v", repo, err)
			return fmt.Errorf("Unable to check delegate hooks for repo %-v. ERROR: %v", repo, err)
		}
		if len(results) > 0 && autofix {
			logger.Warn("Regenerated hooks for %s", repo.FullName())
			if err := repository.CreateDelegateHooks(repo.RepoPath()); err != nil {
				logger.Critical("Unable to recreate delegate hooks for %-v. ERROR: %v", repo, err)
				return fmt.Errorf("Unable to recreate delegate hooks for %-v. ERROR: %v", repo, err)
			}
		}
		for _, result := range results {
			logger.Warn(result)
		}
		return nil
	}); err != nil {
		logger.Critical("Errors noted whilst checking delegate hooks.")
		return err
	}
	return nil
}

func checkUserStarNum(logger log.Logger, autofix bool) error {
	if err := models.DoctorUserStarNum(); err != nil {
		logger.Critical("Unable update User Stars numbers")
		return err
	}
	return nil
}

func checkEnablePushOptions(logger log.Logger, autofix bool) error {
	numRepos := 0
	numNeedUpdate := 0
	if err := iterateRepositories(func(repo *models.Repository) error {
		numRepos++
		r, err := git.OpenRepository(repo.RepoPath())
		if err != nil {
			return err
		}
		defer r.Close()

		if autofix {
			_, err := git.NewCommand("config", "receive.advertisePushOptions", "true").RunInDir(r.Path)
			return err
		}

		value, err := git.NewCommand("config", "receive.advertisePushOptions").RunInDir(r.Path)
		if err != nil {
			return err
		}

		result, valid := git.ParseBool(strings.TrimSpace(value))
		if !result || !valid {
			numNeedUpdate++
			logger.Info("%s: does not have receive.advertisePushOptions set correctly: %q", repo.FullName(), value)
		}
		return nil
	}); err != nil {
		logger.Critical("Unable to EnablePushOptions: %v", err)
		return err
	}

	if autofix {
		logger.Info("Enabled push options for %d repositories.", numRepos)
	} else {
		logger.Info("Checked %d repositories, %d need updates.", numRepos, numNeedUpdate)
	}

	return nil
}

func init() {
	Register(&Check{
		Title:     "Check if SCRIPT_TYPE is available",
		Name:      "script-type",
		IsDefault: false,
		Run:       checkScriptType,
		Priority:  5,
	})
	Register(&Check{
		Title:     "Check if hook files are up-to-date and executable",
		Name:      "hooks",
		IsDefault: false,
		Run:       checkHooks,
		Priority:  6,
	})
	Register(&Check{
		Title:     "Recalculate Stars number for all user",
		Name:      "recalculate-stars-number",
		IsDefault: false,
		Run:       checkUserStarNum,
		Priority:  6,
	})
	Register(&Check{
		Title:     "Enable push options",
		Name:      "enable-push-options",
		IsDefault: false,
		Run:       checkEnablePushOptions,
		Priority:  7,
	})
}
