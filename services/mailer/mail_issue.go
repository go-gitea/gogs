// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
)

func fallbackMailSubject(issue *models.Issue) string {
	return fmt.Sprintf("[%s] %s (#%d)", issue.Repo.FullName(), issue.Title, issue.Index)
}

type mailCommentContext struct {
	Issue      *models.Issue
	Doer       *models.User
	ActionType models.ActionType
	Content    string
	Comment    *models.Comment
}

const (
	// MailBatchSize set the batch size used in mailIssueCommentBatch
	MailBatchSize = 100
)

// mailIssueCommentToParticipants can be used for both new issue creation and comment.
// This function sends two list of emails:
// 1. Repository watchers (except for WIP pull requests) and users who are participated in comments.
// 2. Users who are not in 1. but get mentioned in current issue/comment.
func mailIssueCommentToParticipants(ctx *mailCommentContext, mentions []*models.User) error {

	// Required by the mail composer; make sure to load these before calling the async function
	if err := ctx.Issue.LoadRepo(); err != nil {
		return fmt.Errorf("LoadRepo(): %v", err)
	}
	if err := ctx.Issue.LoadPoster(); err != nil {
		return fmt.Errorf("LoadPoster(): %v", err)
	}
	if err := ctx.Issue.LoadPullRequest(); err != nil {
		return fmt.Errorf("LoadPullRequest(): %v", err)
	}

	// Enough room to avoid reallocations
	unfiltered := make([]int64, 1, 64)

	// =========== Original poster ===========
	unfiltered[0] = ctx.Issue.PosterID

	// =========== Assignees ===========
	ids, err := models.GetAssigneeIDsByIssue(ctx.Issue.ID)
	if err != nil {
		return fmt.Errorf("GetAssigneeIDsByIssue(%d): %v", ctx.Issue.ID, err)
	}
	unfiltered = append(unfiltered, ids...)

	// =========== Participants (i.e. commenters, reviewers) ===========
	ids, err = models.GetParticipantsIDsByIssueID(ctx.Issue.ID)
	if err != nil {
		return fmt.Errorf("GetParticipantsIDsByIssueID(%d): %v", ctx.Issue.ID, err)
	}
	unfiltered = append(unfiltered, ids...)

	// =========== Issue watchers ===========
	ids, err = models.GetIssueWatchersIDs(ctx.Issue.ID, true)
	if err != nil {
		return fmt.Errorf("GetIssueWatchersIDs(%d): %v", ctx.Issue.ID, err)
	}
	unfiltered = append(unfiltered, ids...)

	// =========== Repo watchers ===========
	// Make repo watchers last, since it's likely the list with the most users
	if !(ctx.Issue.IsPull && ctx.Issue.PullRequest.IsWorkInProgress() && ctx.ActionType != models.ActionCreatePullRequest) {
		ids, err = models.GetRepoWatchersIDs(ctx.Issue.RepoID)
		if err != nil {
			return fmt.Errorf("GetRepoWatchersIDs(%d): %v", ctx.Issue.RepoID, err)
		}
		unfiltered = append(ids, unfiltered...)
	}

	visited := make(map[int64]bool, len(unfiltered)+len(mentions)+1)

	// Avoid mailing the doer
	visited[ctx.Doer.ID] = true

	// =========== Mentions ===========
	if err = mailIssueCommentBatch(ctx, mentions, visited, true); err != nil {
		return fmt.Errorf("mailIssueCommentBatch() mentions: %v", err)
	}

	// Avoid mailing explicit unwatched
	ids, err = models.GetIssueWatchersIDs(ctx.Issue.ID, false)
	if err != nil {
		return fmt.Errorf("GetIssueWatchersIDs(%d): %v", ctx.Issue.ID, err)
	}
	for _, i := range ids {
		visited[i] = true
	}

	unfilteredUsers, err := models.GetMaileableUsersByIDs(unfiltered, false)
	if err != nil {
		return err
	}
	if err = mailIssueCommentBatch(ctx, unfilteredUsers, visited, false); err != nil {
		return fmt.Errorf("mailIssueCommentBatch(): %v", err)
	}

	return nil
}

func mailIssueCommentBatch(ctx *mailCommentContext, users []*models.User, visited map[int64]bool, fromMention bool) error {
	checkUnit := models.UnitTypeIssues
	if ctx.Issue.IsPull {
		checkUnit = models.UnitTypePullRequests
	}

	langMap := make(map[string][]*models.User)
	for _, user := range users {
		// At this point we exclude:
		// user that don't have all mails enabled or users only get mail on mention and this is one ...
		if !(user.EmailNotificationsPreference == models.EmailNotificationsEnabled ||
			fromMention && user.EmailNotificationsPreference == models.EmailNotificationsOnMention) {
			continue
		}

		// if we have already visited this user we exclude them
		if _, ok := visited[user.ID]; ok {
			continue
		}

		// now mark them as visited
		visited[user.ID] = true

		// test if this user is allowed to see the issue/pull
		if !ctx.Issue.Repo.CheckUnitUser(user, checkUnit) {
			continue
		}

		langMap[user.Language] = append(langMap[user.Language], user)
	}

	for lang, receivers := range langMap {
		// because we know that the len(receivers) > 0 and we don't care about the order particularly
		// working backwards from the last (possibly) incomplete batch. If len(receivers) can be 0 this
		// starting condition will need to be changed slightly
		for i := ((len(receivers) - 1) / MailBatchSize) * MailBatchSize; i >= 0; i -= MailBatchSize {
			msgs, err := composeIssueCommentMessages(ctx, lang, receivers[i:], fromMention, "issue comments")
			if err != nil {
				return err
			}
			SendAsyncs(msgs)
			receivers = receivers[:i]
		}
	}

	return nil
}

// MailParticipants sends new issue thread created emails to repository watchers
// and mentioned people.
func MailParticipants(issue *models.Issue, doer *models.User, opType models.ActionType, mentions []*models.User) error {
	content := issue.Content
	if opType == models.ActionCloseIssue || opType == models.ActionClosePullRequest ||
		opType == models.ActionReopenIssue || opType == models.ActionReopenPullRequest ||
		opType == models.ActionMergePullRequest {
		content = ""
	}
	if err := mailIssueCommentToParticipants(
		&mailCommentContext{
			Issue:      issue,
			Doer:       doer,
			ActionType: opType,
			Content:    content,
			Comment:    nil,
		}, mentions); err != nil {
		log.Error("mailIssueCommentToParticipants: %v", err)
	}
	return nil
}
