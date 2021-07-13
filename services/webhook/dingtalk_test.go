// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDingTalkPayload(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		d := new(DingtalkPayload)
		pl, err := d.Create(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "[test/repo] branch test created", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "[test/repo] branch test created", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view ref test", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.(*DingtalkPayload).ActionCard.SingleURL)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		d := new(DingtalkPayload)
		pl, err := d.Delete(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "[test/repo] branch test deleted", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "[test/repo] branch test deleted", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view ref test", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.(*DingtalkPayload).ActionCard.SingleURL)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		d := new(DingtalkPayload)
		pl, err := d.Fork(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "test/repo2 is forked to test/repo", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "test/repo2 is forked to test/repo", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view forked repo test/repo", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.(*DingtalkPayload).ActionCard.SingleURL)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		d := new(DingtalkPayload)
		pl, err := d.Push(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "[test/repo:test] 2 new commits", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view commit 2020558...2020558", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.(*DingtalkPayload).ActionCard.SingleURL)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		d := new(DingtalkPayload)
		p.Action = api.HookIssueOpened
		pl, err := d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "[test/repo] Issue opened: #2 crash by user1\r\n\r\nissue body", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "#2 crash", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view issue", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.(*DingtalkPayload).ActionCard.SingleURL)

		p.Action = api.HookIssueClosed
		pl, err = d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "[test/repo] Issue closed: #2 crash by user1", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "#2 crash", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view issue", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.(*DingtalkPayload).ActionCard.SingleURL)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		d := new(DingtalkPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "[test/repo] New comment on issue #2 crash by user1\r\n\r\nmore info needed", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "#2 crash", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view issue comment", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2#issuecomment-4", pl.(*DingtalkPayload).ActionCard.SingleURL)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		d := new(DingtalkPayload)
		pl, err := d.PullRequest(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "[test/repo] Pull request opened: #12 Fix bug by user1\r\n\r\nfixes bug #2", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "#12 Fix bug", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view pull request", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.(*DingtalkPayload).ActionCard.SingleURL)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		d := new(DingtalkPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "[test/repo] New comment on pull request #12 Fix bug by user1\r\n\r\nchanges requested", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "#12 Fix bug", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view issue comment", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12#issuecomment-4", pl.(*DingtalkPayload).ActionCard.SingleURL)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		d := new(DingtalkPayload)
		pl, err := d.Review(p, models.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "[test/repo] Pull request review approved : #12 Fix bug\r\n\r\ngood job", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "[test/repo] Pull request review approved : #12 Fix bug", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view pull request", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.(*DingtalkPayload).ActionCard.SingleURL)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		d := new(DingtalkPayload)
		pl, err := d.Repository(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "[test/repo] Repository created", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "[test/repo] Repository created", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view repository", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.(*DingtalkPayload).ActionCard.SingleURL)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		d := new(DingtalkPayload)
		pl, err := d.Release(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DingtalkPayload{}, pl)

		assert.Equal(t, "[test/repo] Release created: v1.0 by user1", pl.(*DingtalkPayload).ActionCard.Text)
		assert.Equal(t, "[test/repo] Release created: v1.0 by user1", pl.(*DingtalkPayload).ActionCard.Title)
		assert.Equal(t, "view release", pl.(*DingtalkPayload).ActionCard.SingleTitle)
		assert.Equal(t, "http://localhost:3000/api/v1/repos/test/repo/releases/2", pl.(*DingtalkPayload).ActionCard.SingleURL)
	})
}

func TestDingTalkJSONPayload(t *testing.T) {
	p := pushTestPayload()

	pl, err := new(DingtalkPayload).Push(p)
	require.NoError(t, err)
	require.NotNil(t, pl)
	require.IsType(t, &DingtalkPayload{}, pl)

	json, err := pl.JSONPayload()
	require.NoError(t, err)
	assert.NotEmpty(t, json)
}
