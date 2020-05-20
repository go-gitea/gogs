// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mail

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/services/mailer"
)

type mailNotifier struct {
	base.NullNotifier
}

var (
	_ base.Notifier = &mailNotifier{}
)

// NewNotifier create a new mailNotifier notifier
func NewNotifier() base.Notifier {
	return &mailNotifier{}
}

func (m *mailNotifier) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment) {
	var act models.ActionType
	if comment.Type == models.CommentTypeClose {
		act = models.ActionCloseIssue
	} else if comment.Type == models.CommentTypeReopen {
		act = models.ActionReopenIssue
	} else if comment.Type == models.CommentTypeComment {
		act = models.ActionCommentIssue
	} else if comment.Type == models.CommentTypeCode {
		act = models.ActionCommentIssue
	}

	if err := mailer.MailParticipantsComment(comment, act, issue); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyNewIssue(issue *models.Issue) {
	if err := issue.LoadPoster(); err != nil {
		log.Error("Unable to load poster: %d for issue: %d: Error: %v", issue.PosterID, issue.ID, err)
		return
	}
	if err := mailer.MailParticipants(issue, issue.Poster, models.ActionCreateIssue); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, actionComment *models.Comment, isClosed bool) {
	var actionType models.ActionType
	issue.Content = ""
	if issue.IsPull {
		if isClosed {
			actionType = models.ActionClosePullRequest
		} else {
			actionType = models.ActionReopenPullRequest
		}
	} else {
		if isClosed {
			actionType = models.ActionCloseIssue
		} else {
			actionType = models.ActionReopenIssue
		}
	}

	if err := mailer.MailParticipants(issue, doer, actionType); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyNewPullRequest(pr *models.PullRequest) {
	if err := pr.LoadIssue(); err != nil {
		log.Error("Unable to load issue: %d for pr: %d: Error: %v", pr.IssueID, pr.ID, err)
		return
	}
	if err := pr.Issue.LoadPoster(); err != nil {
		log.Error("Unable to load poster: %d for pr: %d, issue: %d: Error: %v", pr.Issue.PosterID, pr.ID, pr.IssueID, err)
		return
	}
	if err := mailer.MailParticipants(pr.Issue, pr.Issue.Poster, models.ActionCreatePullRequest); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}

func (m *mailNotifier) NotifyPullRequestReview(pr *models.PullRequest, r *models.Review, comment *models.Comment) {
	var act models.ActionType
	if comment.Type == models.CommentTypeClose {
		act = models.ActionCloseIssue
	} else if comment.Type == models.CommentTypeReopen {
		act = models.ActionReopenIssue
	} else if comment.Type == models.CommentTypeComment {
		act = models.ActionCommentPull
	}
	if err := mailer.MailParticipantsComment(comment, act, pr.Issue); err != nil {
		log.Error("MailParticipantsComment: %v", err)
	}
}

func (m *mailNotifier) NotifyIssueChangeAssignee(doer *models.User, issue *models.Issue, assignee *models.User, removed bool, comment *models.Comment) {
	// mail only sent to added assignees and not self-assignee
	if !removed && doer.ID != assignee.ID && assignee.EmailNotifications() == models.EmailNotificationsEnabled {
		ct := fmt.Sprintf("Assigned #%d.", issue.Index)
		mailer.SendIssueAssignedMail(issue, doer, ct, comment, []string{assignee.Email})
	}
}

func (m *mailNotifier) NotifyPullReviewRequest(doer *models.User, issue *models.Issue, reviewer *models.User, isRequest bool, comment *models.Comment) {
	if isRequest && doer.ID != reviewer.ID && reviewer.EmailNotifications() == models.EmailNotificationsEnabled {
		ct := fmt.Sprintf("Requested to review #%d.", issue.Index)
		mailer.SendIssueAssignedMail(issue, doer, ct, comment, []string{reviewer.Email})
	}
}

func (m *mailNotifier) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User) {
	if err := pr.LoadIssue(); err != nil {
		log.Error("pr.LoadIssue: %v", err)
		return
	}
	pr.Issue.Content = ""
	if err := mailer.MailParticipants(pr.Issue, doer, models.ActionMergePullRequest); err != nil {
		log.Error("MailParticipants: %v", err)
	}
}
