// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	repo_module "code.gitea.io/gitea/modules/repository"
	api "code.gitea.io/gitea/modules/structs"
)

// ToAPIPullRequest assumes following fields have been assigned with valid values:
// Required - Issue
// Optional - Merger
func ToAPIPullRequest(pr *models.PullRequest) *api.PullRequest {
	var (
		baseBranch *git.Branch
		headBranch *git.Branch
		baseCommit *git.Commit
		err        error
	)

	if pr.Issue.PullRequest == nil {
		pr.Issue.PullRequest = pr
	}

	if err = pr.Issue.LoadRepo(); err != nil {
		log.Error("pr.Issue.LoadRepo[%d]: %v", pr.ID, err)
		return nil
	}

	apiIssue := ToAPIIssue(pr.Issue)
	if err := pr.LoadBaseRepo(); err != nil {
		log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
		return nil
	}

	if err := pr.LoadHeadRepo(); err != nil {
		log.Error("GetRepositoryById[%d]: %v", pr.ID, err)
		return nil
	}

	apiPullRequest := &api.PullRequest{
		ID:        pr.ID,
		URL:       pr.Issue.HTMLURL(),
		Index:     pr.Index,
		Poster:    apiIssue.Poster,
		Title:     apiIssue.Title,
		Body:      apiIssue.Body,
		Labels:    apiIssue.Labels,
		Milestone: apiIssue.Milestone,
		Assignee:  apiIssue.Assignee,
		Assignees: apiIssue.Assignees,
		State:     apiIssue.State,
		Comments:  apiIssue.Comments,
		HTMLURL:   pr.Issue.HTMLURL(),
		DiffURL:   pr.Issue.DiffURL(),
		PatchURL:  pr.Issue.PatchURL(),
		HasMerged: pr.HasMerged,
		MergeBase: pr.MergeBase,
		Deadline:  apiIssue.Deadline,
		Created:   pr.Issue.CreatedUnix.AsTimePtr(),
		Updated:   pr.Issue.UpdatedUnix.AsTimePtr(),

		Base: &api.PRBranchInfo{
			Name:       pr.BaseBranch,
			Ref:        pr.BaseBranch,
			RepoID:     pr.BaseRepoID,
			Repository: pr.BaseRepo.APIFormat(models.AccessModeNone),
		},
		Head: &api.PRBranchInfo{
			Name:   pr.HeadBranch,
			Ref:    fmt.Sprintf("refs/pull/%d/head", pr.Index),
			RepoID: -1,
		},
	}

	baseBranch, err = repo_module.GetBranch(pr.BaseRepo, pr.BaseBranch)
	if err != nil && !git.IsErrBranchNotExist(err) {
		log.Error("GetBranch[%s]: %v", pr.BaseBranch, err)
		return nil
	}

	if err == nil {
		baseCommit, err = baseBranch.GetCommit()
		if err != nil && !git.IsErrNotExist(err) {
			log.Error("GetCommit[%s]: %v", baseBranch.Name, err)
			return nil
		}

		if err == nil {
			apiPullRequest.Base.Sha = baseCommit.ID.String()
		}
	}

	if pr.HeadRepo != nil {
		apiPullRequest.Head.RepoID = pr.HeadRepo.ID
		apiPullRequest.Head.Repository = pr.HeadRepo.APIFormat(models.AccessModeNone)

		headGitRepo, err := git.OpenRepository(pr.HeadRepo.RepoPath())
		if err != nil {
			log.Error("OpenRepository[%s]: %v", pr.HeadRepo.RepoPath(), err)
			return nil
		}
		defer headGitRepo.Close()

		headBranch, err = headGitRepo.GetBranch(pr.HeadBranch)
		if err != nil && !git.IsErrBranchNotExist(err) {
			log.Error("GetBranch[%s]: %v", pr.HeadBranch, err)
			return nil
		}

		if git.IsErrBranchNotExist(err) {
			headCommitID, err := headGitRepo.GetRefCommitID(apiPullRequest.Head.Ref)
			if err != nil && !git.IsErrNotExist(err) {
				log.Error("GetCommit[%s]: %v", pr.HeadBranch, err)
				return nil
			}
			if err == nil {
				apiPullRequest.Head.Sha = headCommitID
			}
		} else {
			commit, err := headBranch.GetCommit()
			if err != nil && !git.IsErrNotExist(err) {
				log.Error("GetCommit[%s]: %v", headBranch.Name, err)
				return nil
			}
			if err == nil {
				apiPullRequest.Head.Ref = pr.HeadBranch
				apiPullRequest.Head.Sha = commit.ID.String()
			}
		}
	}

	if pr.Status != models.PullRequestStatusChecking {
		mergeable := !(pr.Status == models.PullRequestStatusConflict || pr.Status == models.PullRequestStatusError) && !pr.IsWorkInProgress()
		apiPullRequest.Mergeable = mergeable
	}
	if pr.HasMerged {
		apiPullRequest.Merged = pr.MergedUnix.AsTimePtr()
		apiPullRequest.MergedCommitID = &pr.MergedCommitID
		apiPullRequest.MergedBy = pr.Merger.APIFormat()
	}

	return apiPullRequest
}
