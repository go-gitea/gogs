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

func TestMSTeamsPayload(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		p := createTestPayload()

		d := new(MSTeamsPayload)
		pl, err := d.Create(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "[test/repo] branch test created", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "[test/repo] branch test created", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 2)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repo.FullName, fact.Value)
			} else if fact.Name == "branch:" {
				assert.Equal(t, "test", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)
	})

	t.Run("Delete", func(t *testing.T) {
		p := deleteTestPayload()

		d := new(MSTeamsPayload)
		pl, err := d.Delete(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "[test/repo] branch test deleted", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "[test/repo] branch test deleted", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 2)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repo.FullName, fact.Value)
			} else if fact.Name == "branch:" {
				assert.Equal(t, "test", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)
	})

	t.Run("Fork", func(t *testing.T) {
		p := forkTestPayload()

		d := new(MSTeamsPayload)
		pl, err := d.Fork(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "test/repo2 is forked to test/repo", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "test/repo2 is forked to test/repo", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 2)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repo.FullName, fact.Value)
			} else if fact.Name == "Forkee:" {
				assert.Equal(t, p.Forkee.FullName, fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)
	})

	t.Run("Push", func(t *testing.T) {
		p := pushTestPayload()

		d := new(MSTeamsPayload)
		pl, err := d.Push(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "[test/repo:test] 2 new commits", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "[test/repo:test] 2 new commits", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Equal(t, "[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1\n\n[2020558](http://localhost:3000/test/repo/commit/2020558fe2e34debb818a514715839cabd25e778) commit message - user1", pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 2)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repo.FullName, fact.Value)
			} else if fact.Name == "Commit count:" {
				assert.Equal(t, "2", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/src/test", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)
	})

	t.Run("Issue", func(t *testing.T) {
		p := issueTestPayload()

		d := new(MSTeamsPayload)
		p.Action = api.HookIssueOpened
		pl, err := d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "[test/repo] Issue opened: #2 crash", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "[test/repo] Issue opened: #2 crash", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Equal(t, "issue body", pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 2)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Issue #:" {
				assert.Equal(t, "2", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)

		p.Action = api.HookIssueClosed
		pl, err = d.Issue(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "[test/repo] Issue closed: #2 crash", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "[test/repo] Issue closed: #2 crash", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 2)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Issue #:" {
				assert.Equal(t, "2", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)
	})

	t.Run("IssueComment", func(t *testing.T) {
		p := issueCommentTestPayload()

		d := new(MSTeamsPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "[test/repo] New comment on issue #2 crash", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "[test/repo] New comment on issue #2 crash", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Equal(t, "more info needed", pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 2)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Issue #:" {
				assert.Equal(t, "2", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/issues/2#issuecomment-4", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)
	})

	t.Run("PullRequest", func(t *testing.T) {
		p := pullRequestTestPayload()

		d := new(MSTeamsPayload)
		pl, err := d.PullRequest(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "[test/repo] Pull request opened: #12 Fix bug", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "[test/repo] Pull request opened: #12 Fix bug", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Equal(t, "fixes bug #2", pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 2)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Pull request #:" {
				assert.Equal(t, "12", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)
	})

	t.Run("PullRequestComment", func(t *testing.T) {
		p := pullRequestCommentTestPayload()

		d := new(MSTeamsPayload)
		pl, err := d.IssueComment(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "[test/repo] New comment on pull request #12 Fix bug", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "[test/repo] New comment on pull request #12 Fix bug", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Equal(t, "changes requested", pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 2)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Issue #:" {
				assert.Equal(t, "12", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12#issuecomment-4", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)
	})

	t.Run("Review", func(t *testing.T) {
		p := pullRequestTestPayload()
		p.Action = api.HookIssueReviewed

		d := new(MSTeamsPayload)
		pl, err := d.Review(p, models.HookEventPullRequestReviewApproved)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "[test/repo] Pull request review approved: #12 Fix bug", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "[test/repo] Pull request review approved: #12 Fix bug", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Equal(t, "good job", pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 2)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Pull request #:" {
				assert.Equal(t, "12", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo/pulls/12", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)
	})

	t.Run("Repository", func(t *testing.T) {
		p := repositoryTestPayload()

		d := new(MSTeamsPayload)
		pl, err := d.Repository(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "[test/repo] Repository created", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "[test/repo] Repository created", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 1)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/test/repo", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)
	})

	t.Run("Release", func(t *testing.T) {
		p := pullReleaseTestPayload()

		d := new(MSTeamsPayload)
		pl, err := d.Release(p)
		require.NoError(t, err)
		require.NotNil(t, pl)
		require.IsType(t, &MSTeamsPayload{}, pl)

		assert.Equal(t, "[test/repo] Release created: v1.0", pl.(*MSTeamsPayload).Title)
		assert.Equal(t, "[test/repo] Release created: v1.0", pl.(*MSTeamsPayload).Summary)
		assert.Len(t, pl.(*MSTeamsPayload).Sections, 1)
		assert.Equal(t, "user1", pl.(*MSTeamsPayload).Sections[0].ActivitySubtitle)
		assert.Empty(t, pl.(*MSTeamsPayload).Sections[0].Text)
		assert.Len(t, pl.(*MSTeamsPayload).Sections[0].Facts, 2)
		for _, fact := range pl.(*MSTeamsPayload).Sections[0].Facts {
			if fact.Name == "Repository:" {
				assert.Equal(t, p.Repository.FullName, fact.Value)
			} else if fact.Name == "Tag:" {
				assert.Equal(t, "v1.0", fact.Value)
			} else {
				t.Fail()
			}
		}
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction, 1)
		assert.Len(t, pl.(*MSTeamsPayload).PotentialAction[0].Targets, 1)
		assert.Equal(t, "http://localhost:3000/api/v1/repos/test/repo/releases/2", pl.(*MSTeamsPayload).PotentialAction[0].Targets[0].URI)
	})
}

func TestMSTeamsJSONPayload(t *testing.T) {
	p := pushTestPayload()

	pl, err := new(MSTeamsPayload).Push(p)
	require.NoError(t, err)
	require.NotNil(t, pl)
	require.IsType(t, &MSTeamsPayload{}, pl)

	json, err := pl.JSONPayload()
	require.NoError(t, err)
	assert.NotEmpty(t, json)
}
