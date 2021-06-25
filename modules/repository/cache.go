// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repository

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
)

func getRefName(fullRefName string) string {
	if strings.HasPrefix(fullRefName, git.TagPrefix) {
		return fullRefName[len(git.TagPrefix):]
	} else if strings.HasPrefix(fullRefName, git.BranchPrefix) {
		return fullRefName[len(git.BranchPrefix):]
	}
	return ""
}

// CacheRef cachhe last commit information of the branch or the tag
func CacheRef(ctx context.Context, repo *models.Repository, gitRepo *git.Repository, fullRefName string) error {
	if !setting.CacheService.LastCommit.Enabled {
		return nil
	}

	commit, err := gitRepo.GetCommit(fullRefName)
	if err != nil {
		return err
	}

	commitsCount, err := cache.GetInt64(repo.GetCommitsCountCacheKey(getRefName(fullRefName), true), commit.CommitsCount)
	if err != nil {
		return err
	}
	if commitsCount < setting.CacheService.LastCommit.CommitsCount {
		return nil
	}

	repoFullName := repo.FullName()
	commitID := commit.ID.String()

	return UpdateCache(repoFullName, commitID, "", true)
}
