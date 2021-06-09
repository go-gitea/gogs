// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package action

import (
	"fmt"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/repository"

	"github.com/goccy/go-json"
)

type actionNotifier struct {
	base.NullNotifier
}

var (
	_ base.Notifier = &actionNotifier{}
)

// NewNotifier create a new actionNotifier notifier
func NewNotifier() base.Notifier {
	return &actionNotifier{}
}

func (a *actionNotifier) NotifyNewIssue(issue *models.Issue, mentions []*models.User) {
	if err := issue.LoadPoster(); err != nil {
		log.Error("issue.LoadPoster: %v", err)
		return
	}
	if err := issue.LoadRepo(); err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	repo := issue.Repo

	if err := models.NotifyWatchers(&models.Action{
		ActUserID: issue.Poster.ID,
		ActUser:   issue.Poster,
		OpType:    models.ActionCreateIssue,
		Content:   fmt.Sprintf("%d|%s", issue.Index, issue.Title),
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

// NotifyIssueChangeStatus notifies close or reopen issue to notifiers
func (a *actionNotifier) NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, actionComment *models.Comment, closeOrReopen bool) {
	// Compose comment action, could be plain comment, close or reopen issue/pull request.
	// This object will be used to notify watchers in the end of function.
	act := &models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		Content:   fmt.Sprintf("%d|%s", issue.Index, ""),
		RepoID:    issue.Repo.ID,
		Repo:      issue.Repo,
		Comment:   actionComment,
		CommentID: actionComment.ID,
		IsPrivate: issue.Repo.IsPrivate,
	}
	// Check comment type.
	if closeOrReopen {
		act.OpType = models.ActionCloseIssue
		if issue.IsPull {
			act.OpType = models.ActionClosePullRequest
		}
	} else {
		act.OpType = models.ActionReopenIssue
		if issue.IsPull {
			act.OpType = models.ActionReopenPullRequest
		}
	}

	// Notify watchers for whatever action comes in, ignore if no action type.
	if err := models.NotifyWatchers(act); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

// NotifyCreateIssueComment notifies comment on an issue to notifiers
func (a *actionNotifier) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment, mentions []*models.User) {
	act := &models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		RepoID:    issue.Repo.ID,
		Repo:      issue.Repo,
		Comment:   comment,
		CommentID: comment.ID,
		IsPrivate: issue.Repo.IsPrivate,
	}

	content := ""

	if len(comment.Content) > 200 {
		content = comment.Content[:strings.LastIndex(comment.Content[0:200], " ")] + "…"
	} else {
		content = comment.Content
	}
	act.Content = fmt.Sprintf("%d|%s", issue.Index, content)

	if issue.IsPull {
		act.OpType = models.ActionCommentPull
	} else {
		act.OpType = models.ActionCommentIssue
	}

	// Notify watchers for whatever action comes in, ignore if no action type.
	if err := models.NotifyWatchers(act); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyNewPullRequest(pull *models.PullRequest, mentions []*models.User) {
	if err := pull.LoadIssue(); err != nil {
		log.Error("pull.LoadIssue: %v", err)
		return
	}
	if err := pull.Issue.LoadRepo(); err != nil {
		log.Error("pull.Issue.LoadRepo: %v", err)
		return
	}
	if err := pull.Issue.LoadPoster(); err != nil {
		log.Error("pull.Issue.LoadPoster: %v", err)
		return
	}

	if err := models.NotifyWatchers(&models.Action{
		ActUserID: pull.Issue.Poster.ID,
		ActUser:   pull.Issue.Poster,
		OpType:    models.ActionCreatePullRequest,
		Content:   fmt.Sprintf("%d|%s", pull.Issue.Index, pull.Issue.Title),
		RepoID:    pull.Issue.Repo.ID,
		Repo:      pull.Issue.Repo,
		IsPrivate: pull.Issue.Repo.IsPrivate,
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyRenameRepository(doer *models.User, repo *models.Repository, oldRepoName string) {
	log.Trace("action.ChangeRepositoryName: %s/%s", doer.Name, repo.Name)

	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionRenameRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		Content:   oldRepoName,
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyTransferRepository(doer *models.User, repo *models.Repository, oldOwnerName string) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionTransferRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		Content:   path.Join(oldOwnerName, repo.Name),
	}); err != nil {
		log.Error("NotifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyCreateRepository(doer *models.User, u *models.User, repo *models.Repository) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionCreateRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error("notify watchers '%d/%d': %v", doer.ID, repo.ID, err)
	}
}

func (a *actionNotifier) NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionCreateRepo,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error("notify watchers '%d/%d': %v", doer.ID, repo.ID, err)
	}
}

func (a *actionNotifier) NotifyPullRequestReview(pr *models.PullRequest, review *models.Review, comment *models.Comment, mentions []*models.User) {
	if err := review.LoadReviewer(); err != nil {
		log.Error("LoadReviewer '%d/%d': %v", review.ID, review.ReviewerID, err)
		return
	}
	if err := review.LoadCodeComments(); err != nil {
		log.Error("LoadCodeComments '%d/%d': %v", review.Reviewer.ID, review.ID, err)
		return
	}

	var actions = make([]*models.Action, 0, 10)
	for _, lines := range review.CodeComments {
		for _, comments := range lines {
			for _, comm := range comments {
				actions = append(actions, &models.Action{
					ActUserID: review.Reviewer.ID,
					ActUser:   review.Reviewer,
					Content:   fmt.Sprintf("%d|%s", review.Issue.Index, strings.Split(comm.Content, "\n")[0]),
					OpType:    models.ActionCommentPull,
					RepoID:    review.Issue.RepoID,
					Repo:      review.Issue.Repo,
					IsPrivate: review.Issue.Repo.IsPrivate,
					Comment:   comm,
					CommentID: comm.ID,
				})
			}
		}
	}

	if review.Type != models.ReviewTypeComment || strings.TrimSpace(comment.Content) != "" {
		action := &models.Action{
			ActUserID: review.Reviewer.ID,
			ActUser:   review.Reviewer,
			Content:   fmt.Sprintf("%d|%s", review.Issue.Index, strings.Split(comment.Content, "\n")[0]),
			RepoID:    review.Issue.RepoID,
			Repo:      review.Issue.Repo,
			IsPrivate: review.Issue.Repo.IsPrivate,
			Comment:   comment,
			CommentID: comment.ID,
		}

		switch review.Type {
		case models.ReviewTypeApprove:
			action.OpType = models.ActionApprovePullRequest
		case models.ReviewTypeReject:
			action.OpType = models.ActionRejectPullRequest
		default:
			action.OpType = models.ActionCommentPull
		}

		actions = append(actions, action)
	}

	if err := models.NotifyWatchersActions(actions); err != nil {
		log.Error("notify watchers '%d/%d': %v", review.Reviewer.ID, review.Issue.RepoID, err)
	}
}

func (*actionNotifier) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionMergePullRequest,
		Content:   fmt.Sprintf("%d|%s", pr.Issue.Index, pr.Issue.Title),
		RepoID:    pr.Issue.Repo.ID,
		Repo:      pr.Issue.Repo,
		IsPrivate: pr.Issue.Repo.IsPrivate,
	}); err != nil {
		log.Error("NotifyWatchers [%d]: %v", pr.ID, err)
	}
}

