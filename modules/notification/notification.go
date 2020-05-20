// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package notification

import (
	"sync"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/notification/action"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/notification/indexer"
	"code.gitea.io/gitea/modules/notification/mail"
	"code.gitea.io/gitea/modules/notification/ui"
	"code.gitea.io/gitea/modules/notification/webhook"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
)

var (
	notifiers []base.Notifier
	once      = sync.Once{}
)

// RegisterNotifier providers method to receive notify messages
func RegisterNotifier(notifier base.Notifier) {
	go notifier.Run()
	notifiers = append(notifiers, notifier)
}

// NewContext registers notification handlers
func NewContext() {
	once.Do(func() {
		var ns []base.Notifier
		ns = append(ns, ui.NewNotifier())
		if setting.Service.EnableNotifyMail {
			ns = append(ns, mail.NewNotifier())
		}
		ns = append(ns, indexer.NewNotifier())
		ns = append(ns, webhook.NewNotifier())
		ns = append(ns, action.NewNotifier())

		RegisterNotifier(base.NewQueueNotifier("notification", ns))
	})
}

// NotifyCreateIssueComment notifies issue comment related message to notifiers
func NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyCreateIssueComment(doer, repo, issue, comment)
	}
}

// NotifyNewIssue notifies new issue to notifiers
func NotifyNewIssue(issue *models.Issue) {
	for _, notifier := range notifiers {
		notifier.NotifyNewIssue(issue)
	}
}

// NotifyIssueChangeStatus notifies close or reopen issue to notifiers
func NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, actionComment *models.Comment, closeOrReopen bool) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeStatus(doer, issue, actionComment, closeOrReopen)
	}
}

// NotifyMergePullRequest notifies merge pull request to notifiers
func NotifyMergePullRequest(pr *models.PullRequest, doer *models.User) {
	for _, notifier := range notifiers {
		notifier.NotifyMergePullRequest(pr, doer)
	}
}

// NotifyNewPullRequest notifies new pull request to notifiers
func NotifyNewPullRequest(pr *models.PullRequest) {
	for _, notifier := range notifiers {
		notifier.NotifyNewPullRequest(pr)
	}
}

// NotifyPullRequestSynchronized notifies Synchronized pull request
func NotifyPullRequestSynchronized(doer *models.User, pr *models.PullRequest) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestSynchronized(doer, pr)
	}
}

// NotifyPullRequestReview notifies new pull request review
func NotifyPullRequestReview(pr *models.PullRequest, review *models.Review, comment *models.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestReview(pr, review, comment)
	}
}

// NotifyPullRequestChangeTargetBranch notifies when a pull request's target branch was changed
func NotifyPullRequestChangeTargetBranch(doer *models.User, pr *models.PullRequest, oldBranch string) {
	for _, notifier := range notifiers {
		notifier.NotifyPullRequestChangeTargetBranch(doer, pr, oldBranch)
	}
}

// NotifyUpdateComment notifies update comment to notifiers
func NotifyUpdateComment(doer *models.User, c *models.Comment, oldContent string) {
	for _, notifier := range notifiers {
		notifier.NotifyUpdateComment(doer, c, oldContent)
	}
}

// NotifyDeleteComment notifies delete comment to notifiers
func NotifyDeleteComment(doer *models.User, c *models.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteComment(doer, c)
	}
}

// NotifyNewRelease notifies new release to notifiers
func NotifyNewRelease(rel *models.Release) {
	for _, notifier := range notifiers {
		notifier.NotifyNewRelease(rel)
	}
}

// NotifyUpdateRelease notifies update release to notifiers
func NotifyUpdateRelease(doer *models.User, rel *models.Release) {
	for _, notifier := range notifiers {
		notifier.NotifyUpdateRelease(doer, rel)
	}
}

// NotifyDeleteRelease notifies delete release to notifiers
func NotifyDeleteRelease(doer *models.User, rel *models.Release) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteRelease(doer, rel)
	}
}

// NotifyIssueChangeMilestone notifies change milestone to notifiers
func NotifyIssueChangeMilestone(doer *models.User, issue *models.Issue, oldMilestoneID int64) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeMilestone(doer, issue, oldMilestoneID)
	}
}

// NotifyIssueChangeContent notifies change content to notifiers
func NotifyIssueChangeContent(doer *models.User, issue *models.Issue, oldContent string) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeContent(doer, issue, oldContent)
	}
}

