// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pull

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// Update updates pull request with base branch.
func Update(pull *models.PullRequest, doer *models.User, message string) error {
	//use merge functions but switch repo's and branch's
	pr := &models.PullRequest{
		HeadRepoID: pull.BaseRepoID,
		BaseRepoID: pull.HeadRepoID,
		HeadBranch: pull.BaseBranch,
		BaseBranch: pull.HeadBranch,
	}

	if err := pr.LoadHeadRepo(); err != nil {
		log.Error("LoadHeadRepo: %v", err)
		return fmt.Errorf("LoadHeadRepo: %v", err)
	} else if err = pr.LoadBaseRepo(); err != nil {
		log.Error("LoadBaseRepo: %v", err)
		return fmt.Errorf("LoadBaseRepo: %v", err)
	}

	diffCount, err := GetDiverging(pull)
	if err != nil {
		return err
	} else if diffCount.Behind == 0 {
		return fmt.Errorf("HeadBranch of PR %d is up to date", pull.Index)
	}

	_, err = rawMerge(pr, doer, models.MergeStyleMerge, message)

	defer func() {
		go AddTestPullRequestTask(doer, pr.HeadRepo.ID, pr.HeadBranch, false, "", "")
	}()

	return err
}

// IsUserAllowedToUpdate check if user is allowed to update PR with given permissions and branch protections
func IsUserAllowedToUpdate(pull *models.PullRequest, user *models.User) (bool, error) {
	if user == nil {
		return false, nil
	}
	headRepoPerm, err := models.GetUserRepoPermission(pull.HeadRepo, user)
	if err != nil {
		return false, err
	}

	pr := &models.PullRequest{
		HeadRepoID: pull.BaseRepoID,
		BaseRepoID: pull.HeadRepoID,
		HeadBranch: pull.BaseBranch,
		BaseBranch: pull.HeadBranch,
	}

	err = pr.LoadProtectedBranch()
	if err != nil {
		return false, err
	}

	// Update function need push permission
	if pr.ProtectedBranch != nil && !pr.ProtectedBranch.CanUserPush(user.ID) {
		return false, nil
	}

	return IsUserAllowedToMerge(pr, headRepoPerm, user)
}

// GetDiverging determines how many commits a PR is ahead or behind the PR base branch
func GetDiverging(pr *models.PullRequest) (*git.DivergeObject, error) {
	log.Trace("GetDiverging[%d]: compare commits", pr.ID)
	if err := pr.LoadBaseRepo(); err != nil {
		return nil, err
	}
	if err := pr.LoadHeadRepo(); err != nil {
		return nil, err
	}

	tmpRepo, err := createTemporaryRepo(pr)
	if err != nil {
		if !models.IsErrBranchDoesNotExist(err) {
			log.Error("CreateTemporaryRepo: %v", err)
		}
		return nil, err
	}
	defer func() {
		if err := models.RemoveTemporaryPath(tmpRepo); err != nil {
			log.Error("Merge: RemoveTemporaryPath: %s", err)
		}
	}()

	diff, err := git.GetDivergingCommits(tmpRepo, "base", "tracking")
	return &diff, err
}
