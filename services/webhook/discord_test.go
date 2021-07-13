// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscordPayload(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		d := new(DiscordPayload)
		pl, err := d.Create(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "[test/repo] branch test created", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Empty(t, pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		d := new(DiscordPayload)
		pl, err := d.Delete(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "[test/repo] branch test deleted", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Empty(t, pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		d := new(DiscordPayload)
		pl, err := d.Fork(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "test/repo2 is forked to test/repo", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Empty(t, pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		d := new(DiscordPayload)
		pl, err := d.Push(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "[test/repo:test] 2 new commits", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Equal(t, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		d := new(DiscordPayload)
		p.Action = api.HookIssueOpened
		pl, err := d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "[test/repo] Issue opened: #2 crash", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Equal(t, "issue body", pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)

		p.Action = api.HookIssueClosed
		pl, err = d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "[test/repo] Issue closed: #2 crash", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Empty(t, pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		d := new(DiscordPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "[test/repo] New comment on issue #2 crash", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Equal(t, "more info needed", pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2#issuecomment-4", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		d := new(DiscordPayload)
		pl, err := d.PullRequest(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "[test/repo] Pull request opened: #12 Fix bug", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Equal(t, "fixes bug #2", pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		d := new(DiscordPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "[test/repo] New comment on pull request #12 Fix bug", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Equal(t, "changes requested", pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12#issuecomment-4", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		d := new(DiscordPayload)
		pl, err := d.Review(p, models.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "[test/repo] Pull request review approved: #12 Fix bug", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Equal(t, "good job", pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		d := new(DiscordPayload)
		pl, err := d.Repository(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "[test/repo] Repository created", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Empty(t, pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		d := new(DiscordPayload)
		pl, err := d.Release(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &DiscordPayload{}, pl)

		assert.Len(t, pl.(*DiscordPayload).Embeds, 1)
		assert.Equal(t, "[test/repo] Release created: v1.0", pl.(*DiscordPayload).Embeds[0].Title)
		assert.Equal(t, "Note of first stable release", pl.(*DiscordPayload).Embeds[0].Description)
		assert.Equal(t, "http://localhost:3000/api/v1/repos/test/repo/releases/2", pl.(*DiscordPayload).Embeds[0].URL)
		assert.Equal(t, p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.Name)
		assert.Equal(t, setting.AppURL+p.Sender.UserName, pl.(*DiscordPayload).Embeds[0].Author.URL)
		assert.Equal(t, p.Sender.AvatarURL, pl.(*DiscordPayload).Embeds[0].Author.IconURL)
	})
}

func TestDiscordJSONPayload(t *testing.T) {
	p := pushTestPayload()

	pl, err := new(DiscordPayload).Push(p)
	require.NoError(t, err)
	require.NotNil(t, pl)
	require.IsType(t, &DiscordPayload{}, pl)

	json, err := pl.JSONPayload()
	require.NoError(t, err)
	assert.NotEmpty(t, json)
}
