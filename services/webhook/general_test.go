// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"testing"

	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func createTestPayload() *api.CreatePayload {
	return &api.CreatePayload{
		Sha:     "2020558fe2e34debb818a514715839cabd25e777",
		Ref:     "refs/heads/test",
		RefType: "branch",
		Repo: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
	}
}

func deleteTestPayload() *api.DeletePayload {
	return &api.DeletePayload{
		Ref:     "refs/heads/test",
		RefType: "branch",
		Repo: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
	}
}

func forkTestPayload() *api.ForkPayload {
	return &api.ForkPayload{
		Forkee: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo2",
			Name:     "repo2",
			FullName: "test/repo2",
		},
		Repo: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
	}
}

func pushTestPayload() *api.PushPayload {
	commit := &api.PayloadCommit{
		ID:      "2020558fe2e34debb818a514715839cabd25e778",
		Message: "commit message",
		URL:     "http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778",
		Author: &api.PayloadUser{
			Name:     "user1",
			Email:    "user1@localhost",
			UserName: "user1",
		},
		Committer: &api.PayloadUser{
			Name:     "user1",
			Email:    "user1@localhost",
			UserName: "user1",
		},
	}

	return &api.PushPayload{
		Ref:        "refs/heads/test",
		Before:     "2020558fe2e34debb818a514715839cabd25e777",
		After:      "2020558fe2e34debb818a514715839cabd25e778",
		CompareURL: "",
		HeadCommit: commit,
		Commits:    []*api.PayloadCommit{commit, commit},
		Repo: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Pusher: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
	}
}

func issueTestPayload() *api.IssuePayload {
	return &api.IssuePayload{
		Index: 2,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Issue: &api.Issue{
			ID:      2,
			Index:   2,
			URL:     "http://localhost:3000/api/v1/repos/test/repo/issues/2",
			HTMLURL: "http://localhost:3000/test/repo/issues/2",
			Title:   "crash",
			Body:    "issue body",
			Assignees: []*api.User{
				{
					UserName:  "user1",
					AvatarURL: "http://localhost:3000/user1/avatar",
				},
			},
			Milestone: &api.Milestone{
				ID:          1,
				Title:       "Milestone Title",
				Description: "Milestone Description",
			},
		},
	}
}

func issueCommentTestPayload() *api.IssueCommentPayload {
	return &api.IssueCommentPayload{
		Action: api.HookIssueCommentCreated,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Comment: &api.Comment{
			HTMLURL:  "http://localhost:3000/test/repo/issues/2#issuecomment-4",
			IssueURL: "http://localhost:3000/test/repo/issues/2",
			Body:     "more info needed",
		},
		Issue: &api.Issue{
			ID:      2,
			Index:   2,
			URL:     "http://localhost:3000/api/v1/repos/test/repo/issues/2",
			HTMLURL: "http://localhost:3000/test/repo/issues/2",
			Title:   "crash",
			Body:    "this happened",
		},
	}
}

func pullRequestCommentTestPayload() *api.IssueCommentPayload {
	return &api.IssueCommentPayload{
		Action: api.HookIssueCommentCreated,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Comment: &api.Comment{
			HTMLURL: "http://localhost:3000/test/repo/pulls/12#issuecomment-4",
			PRURL:   "http://localhost:3000/test/repo/pulls/12",
			Body:    "changes requested",
		},
		Issue: &api.Issue{
			ID:      12,
			Index:   12,
			URL:     "http://localhost:3000/api/v1/repos/test/repo/pulls/12",
			HTMLURL: "http://localhost:3000/test/repo/pulls/12",
			Title:   "Fix bug",
			Body:    "fixes bug #2",
		},
		IsPull: true,
	}
}

func pullReleaseTestPayload() *api.ReleasePayload {
	return &api.ReleasePayload{
		Action: api.HookReleasePublished,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		Release: &api.Release{
			TagName: "v1.0",
			Target:  "master",
			Title:   "First stable release",
			Note:    "Note of first stable release",
			URL:     "http://localhost:3000/api/v1/repos/test/repo/releases/2",
		},
	}
}

