// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/repository"
)

// Notifier defines an interface to notify receiver
type Notifier interface {
	Run()

	NotifyCreateRepository(doer *models.User, u *models.User, repo *models.Repository)
	NotifyMigrateRepository(doer *models.User, u *models.User, repo *models.Repository)
	NotifyDeleteRepository(doer *models.User, repo *models.Repository)
	NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository)
	NotifyRenameRepository(doer *models.User, repo *models.Repository, oldRepoName string)
	NotifyTransferRepository(doer *models.User, repo *models.Repository, oldOwnerName string)

	NotifyNewIssue(issue *models.Issue, mentions []*models.User)
	NotifyIssueChangeStatus(*models.User, *models.Issue, *models.Comment, bool)
	NotifyIssueChangeMilestone(doer *models.User, issue *models.Issue, oldMilestoneID int64)
	NotifyIssueChangeAssignee(doer *models.User, issue *models.Issue, assignee *models.User, removed bool, comment *models.Comment)
	NotifyPullReviewRequest(doer *models.User, issue *models.Issue, reviewer *models.User, isRequest bool, comment *models.Comment)
	NotifyIssueChangeContent(doer *models.User, issue *models.Issue, oldContent string)
	NotifyIssueClearLabels(doer *models.User, issue *models.Issue)
	NotifyIssueChangeTitle(doer *models.User, issue *models.Issue, oldTitle string)
	NotifyIssueChangeRef(doer *models.User, issue *models.Issue, oldRef string)
	NotifyIssueChangeLabels(doer *models.User, issue *models.Issue,
		addedLabels []*models.Label, removedLabels []*models.Label)

	NotifyNewPullRequest(pr *models.PullRequest, mentions []*models.User)
	NotifyMergePullRequest(*models.PullRequest, *models.User)
	NotifyPullRequestSynchronized(doer *models.User, pr *models.PullRequest)
	NotifyPullRequestReview(pr *models.PullRequest, review *models.Review, comment *models.Comment, mentions []*models.User)
	NotifyPullRequestCodeComment(pr *models.PullRequest, comment *models.Comment, mentions []*models.User)
	NotifyPullRequestChangeTargetBranch(doer *models.User, pr *models.PullRequest, oldBranch string)
	NotifyPullRequestPushCommits(doer *models.User, pr *models.PullRequest, comment *models.Comment)
	NotifyPullRevieweDismiss(doer *models.User, review *models.Review, comment *models.Comment)

	NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
		issue *models.Issue, comment *models.Comment, mentions []*models.User)
	NotifyUpdateComment(*models.User, *models.Comment, string)
	NotifyDeleteComment(*models.User, *models.Comment)

	NotifyNewRelease(rel *models.Release)
	NotifyUpdateRelease(doer *models.User, rel *models.Release)
	NotifyDeleteRelease(doer *models.User, rel *models.Release)

	NotifyPushCommits(pusher *models.User, repo *models.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits)
	NotifyCreateRef(doer *models.User, repo *models.Repository, refType, refFullName string)
	NotifyDeleteRef(doer *models.User, repo *models.Repository, refType, refFullName string)

	NotifySyncPushCommits(pusher *models.User, repo *models.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits)
	NotifySyncCreateRef(doer *models.User, repo *models.Repository, refType, refFullName string)
	NotifySyncDeleteRef(doer *models.User, repo *models.Repository, refType, refFullName string)

	NotifyRepoPendingTransfer(doer, newOwner *models.User, repo *models.Repository)

	// org
	NotifyAddOrgMember(doer, org, member *models.User)
	NotifyRemoveOrgMember(doer, org, member *models.User)
	NotifyAddOrgTeam(doer, org *models.User, team *models.Team)
	NotifyRemoveOrgTeam(doer, org *models.User, team *models.Team)
	NotifyAddTeamMember(doer, org, member *models.User, team *models.Team)
	NotifyRemoveTeamMember(doer, org, member *models.User, team *models.Team)
}
