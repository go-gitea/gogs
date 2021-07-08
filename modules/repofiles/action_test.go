// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestUpdateIssuesCommit(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	pushCommits := []*repository.PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "start working on #FST-1, #1",
		},
		{
			Sha1:           "abcdef2",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "a plain message",
		},
		{
			Sha1:           "abcdef2",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "close #2",
		},
	}

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repo.Owner = user

	commentBean := &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   1,
	}
	issueBean := &models.Issue{RepoID: repo.ID, Index: 4}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, &models.Issue{RepoID: repo.ID, Index: 2}, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, repo.DefaultBranch))
	models.AssertExistsAndLoadBean(t, commentBean)
	models.AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})

	// Test that push to a non-default branch closes no issue.
	pushCommits = []*repository.PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "close #1",
		},
	}
	repo = models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)
	commentBean = &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   6,
	}
	issueBean = &models.Issue{RepoID: repo.ID, Index: 1}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, &models.Issue{RepoID: repo.ID, Index: 1}, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, "non-existing-branch"))
	models.AssertExistsAndLoadBean(t, commentBean)
	models.AssertNotExistsBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})

	pushCommits = []*repository.PushCommit{
		{
			Sha1:           "abcdef3",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "close " + setting.AppURL + repo.FullName() + "/pulls/1",
		},
	}
	repo = models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)
	commentBean = &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef3",
		PosterID:  user.ID,
		IssueID:   6,
	}
	issueBean = &models.Issue{RepoID: repo.ID, Index: 1}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, &models.Issue{RepoID: repo.ID, Index: 1}, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, repo.DefaultBranch))
	models.AssertExistsAndLoadBean(t, commentBean)
	models.AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})
}

func TestUpdateIssuesCommit_Colon(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	pushCommits := []*repository.PushCommit{
		{
			Sha1:           "abcdef2",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "close: #2",
		},
	}

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repo.Owner = user

	issueBean := &models.Issue{RepoID: repo.ID, Index: 4}

	models.AssertNotExistsBean(t, &models.Issue{RepoID: repo.ID, Index: 2}, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, repo.DefaultBranch))
	models.AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})
}

func TestUpdateIssuesCommit_Issue5957(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	// Test that push to a non-default branch closes an issue.
	pushCommits := []*repository.PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "close #2",
		},
	}

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	commentBean := &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   7,
	}

	issueBean := &models.Issue{RepoID: repo.ID, Index: 2, ID: 7}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, issueBean, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, "non-existing-branch"))
	models.AssertExistsAndLoadBean(t, commentBean)
	models.AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})
}

func TestUpdateIssuesCommit_AnotherRepo(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	// Test that a push to default branch closes issue in another repo
	// If the user also has push permissions to that repo
	pushCommits := []*repository.PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "close user2/repo1#1",
		},
	}

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	commentBean := &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   1,
	}

	issueBean := &models.Issue{RepoID: 1, Index: 1, ID: 1}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, issueBean, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, repo.DefaultBranch))
	models.AssertExistsAndLoadBean(t, commentBean)
	models.AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})
}

func TestUpdateIssuesCommit_AnotherRepo_FullAddress(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)

	// Test that a push to default branch closes issue in another repo
	// If the user also has push permissions to that repo
	pushCommits := []*repository.PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "close " + setting.AppURL + "user2/repo1/issues/1",
		},
	}

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	commentBean := &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   1,
	}

	issueBean := &models.Issue{RepoID: 1, Index: 1, ID: 1}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, issueBean, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, repo.DefaultBranch))
	models.AssertExistsAndLoadBean(t, commentBean)
	models.AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})
}

func TestUpdateIssuesCommit_AnotherRepoNoPermission(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 10}).(*models.User)

	// Test that a push with close reference *can not* close issue
	// If the committer doesn't have push rights in that repo
	pushCommits := []*repository.PushCommit{
		{
			Sha1:           "abcdef3",
			CommitterEmail: "user10@example.com",
			CommitterName:  "User Ten",
			AuthorEmail:    "user10@example.com",
			AuthorName:     "User Ten",
			Message:        "close user3/repo3#1",
		},
		{
			Sha1:           "abcdef4",
			CommitterEmail: "user10@example.com",
			CommitterName:  "User Ten",
			AuthorEmail:    "user10@example.com",
			AuthorName:     "User Ten",
			Message:        "close " + setting.AppURL + "user3/repo3/issues/1",
		},
	}

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 6}).(*models.Repository)
	commentBean := &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef3",
		PosterID:  user.ID,
		IssueID:   6,
	}
	commentBean2 := &models.Comment{
		Type:      models.CommentTypeCommitRef,
		CommitSHA: "abcdef4",
		PosterID:  user.ID,
		IssueID:   6,
	}

	issueBean := &models.Issue{RepoID: 3, Index: 1, ID: 6}

	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, commentBean2)
	models.AssertNotExistsBean(t, issueBean, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, repo.DefaultBranch))
	models.AssertNotExistsBean(t, commentBean)
	models.AssertNotExistsBean(t, commentBean2)
	models.AssertNotExistsBean(t, issueBean, "is_closed=1")
	models.CheckConsistencyFor(t, &models.Action{})
}