func pullRequestTestPayload() *api.PullRequestPayload {
	return &api.PullRequestPayload{
		Action: api.HookIssueOpened,
		Index:  12,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
		PullRequest: &api.PullRequest{
			ID:        12,
			Index:     12,
			URL:       "http://localhost:3000/test/repo/pulls/12",
			HTMLURL:   "http://localhost:3000/test/repo/pulls/12",
			Title:     "Fix bug",
			Body:      "fixes bug #2",
			Mergeable: true,
			Assignees: []*api.User{
				{
					UserName:  "user1",
					AvatarURL: "http://localhost:3000/user1/avatar",
				},
			},
			Milestone: &api.Milestone{
				ID:          1,
				Title:       "Milestone Title",
				Description: "Milestone Description",
			},
		},
		Review: &api.ReviewPayload{
			Content: "good job",
		},
	}
}

func repositoryTestPayload() *api.RepositoryPayload {
	return &api.RepositoryPayload{
		Action: api.HookRepoCreated,
		Sender: &api.User{
			UserName:  "user1",
			AvatarURL: "http://localhost:3000/user1/avatar",
		},
		Repository: &api.Repository{
			HTMLURL:  "http://localhost:3000/test/repo",
			Name:     "repo",
			FullName: "test/repo",
		},
	}
}

func TestGetIssuesPayloadInfo(t *testing.T) {
	p := issueTestPayload()

	cases := []struct {
		action         api.HookIssueAction
		text           string
		issueTitle     string
		attachmentText string
		color          int
	}{
		{
			api.HookIssueOpened,
			"[test/repo] Issue opened: #2 crash by user1",
			"#2 crash",
			"issue body",
			orangeColor,
		},
		{
			api.HookIssueClosed,
			"[test/repo] Issue closed: #2 crash by user1",
			"#2 crash",
			"",
			redColor,
		},
		{
			api.HookIssueReOpened,
			"[test/repo] Issue re-opened: #2 crash by user1",
			"#2 crash",
			"",
			yellowColor,
		},
		{
			api.HookIssueEdited,
			"[test/repo] Issue edited: #2 crash by user1",
			"#2 crash",
			"issue body",
			yellowColor,
		},
		{
			api.HookIssueAssigned,
			"[test/repo] Issue assigned to user1: #2 crash by user1",
			"#2 crash",
			"",
			greenColor,
		},
		{
			api.HookIssueUnassigned,
			"[test/repo] Issue unassigned: #2 crash by user1",
			"#2 crash",
			"",
			yellowColor,
		},
		{
			api.HookIssueLabelUpdated,
			"[test/repo] Issue labels updated: #2 crash by user1",
			"#2 crash",
			"",
			yellowColor,
		},
		{
			api.HookIssueLabelCleared,
			"[test/repo] Issue labels cleared: #2 crash by user1",
			"#2 crash",
			"",
			yellowColor,
		},
		{
			api.HookIssueSynchronized,
			"[test/repo] Issue synchronized: #2 crash by user1",
			"#2 crash",
			"",
			yellowColor,
		},
		{
			api.HookIssueMilestoned,
			"[test/repo] Issue milestoned to Milestone Title: #2 crash by user1",
			"#2 crash",
			"",
			yellowColor,
		},
		{
			api.HookIssueDemilestoned,
			"[test/repo] Issue milestone cleared: #2 crash by user1",
			"#2 crash",
			"",
			yellowColor,
		},
	}

	for i, c := range cases {
		p.Action = c.action
		text, issueTitle, attachmentText, color := getIssuesPayloadInfo(p, noneLinkFormatter, true)
		assert.Equal(t, c.text, text, "case %d", i)
		assert.Equal(t, c.issueTitle, issueTitle, "case %d", i)
		assert.Equal(t, c.attachmentText, attachmentText, "case %d", i)
		assert.Equal(t, c.color, color, "case %d", i)
	}
}

