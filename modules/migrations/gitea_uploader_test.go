// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/stretchr/testify/assert"
)

func TestGiteaUploadRepo(t *testing.T) {
	// FIXME: Since no accesskey or user/password will trigger rate limit of github, just skip
	t.Skip()

	models.PrepareTestEnv(t)

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)

	var (
		downloader = NewGithubDownloaderV3(context.Background(), "https://github.com", "", "", "", "go-xorm", "builder")
		repoName   = "builder-" + time.Now().Format("2006-01-02-15-04-05")
		uploader   = NewGiteaLocalUploader(graceful.GetManager().HammerContext(), user, user.Name, repoName)
	)

	err := migrateRepository(downloader, uploader, base.MigrateOptions{
		CloneAddr:    "https://github.com/go-xorm/builder",
		RepoName:     repoName,
		AuthUsername: "",

		Wiki:         true,
		Issues:       true,
		Milestones:   true,
		Labels:       true,
		Releases:     true,
		Comments:     true,
		PullRequests: true,
		Private:      true,
		Mirror:       false,
	}, nil)
	assert.NoError(t, err)

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{OwnerID: user.ID, Name: repoName}).(*models.Repository)
	assert.True(t, repo.HasWiki())
	assert.EqualValues(t, models.RepositoryReady, repo.Status)

	milestones, err := models.GetMilestones(models.GetMilestonesOption{
		RepoID: repo.ID,
		State:  structs.StateOpen,
	})
	assert.NoError(t, err)
	assert.Len(t, milestones, 1)

	milestones, err = models.GetMilestones(models.GetMilestonesOption{
		RepoID: repo.ID,
		State:  structs.StateClosed,
	})
	assert.NoError(t, err)
	assert.Empty(t, milestones)

	labels, err := models.GetLabelsByRepoID(repo.ID, "", models.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, labels, 11)

	releases, err := models.GetReleasesByRepoID(repo.ID, models.FindReleasesOptions{
		ListOptions: models.ListOptions{
			PageSize: 10,
			Page:     0,
		},
		IncludeTags: true,
	})
	assert.NoError(t, err)
	assert.Len(t, releases, 8)

	releases, err = models.GetReleasesByRepoID(repo.ID, models.FindReleasesOptions{
		ListOptions: models.ListOptions{
			PageSize: 10,
			Page:     0,
		},
		IncludeTags: false,
	})
	assert.NoError(t, err)
	assert.Len(t, releases, 1)

	issues, err := models.Issues(&models.IssuesOptions{
		RepoIDs:  []int64{repo.ID},
		IsPull:   util.OptionalBoolFalse,
		SortType: "oldest",
	})
	assert.NoError(t, err)
	assert.Len(t, issues, 14)
	assert.NoError(t, issues[0].LoadDiscussComments())
	assert.Empty(t, issues[0].Comments)

	pulls, _, err := models.PullRequests(repo.ID, &models.PullRequestsOptions{
		SortType: "oldest",
	})
	assert.NoError(t, err)
	assert.Len(t, pulls, 34)
	assert.NoError(t, pulls[0].LoadIssue())
	assert.NoError(t, pulls[0].Issue.LoadDiscussComments())
	assert.Len(t, pulls[0].Issue.Comments, 2)
}