func (*actionNotifier) NotifyPullRevieweDismiss(doer *models.User, review *models.Review, comment *models.Comment) {
	reviewerName := review.Reviewer.Name
	if len(review.OriginalAuthor) > 0 {
		reviewerName = review.OriginalAuthor
	}
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    models.ActionPullReviewDismissed,
		Content:   fmt.Sprintf("%d|%s|%s", review.Issue.Index, reviewerName, comment.Content),
		RepoID:    review.Issue.Repo.ID,
		Repo:      review.Issue.Repo,
		IsPrivate: review.Issue.Repo.IsPrivate,
		CommentID: comment.ID,
		Comment:   comment,
	}); err != nil {
		log.Error("NotifyWatchers [%d]: %v", review.Issue.ID, err)
	}
}

func (a *actionNotifier) NotifyPushCommits(pusher *models.User, repo *models.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	data, err := json.Marshal(commits)
	if err != nil {
		log.Error("Marshal: %v", err)
		return
	}

	opType := models.ActionCommitRepo

	// Check it's tag push or branch.
	if opts.IsTag() {
		opType = models.ActionPushTag
		if opts.IsDelRef() {
			opType = models.ActionDeleteTag
		}
	} else if opts.IsDelRef() {
		opType = models.ActionDeleteBranch
	}

	if err = models.NotifyWatchers(&models.Action{
		ActUserID: pusher.ID,
		ActUser:   pusher,
		OpType:    opType,
		Content:   string(data),
		RepoID:    repo.ID,
		Repo:      repo,
		RefName:   opts.RefFullName,
		IsPrivate: repo.IsPrivate,
	}); err != nil {
		log.Error("notifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyCreateRef(doer *models.User, repo *models.Repository, refType, refFullName string) {
	opType := models.ActionCommitRepo
	if refType == "tag" {
		// has sent same action in `NotifyPushCommits`, so skip it.
		return
	}
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    opType,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName,
	}); err != nil {
		log.Error("notifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyDeleteRef(doer *models.User, repo *models.Repository, refType, refFullName string) {
	opType := models.ActionDeleteBranch
	if refType == "tag" {
		// has sent same action in `NotifyPushCommits`, so skip it.
		return
	}
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: doer.ID,
		ActUser:   doer,
		OpType:    opType,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName,
	}); err != nil {
		log.Error("notifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifySyncPushCommits(pusher *models.User, repo *models.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	data, err := json.Marshal(commits)
	if err != nil {
		log.Error("json.Marshal: %v", err)
		return
	}

	if err := models.NotifyWatchers(&models.Action{
		ActUserID: repo.OwnerID,
		ActUser:   repo.MustOwner(),
		OpType:    models.ActionMirrorSyncPush,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   opts.RefFullName,
		Content:   string(data),
	}); err != nil {
		log.Error("notifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifySyncCreateRef(doer *models.User, repo *models.Repository, refType, refFullName string) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: repo.OwnerID,
		ActUser:   repo.MustOwner(),
		OpType:    models.ActionMirrorSyncCreate,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName,
	}); err != nil {
		log.Error("notifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifySyncDeleteRef(doer *models.User, repo *models.Repository, refType, refFullName string) {
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: repo.OwnerID,
		ActUser:   repo.MustOwner(),
		OpType:    models.ActionMirrorSyncDelete,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		RefName:   refFullName,
	}); err != nil {
		log.Error("notifyWatchers: %v", err)
	}
}

func (a *actionNotifier) NotifyNewRelease(rel *models.Release) {
	if err := rel.LoadAttributes(); err != nil {
		log.Error("NotifyNewRelease: %v", err)
		return
	}
	if err := models.NotifyWatchers(&models.Action{
		ActUserID: rel.PublisherID,
		ActUser:   rel.Publisher,
		OpType:    models.ActionPublishRelease,
		RepoID:    rel.RepoID,
		Repo:      rel.Repo,
		IsPrivate: rel.Repo.IsPrivate,
		Content:   rel.Title,
		RefName:   rel.TagName,
	}); err != nil {
		log.Error("notifyWatchers: %v", err)
	}
}