func TestGetPullRequestPayloadInfo(t *testing.T) {
	p := pullRequestTestPayload()

	cases := []struct {
		action         api.HookIssueAction
		text           string
		issueTitle     string
		attachmentText string
		color          int
	}{
		{
			api.HookIssueOpened,
			"[test/repo] Pull request opened: #12 Fix bug by user1",
			"#12 Fix bug",
			"fixes bug #2",
			greenColor,
		},
		{
			api.HookIssueClosed,
			"[test/repo] Pull request closed: #12 Fix bug by user1",
			"#12 Fix bug",
			"",
			redColor,
		},
		{
			api.HookIssueReOpened,
			"[test/repo] Pull request re-opened: #12 Fix bug by user1",
			"#12 Fix bug",
			"",
			yellowColor,
		},
		{
			api.HookIssueEdited,
			"[test/repo] Pull request edited: #12 Fix bug by user1",
			"#12 Fix bug",
			"fixes bug #2",
			yellowColor,
		},
		{
			api.HookIssueAssigned,
			"[test/repo] Pull request assigned to user1: #12 Fix bug by user1",
			"#12 Fix bug",
			"",
			greenColor,
		},
		{
			api.HookIssueUnassigned,
			"[test/repo] Pull request unassigned: #12 Fix bug by user1",
			"#12 Fix bug",
			"",
			yellowColor,
		},
		{
			api.HookIssueLabelUpdated,
			"[test/repo] Pull request labels updated: #12 Fix bug by user1",
			"#12 Fix bug",
			"",
			yellowColor,
		},
		{
			api.HookIssueLabelCleared,
			"[test/repo] Pull request labels cleared: #12 Fix bug by user1",
			"#12 Fix bug",
			"",
			yellowColor,
		},
		{
			api.HookIssueSynchronized,
			"[test/repo] Pull request synchronized: #12 Fix bug by user1",
			"#12 Fix bug",
			"",
			yellowColor,
		},
		{
			api.HookIssueMilestoned,
			"[test/repo] Pull request milestoned to Milestone Title: #12 Fix bug by user1",
			"#12 Fix bug",
			"",
			yellowColor,
		},
		{
			api.HookIssueDemilestoned,
			"[test/repo] Pull request milestone cleared: #12 Fix bug by user1",
			"#12 Fix bug",
			"",
			yellowColor,
		},
	}

	for i, c := range cases {
		p.Action = c.action
		text, issueTitle, attachmentText, color := getPullRequestPayloadInfo(p, noneLinkFormatter, true)
		assert.Equal(t, c.text, text, "case %d", i)
		assert.Equal(t, c.issueTitle, issueTitle, "case %d", i)
		assert.Equal(t, c.attachmentText, attachmentText, "case %d", i)
		assert.Equal(t, c.color, color, "case %d", i)
	}
}

func TestGetReleasePayloadInfo(t *testing.T) {
	p := pullReleaseTestPayload()

	cases := []struct {
		action api.HookReleaseAction
		text   string
		color  int
	}{
		{
			api.HookReleasePublished,
			"[test/repo] Release created: v1.0 by user1",
			greenColor,
		},
		{
			api.HookReleaseUpdated,
			"[test/repo] Release updated: v1.0 by user1",
			yellowColor,
		},
		{
			api.HookReleaseDeleted,
			"[test/repo] Release deleted: v1.0 by user1",
			redColor,
		},
	}

	for i, c := range cases {
		p.Action = c.action
		text, color := getReleasePayloadInfo(p, noneLinkFormatter, true)
		assert.Equal(t, c.text, text, "case %d", i)
		assert.Equal(t, c.color, color, "case %d", i)
	}
}

func TestGetIssueCommentPayloadInfo(t *testing.T) {
	p := pullRequestCommentTestPayload()

	cases := []struct {
		action     api.HookIssueCommentAction
		text       string
		issueTitle string
		color      int
	}{
		{
			api.HookIssueCommentCreated,
			"[test/repo] New comment on pull request #12 Fix bug by user1",
			"#12 Fix bug",
			greenColorLight,
		},
		{
			api.HookIssueCommentEdited,
			"[test/repo] Comment edited on pull request #12 Fix bug by user1",
			"#12 Fix bug",
			yellowColor,
		},
		{
			api.HookIssueCommentDeleted,
			"[test/repo] Comment deleted on pull request #12 Fix bug by user1",
			"#12 Fix bug",
			redColor,
		},
	}

	for i, c := range cases {
		p.Action = c.action
		text, issueTitle, color := getIssueCommentPayloadInfo(p, noneLinkFormatter, true)
		assert.Equal(t, c.text, text, "case %d", i)
		assert.Equal(t, c.issueTitle, issueTitle, "case %d", i)
		assert.Equal(t, c.color, color, "case %d", i)
	}
}
