// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/repository"
)

//go:generate go run -mod=vendor main.go

// If you change the below definition you must run go generate - see main.go

// Notifier defines an interface to notify receiver
type Notifier interface {
	Run()

	NotifyCreateRepository(doer *models.User, u *models.User, repo *models.Repository)
	NotifyMigrateRepository(doer *models.User, u *models.User, repo *models.Repository)
	NotifyDeleteRepository(doer *models.User, repo *models.Repository)
	NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository)
	NotifyRenameRepository(doer *models.User, repo *models.Repository, oldRepoName string)
	NotifyTransferRepository(doer *models.User, repo *models.Repository, oldOwnerName string)

	NotifyNewIssue(*models.Issue)
	NotifyIssueChangeStatus(*models.User, *models.Issue, *models.Comment, bool)
	NotifyIssueChangeMilestone(doer *models.User, issue *models.Issue, oldMilestoneID int64)
	NotifyIssueChangeAssignee(doer *models.User, issue *models.Issue, assignee *models.User, removed bool, comment *models.Comment)
	NotifyPullReviewRequest(doer *models.User, issue *models.Issue, reviewer *models.User, isRequest bool, comment *models.Comment)
	NotifyIssueChangeContent(doer *models.User, issue *models.Issue, oldContent string)
	NotifyIssueClearLabels(doer *models.User, issue *models.Issue)
	NotifyIssueChangeTitle(doer *models.User, issue *models.Issue, oldTitle string)
	NotifyIssueChangeLabels(doer *models.User, issue *models.Issue,
		addedLabels []*models.Label, removedLabels []*models.Label)

	NotifyNewPullRequest(*models.PullRequest)
	NotifyMergePullRequest(*models.PullRequest, *models.User)
	NotifyPullRequestSynchronized(doer *models.User, pr *models.PullRequest)
	NotifyPullRequestReview(*models.PullRequest, *models.Review, *models.Comment)
	NotifyPullRequestChangeTargetBranch(doer *models.User, pr *models.PullRequest, oldBranch string)

	NotifyCreateIssueComment(*models.User, *models.Repository,
		*models.Issue, *models.Comment)
	NotifyUpdateComment(*models.User, *models.Comment, string)
	NotifyDeleteComment(*models.User, *models.Comment)

	NotifyNewRelease(rel *models.Release)
	NotifyUpdateRelease(doer *models.User, rel *models.Release)
	NotifyDeleteRelease(doer *models.User, rel *models.Release)

	NotifyPushCommits(pusher *models.User, repo *models.Repository, refName, oldCommitID, newCommitID string, commits *repository.PushCommits)
	NotifyCreateRef(doer *models.User, repo *models.Repository, refType, refFullName string)
	NotifyDeleteRef(doer *models.User, repo *models.Repository, refType, refFullName string)

	NotifySyncPushCommits(pusher *models.User, repo *models.Repository, refName, oldCommitID, newCommitID string, commits *repository.PushCommits)
	NotifySyncCreateRef(doer *models.User, repo *models.Repository, refType, refFullName string)
	NotifySyncDeleteRef(doer *models.User, repo *models.Repository, refType, refFullName string)
}