// NotifyIssueChangeAssignee notifies change content to notifiers
func NotifyIssueChangeAssignee(doer *models.User, issue *models.Issue, assignee *models.User, removed bool, comment *models.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeAssignee(doer, issue, assignee, removed, comment)
	}
}

// NotifyPullReviewRequest notifies Request Review change
func NotifyPullReviewRequest(doer *models.User, issue *models.Issue, reviewer *models.User, isRequest bool, comment *models.Comment) {
	for _, notifier := range notifiers {
		notifier.NotifyPullReviewRequest(doer, issue, reviewer, isRequest, comment)
	}
}

// NotifyIssueClearLabels notifies clear labels to notifiers
func NotifyIssueClearLabels(doer *models.User, issue *models.Issue) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueClearLabels(doer, issue)
	}
}

// NotifyIssueChangeTitle notifies change title to notifiers
func NotifyIssueChangeTitle(doer *models.User, issue *models.Issue, oldTitle string) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeTitle(doer, issue, oldTitle)
	}
}

// NotifyIssueChangeLabels notifies change labels to notifiers
func NotifyIssueChangeLabels(doer *models.User, issue *models.Issue,
	addedLabels []*models.Label, removedLabels []*models.Label) {
	for _, notifier := range notifiers {
		notifier.NotifyIssueChangeLabels(doer, issue, addedLabels, removedLabels)
	}
}

// NotifyCreateRepository notifies create repository to notifiers
func NotifyCreateRepository(doer *models.User, u *models.User, repo *models.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyCreateRepository(doer, u, repo)
	}
}

// NotifyMigrateRepository notifies create repository to notifiers
func NotifyMigrateRepository(doer *models.User, u *models.User, repo *models.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyMigrateRepository(doer, u, repo)
	}
}

// NotifyTransferRepository notifies create repository to notifiers
func NotifyTransferRepository(doer *models.User, repo *models.Repository, newOwnerName string) {
	for _, notifier := range notifiers {
		notifier.NotifyTransferRepository(doer, repo, newOwnerName)
	}
}

// NotifyDeleteRepository notifies delete repository to notifiers
func NotifyDeleteRepository(doer *models.User, repo *models.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteRepository(doer, repo)
	}
}

// NotifyForkRepository notifies fork repository to notifiers
func NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository) {
	for _, notifier := range notifiers {
		notifier.NotifyForkRepository(doer, oldRepo, repo)
	}
}

// NotifyRenameRepository notifies repository renamed
func NotifyRenameRepository(doer *models.User, repo *models.Repository, oldName string) {
	for _, notifier := range notifiers {
		notifier.NotifyRenameRepository(doer, repo, oldName)
	}
}

// NotifyPushCommits notifies commits pushed to notifiers
func NotifyPushCommits(pusher *models.User, repo *models.Repository, refName, oldCommitID, newCommitID string, commits *repository.PushCommits) {
	for _, notifier := range notifiers {
		notifier.NotifyPushCommits(pusher, repo, refName, oldCommitID, newCommitID, commits)
	}
}

// NotifyCreateRef notifies branch or tag creation to notifiers
func NotifyCreateRef(pusher *models.User, repo *models.Repository, refType, refFullName string) {
	for _, notifier := range notifiers {
		notifier.NotifyCreateRef(pusher, repo, refType, refFullName)
	}
}

// NotifyDeleteRef notifies branch or tag deletion to notifiers
func NotifyDeleteRef(pusher *models.User, repo *models.Repository, refType, refFullName string) {
	for _, notifier := range notifiers {
		notifier.NotifyDeleteRef(pusher, repo, refType, refFullName)
	}
}

// NotifySyncPushCommits notifies commits pushed to notifiers
func NotifySyncPushCommits(pusher *models.User, repo *models.Repository, refName, oldCommitID, newCommitID string, commits *repository.PushCommits) {
	for _, notifier := range notifiers {
		notifier.NotifySyncPushCommits(pusher, repo, refName, oldCommitID, newCommitID, commits)
	}
}

// NotifySyncCreateRef notifies branch or tag creation to notifiers
func NotifySyncCreateRef(pusher *models.User, repo *models.Repository, refType, refFullName string) {
	for _, notifier := range notifiers {
		notifier.NotifySyncCreateRef(pusher, repo, refType, refFullName)
	}
}

// NotifySyncDeleteRef notifies branch or tag deletion to notifiers
func NotifySyncDeleteRef(pusher *models.User, repo *models.Repository, refType, refFullName string) {
	for _, notifier := range notifiers {
		notifier.NotifySyncDeleteRef(pusher, repo, refType, refFullName)
	}
}
