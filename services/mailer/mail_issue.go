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

// mailIssueCommentToParticipants can be used for both new issue creation and comment.
// This function sends two list of emails:
// 1. Repository watchers and users who are participated in comments.
// 2. Users who are not in 1. but get mentioned in current issue/comment.
func mailIssueCommentToParticipants(ctx *mailCommentContext, mentions []int64) error {
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
	ids, err = models.GetRepoWatchersIDs(ctx.Issue.RepoID)
	if err != nil {
		return fmt.Errorf("GetRepoWatchersIDs(%d): %v", ctx.Issue.RepoID, err)
	}
	unfiltered = append(ids, unfiltered...)

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

	if err = mailIssueCommentBatch(ctx, unfiltered, visited, false); err != nil {
		return fmt.Errorf("mailIssueCommentBatch(): %v", err)
	}

	return nil
}

func mailIssueCommentBatch(ctx *mailCommentContext, ids []int64, visited map[int64]bool, fromMention bool) error {
	const batchSize = 100
	for i := 0; i < len(ids); i += batchSize {
		var last int
		if i+batchSize < len(ids) {
			last = i + batchSize
		} else {
			last = len(ids)
		}
		unique := make([]int64, 0, last-i)
		for j := i; j < last; j++ {
			id := ids[j]
			if _, ok := visited[id]; !ok {
				unique = append(unique, id)
				visited[id] = true
			}
		}
		recipients, err := models.GetMaileableUsersByIDs(unique, fromMention)
		if err != nil {
			return err
		}

		checkUnit := models.UnitTypeIssues
		if ctx.Issue.IsPull {
			checkUnit = models.UnitTypePullRequests
		}
		// Make sure all recipients can still see the issue
		idx := 0
		for _, r := range recipients {
			if ctx.Issue.Repo.CheckUnitUser(r, checkUnit) {
				recipients[idx] = r
				idx++
			}
		}
		recipients = recipients[:idx]

		// TODO: Separate recipients by language for i18n mail templates
		tos := make([]string, len(recipients))
		for i := range recipients {
			tos[i] = recipients[i].Email
		}
		SendAsyncs(composeIssueCommentMessages(ctx, tos, fromMention, "issue comments"))
	}
	return nil
}

// MailParticipants sends new issue thread created emails to repository watchers
// and mentioned people.
func MailParticipants(issue *models.Issue, doer *models.User, opType models.ActionType, mentions []*models.User) error {
	return mailParticipants(issue, doer, opType, mentions)
}

func mailParticipants(issue *models.Issue, doer *models.User, opType models.ActionType, mentions []*models.User) (err error) {
	mentionedIDs := make([]int64, len(mentions))
	for i, u := range mentions {
		mentionedIDs[i] = u.ID
	}
	if err = mailIssueCommentToParticipants(
		&mailCommentContext{
			Issue:      issue,
			Doer:       doer,
			ActionType: opType,
			Content:    issue.Content,
			Comment:    nil,
		}, mentionedIDs); err != nil {
		log.Error("mailIssueCommentToParticipants: %v", err)
	}
	return nil
}
