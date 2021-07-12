// Copyright 2021 Gitea. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package automerge

import (
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	pullservice "code.gitea.io/gitea/services/pull"
)

// This package merges a previously scheduled pull request on successful status check.
// It is a separate package to avoid cyclic dependencies.

// MergeScheduledPullRequest merges a previously scheduled pull request when all checks succeeded
func MergeScheduledPullRequest(sha string, repo *models.Repository) (err error) {
	branches, err := git.GetBranchNamesForSha(sha, repo.RepoPath())
	if err != nil {
		return
	}

	for _, branch := range branches {
		// If the branch starts with "pull/*" we know we're dealing with a fork.
		// In that case, head and base branch are not in the same repo and we need to do some extra work
		// to get the pull request for this branch.
		// Each pull branch starts with refs/pull/ we then go from there to find the index of the pr and then
		// use that to get the pr.
		var pulls = make(map[int64]*models.PullRequest)
		err = nil // Could be filled with an error from an earlier run
		if strings.HasPrefix(branch, "refs/pull/") {

			parts := strings.Split(branch, "/")

			if len(parts) < 3 {
				continue
			}

			prIndex, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return err
			}

			p, err := models.GetPullRequestByIndex(repo.ID, prIndex)
			if err != nil {
				// If there is no pull request for this branch, we don't try to merge it.
				if models.IsErrPullRequestNotExist(err) {
					continue
				}
				return err
			}
			pulls[p.ID] = p
		} else {
			prs, err := models.GetPullRequestsByHeadBranch(branch, repo, models.PullRequestStatusMergeable)
			if err != nil {
				// If there is no pull request for this branch, we don't try to merge it.
				if models.IsErrPullRequestNotExist(err) {
					continue
				}
				return err
			}
			for i := range prs {
				pulls[prs[i].ID] = prs[i]
			}
		}

		for _, pr := range pulls {
			go handlePull(pr, sha)
		}
	}
	return nil
}

func handlePull(pr *models.PullRequest, sha string) {
	if pr.HasMerged {
		return
	}

	// Check if there is a scheduled pr in the db
	exists, scheduledPRM, err := models.GetScheduledPullRequestMergeByPullID(pr.ID)
	if err != nil {
		log.Error(err.Error())
		return
	}
	if !exists {
		return
	}

	// Get all checks for this pr
	// We get the latest sha commit hash again to handle the case where the check of a previous push
	// did not succeed or was not finished yet.

	if err = pr.LoadHeadRepo(); err != nil {
		log.Error(err.Error())
		return
	}

	headGitRepo, err := git.OpenRepository(pr.HeadRepo.RepoPath())
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer headGitRepo.Close()

	headBranchExist := headGitRepo.IsBranchExist(pr.HeadBranch)

	if pr.HeadRepo == nil || !headBranchExist {
		log.Info("Head branch of auto merge pr does not exist [HeadRepoID: %d, Branch: %s, PRID: %d]", pr.HeadRepoID, pr.HeadBranch, pr.ID)
		return
	}

	// Check if all checks succeeded
	pass, err := pullservice.IsPullCommitStatusPass(pr)
	if err != nil {
		log.Error(err.Error())
		return
	}
	if !pass {
		log.Info("Scheduled auto merge pr has unsuccessful status checks [PRID: %d, Commit: %s]", pr.ID, sha)
		return
	}

	// Merge if all checks succeeded
	doer, err := models.GetUserByID(scheduledPRM.DoerID)
	if err != nil {
		log.Error(err.Error())
		return
	}

	if err = pr.LoadBaseRepo(); err != nil {
		log.Error(err.Error())
		return
	}

	baseGitRepo, err := git.OpenRepository(pr.BaseRepo.RepoPath())
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer baseGitRepo.Close()

	if err := pullservice.Merge(pr, doer, baseGitRepo, scheduledPRM.MergeStyle, scheduledPRM.Message); err != nil {
		log.Error(err.Error())
		return
	}

	// Remove the schedule from the db
	if err := models.RemoveScheduledPullRequestMerge(nil, scheduledPRM.PullID, false); err != nil {
		log.Error(err.Error())
		return
	}
}
