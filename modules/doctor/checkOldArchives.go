// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"os"
	"path/filepath"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
)

func checkOldArchives(logger log.Logger, autofix bool) error {
	numRepos := 0
	numReposUpdated := 0
	err := iterateRepositories(func(repo *models.Repository) error {
		if repo.IsEmpty {
			return nil
		}

		p := filepath.Join(repo.RepoPath(), "archives")
		isDir, err := util.IsDir(p)
		if err != nil {
			log.Warn("check if %s is directory failed: %v", p, err)
		}
		if isDir {
			numRepos++
			if autofix {
				if err := os.RemoveAll(p); err == nil {
					numReposUpdated++
				} else {
					log.Warn("remove %s failed: %v", p, err)
				}
			}
		}
		return nil
	})

	if autofix {
		logger.Info("%d / %d old archives in repository deleted", numReposUpdated, numRepos)
	} else {
		logger.Info("%d old archives in repository need to be deleted", numRepos)
	}

	return err
}

func init() {
	Register(&Check{
		Title:     "Check old archives",
		Name:      "check-old-archives",
		IsDefault: false,
		Run:       checkOldArchives,
		Priority:  7,
	})
}
