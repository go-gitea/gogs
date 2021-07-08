// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/repository"
)

// NullNotifier implements a blank notifier
type NullNotifier struct {
}

var (
	_ Notifier = &NullNotifier{}
)

// Run places a place holder function
func (*NullNotifier) Run() {
}

// NotifyCreateIssueComment places a place holder function
func (*NullNotifier) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment, mentions []*models.User) {
}

// NotifyNewIssue places a place holder function
func (*NullNotifier) NotifyNewIssue(issue *models.Issue, mentions []*models.User) {
}

// NotifyIssueChangeStatus places a place holder function
func (*NullNotifier) NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, actionComment *models.Comment, isClosed bool) {
}

// NotifyNewPullRequest places a place holder function
func (*NullNotifier) NotifyNewPullRequest(pr *models.PullRequest, mentions []*models.User) {
}

// NotifyPullRequestReview places a place holder function
func (*NullNotifier) NotifyPullRequestReview(pr *models.PullRequest, r *models.Review, comment *models.Comment, mentions []*models.User) {
}

// NotifyPullRequestCodeComment places a place holder function
func (*NullNotifier) NotifyPullRequestCodeComment(pr *models.PullRequest, comment *models.Comment, mentions []*models.User) {
}

// NotifyMergePullRequest places a place holder function
func (*NullNotifier) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User) {
}

// NotifyPullRequestSynchronized places a place holder function
func (*NullNotifier) NotifyPullRequestSynchronized(doer *models.User, pr *models.PullRequest) {
}

// NotifyPullRequestChangeTargetBranch places a place holder function
func (*NullNotifier) NotifyPullRequestChangeTargetBranch(doer *models.User, pr *models.PullRequest, oldBranch string) {
}

// NotifyPullRequestPushCommits notifies when push commits to pull request's head branch
func (*NullNotifier) NotifyPullRequestPushCommits(doer *models.User, pr *models.PullRequest, comment *models.Comment) {
}

// NotifyPullRevieweDismiss notifies when a review was dismissed by repo admin
func (*NullNotifier) NotifyPullRevieweDismiss(doer *models.User, review *models.Review, comment *models.Comment) {
}

// NotifyUpdateComment places a place holder function
func (*NullNotifier) NotifyUpdateComment(doer *models.User, c *models.Comment, oldContent string) {
}

// NotifyDeleteComment places a place holder function
func (*NullNotifier) NotifyDeleteComment(doer *models.User, c *models.Comment) {
}

// NotifyNewRelease places a place holder function
func (*NullNotifier) NotifyNewRelease(rel *models.Release) {
}

// NotifyUpdateRelease places a place holder function
func (*NullNotifier) NotifyUpdateRelease(doer *models.User, rel *models.Release) {
}

// NotifyDeleteRelease places a place holder function
func (*NullNotifier) NotifyDeleteRelease(doer *models.User, rel *models.Release) {
}

// NotifyIssueChangeMilestone places a place holder function
func (*NullNotifier) NotifyIssueChangeMilestone(doer *models.User, issue *models.Issue, oldMilestoneID int64) {
}

// NotifyIssueChangeContent places a place holder function
func (*NullNotifier) NotifyIssueChangeContent(doer *models.User, issue *models.Issue, oldContent string) {
}

// NotifyIssueChangeAssignee places a place holder function
func (*NullNotifier) NotifyIssueChangeAssignee(doer *models.User, issue *models.Issue, assignee *models.User, removed bool, comment *models.Comment) {
}

// NotifyPullReviewRequest places a place holder function
func (*NullNotifier) NotifyPullReviewRequest(doer *models.User, issue *models.Issue, reviewer *models.User, isRequest bool, comment *models.Comment) {
}

// NotifyIssueClearLabels places a place holder function
func (*NullNotifier) NotifyIssueClearLabels(doer *models.User, issue *models.Issue) {
}

// NotifyIssueChangeTitle places a place holder function
func (*NullNotifier) NotifyIssueChangeTitle(doer *models.User, issue *models.Issue, oldTitle string) {
}

// NotifyIssueChangeRef places a place holder function
func (*NullNotifier) NotifyIssueChangeRef(doer *models.User, issue *models.Issue, oldTitle string) {
}

// NotifyIssueChangeLabels places a place holder function
func (*NullNotifier) NotifyIssueChangeLabels(doer *models.User, issue *models.Issue,
	addedLabels []*models.Label, removedLabels []*models.Label) {
}

// NotifyCreateRepository places a place holder function
func (*NullNotifier) NotifyCreateRepository(doer *models.User, u *models.User, repo *models.Repository) {
}

// NotifyDeleteRepository places a place holder function
func (*NullNotifier) NotifyDeleteRepository(doer *models.User, repo *models.Repository) {
}

// NotifyForkRepository places a place holder function
func (*NullNotifier) NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository) {
}

// NotifyMigrateRepository places a place holder function
func (*NullNotifier) NotifyMigrateRepository(doer *models.User, u *models.User, repo *models.Repository) {
}

// NotifyPushCommits notifies commits pushed to notifiers
func (*NullNotifier) NotifyPushCommits(pusher *models.User, repo *models.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

// NotifyCreateRef notifies branch or tag creation to notifiers
func (*NullNotifier) NotifyCreateRef(doer *models.User, repo *models.Repository, refType, refFullName string) {
}

// NotifyDeleteRef notifies branch or tag deletion to notifiers
func (*NullNotifier) NotifyDeleteRef(doer *models.User, repo *models.Repository, refType, refFullName string) {
}

// NotifyRenameRepository places a place holder function
func (*NullNotifier) NotifyRenameRepository(doer *models.User, repo *models.Repository, oldRepoName string) {
}

// NotifyTransferRepository places a place holder function
func (*NullNotifier) NotifyTransferRepository(doer *models.User, repo *models.Repository, oldOwnerName string) {
}

// NotifySyncPushCommits places a place holder function
func (*NullNotifier) NotifySyncPushCommits(pusher *models.User, repo *models.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
}

// NotifySyncCreateRef places a place holder function
func (*NullNotifier) NotifySyncCreateRef(doer *models.User, repo *models.Repository, refType, refFullName string) {
}

// NotifySyncDeleteRef places a place holder function
func (*NullNotifier) NotifySyncDeleteRef(doer *models.User, repo *models.Repository, refType, refFullName string) {
}

// NotifyRepoPendingTransfer places a place holder function
func (*NullNotifier) NotifyRepoPendingTransfer(doer, newOwner *models.User, repo *models.Repository) {
}
