// Copyright 2021 The Gitea Authors. All rights reserved.
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

func TestFeishuPayload(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Create(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, `[test/repo] branch test created`, pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Delete(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, `[test/repo] branch test deleted`, pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Fork(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, `test/repo2 is forked to test/repo`, pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Push(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[test/repo:test] \r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\r\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		d := new(FeishuPayload)
		p.Action = api.HookIssueOpened
		pl, err := d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "#2 crash\r\n[test/repo] Issue opened: #2 crash by user1\r\n\r\nissue body", pl.(*FeishuPayload).Content.Text)

		p.Action = api.HookIssueClosed
		pl, err = d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "#2 crash\r\n[test/repo] Issue closed: #2 crash by user1", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		d := new(FeishuPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "#2 crash\r\n[test/repo] New comment on issue #2 crash by user1\r\n\r\nmore info needed", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		d := new(FeishuPayload)
		pl, err := d.PullRequest(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "#12 Fix bug\r\n[test/repo] Pull request opened: #12 Fix bug by user1\r\n\r\nfixes bug #2", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		d := new(FeishuPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "#12 Fix bug\r\n[test/repo] New comment on pull request #12 Fix bug by user1\r\n\r\nchanges requested", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		d := new(FeishuPayload)
		pl, err := d.Review(p, models.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[test/repo] Pull request review approved : #12 Fix bug\r\n\r\ngood job", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Repository(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[test/repo] Repository created", pl.(*FeishuPayload).Content.Text)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		d := new(FeishuPayload)
		pl, err := d.Release(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &FeishuPayload{}, pl)

		assert.Equal(t, "[test/repo] Release created: v1.0 by user1", pl.(*FeishuPayload).Content.Text)
	})
}

func TestFeishuJSONPayload(t *testing.T) {
	p := pushTestPayload()

	pl, err := new(FeishuPayload).Push(p)
	require.NoError(t, err)
	require.NotNil(t, pl)
	require.IsType(t, &FeishuPayload{}, pl)

	json, err := pl.JSONPayload()
	require.NoError(t, err)
	assert.NotEmpty(t, json)
}
