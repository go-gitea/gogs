// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/services/automerge"
)

// CreateCommitStatus creates a new CommitStatus given a bunch of parameters
// NOTE: All text-values will be trimmed from whitespaces.
// Requires: Repo, Creator, SHA
func CreateCommitStatus(repo *models.Repository, creator *models.User, sha string, status *models.CommitStatus) error {
	repoPath := repo.RepoPath()

	// confirm that commit is exist
	gitRepo, err := git.OpenRepository(repoPath)
	if err != nil {
		return fmt.Errorf("OpenRepository[%s]: %v", repoPath, err)
	}
	if _, err := gitRepo.GetCommit(sha); err != nil {
		gitRepo.Close()
		return fmt.Errorf("GetCommit[%s]: %v", sha, err)
	}
	gitRepo.Close()

	if err := models.NewCommitStatus(models.NewCommitStatusOptions{
		Repo:         repo,
		Creator:      creator,
		SHA:          sha,
		CommitStatus: status,
	}); err != nil {
		return fmt.Errorf("NewCommitStatus[repo_id: %d, user_id: %d, sha: %s]: %v", repo.ID, creator.ID, sha, err)
	}

	if status.State.IsSuccess() {
		err = automerge.MergeScheduledPullRequest(sha, repo)
		if err != nil {
			return fmt.Errorf("MergeScheduledPullRequest[repo_id: %d, user_id: %d, sha: %s]: %v", repo.ID, creator.ID, sha, err)
		}
	}

	return nil
}
