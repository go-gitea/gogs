// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package release

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))
}

func TestRelease_Create(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repoPath := models.RepoPath(user.Name, repo.Name)

	gitRepo, err := git.OpenRepository(repoPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1",
		Target:       "master",
		Title:        "v0.1 is released",
		Note:         "v0.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))

	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1.1",
		Target:       "65f1bf27bc3bf70f64657658635e66094edbcb4d",
		Title:        "v0.1.1 is released",
		Note:         "v0.1.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))

	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1.2",
		Target:       "65f1bf2",
		Title:        "v0.1.2 is released",
		Note:         "v0.1.2 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))

	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1.3",
		Target:       "65f1bf2",
		Title:        "v0.1.3 is released",
		Note:         "v0.1.3 is released",
		IsDraft:      true,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))

	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1.4",
		Target:       "65f1bf2",
		Title:        "v0.1.4 is released",
		Note:         "v0.1.4 is released",
		IsDraft:      false,
		IsPrerelease: true,
		IsTag:        false,
	}, nil, ""))

	attach, err := models.NewAttachment(&models.Attachment{
		UploaderID: user.ID,
		Name:       "test.txt",
	}, []byte{}, strings.NewReader("testtest"))
	assert.NoError(t, err)

	var release = models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v0.1.5",
		Target:       "65f1bf2",
		Title:        "v0.1.5 is released",
		Note:         "v0.1.5 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        true,
	}
	assert.NoError(t, CreateRelease(gitRepo, &release, []string{attach.UUID}, "test"))
}

func TestRelease_Update(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repoPath := models.RepoPath(user.Name, repo.Name)

	gitRepo, err := git.OpenRepository(repoPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	// Test a changed release
	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v1.1.1",
		Target:       "master",
		Title:        "v1.1.1 is released",
		Note:         "v1.1.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))
	release, err := models.GetRelease(repo.ID, "v1.1.1")
	assert.NoError(t, err)
	releaseCreatedUnix := release.CreatedUnix
	time.Sleep(2 * time.Second) // sleep 2 seconds to ensure a different timestamp
	release.Note = "Changed note"
	assert.NoError(t, UpdateRelease(user, gitRepo, release, nil, nil, nil))
	release, err = models.GetReleaseByID(release.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))

	// Test a changed draft
	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v1.2.1",
		Target:       "65f1bf2",
		Title:        "v1.2.1 is draft",
		Note:         "v1.2.1 is draft",
		IsDraft:      true,
		IsPrerelease: false,
		IsTag:        false,
	}, nil, ""))
	release, err = models.GetRelease(repo.ID, "v1.2.1")
	assert.NoError(t, err)
	releaseCreatedUnix = release.CreatedUnix
	time.Sleep(2 * time.Second) // sleep 2 seconds to ensure a different timestamp
	release.Title = "Changed title"
	assert.NoError(t, UpdateRelease(user, gitRepo, release, nil, nil, nil))
	release, err = models.GetReleaseByID(release.ID)
	assert.NoError(t, err)
	assert.Less(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))

	// Test a changed pre-release
	assert.NoError(t, CreateRelease(gitRepo, &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v1.3.1",
		Target:       "65f1bf2",
		Title:        "v1.3.1 is pre-released",
		Note:         "v1.3.1 is pre-released",
		IsDraft:      false,
		IsPrerelease: true,
		IsTag:        false,
	}, nil, ""))
	release, err = models.GetRelease(repo.ID, "v1.3.1")
	assert.NoError(t, err)
	releaseCreatedUnix = release.CreatedUnix
	time.Sleep(2 * time.Second) // sleep 2 seconds to ensure a different timestamp
	release.Title = "Changed title"
	release.Note = "Changed note"
	assert.NoError(t, UpdateRelease(user, gitRepo, release, nil, nil, nil))
	release, err = models.GetReleaseByID(release.ID)
	assert.NoError(t, err)
	assert.Equal(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))

	// Test create release
	release = &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v1.1.2",
		Target:       "master",
		Title:        "v1.1.2 is released",
		Note:         "v1.1.2 is released",
		IsDraft:      true,
		IsPrerelease: false,
		IsTag:        false,
	}
	assert.NoError(t, CreateRelease(gitRepo, release, nil, ""))
	assert.Greater(t, release.ID, int64(0))

	release.IsDraft = false
	tagName := release.TagName

	assert.NoError(t, UpdateRelease(user, gitRepo, release, nil, nil, nil))
	release, err = models.GetReleaseByID(release.ID)
	assert.NoError(t, err)
	assert.Equal(t, tagName, release.TagName)

	// Add new attachments
	attach, err := models.NewAttachment(&models.Attachment{
		UploaderID: user.ID,
		Name:       "test.txt",
	}, []byte{}, strings.NewReader("testtest"))
	assert.NoError(t, err)

	assert.NoError(t, UpdateRelease(user, gitRepo, release, []string{attach.UUID}, nil, nil))
	assert.NoError(t, models.GetReleaseAttachments(release))
	assert.Len(t, release.Attachments, 1)
	assert.EqualValues(t, attach.UUID, release.Attachments[0].UUID)
	assert.EqualValues(t, release.ID, release.Attachments[0].ReleaseID)
	assert.EqualValues(t, attach.Name, release.Attachments[0].Name)

	// update the attachment name
	assert.NoError(t, UpdateRelease(user, gitRepo, release, nil, nil, map[string]string{
		attach.UUID: "test2.txt",
	}))
	release.Attachments = nil
	assert.NoError(t, models.GetReleaseAttachments(release))
	assert.Len(t, release.Attachments, 1)
	assert.EqualValues(t, attach.UUID, release.Attachments[0].UUID)
	assert.EqualValues(t, release.ID, release.Attachments[0].ReleaseID)
	assert.EqualValues(t, "test2.txt", release.Attachments[0].Name)

	// delete the attachment
	assert.NoError(t, UpdateRelease(user, gitRepo, release, nil, []string{attach.UUID}, nil))
	release.Attachments = nil
	assert.NoError(t, models.GetReleaseAttachments(release))
	assert.Empty(t, release.Attachments)
}

func TestRelease_createTag(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	repoPath := models.RepoPath(user.Name, repo.Name)

	gitRepo, err := git.OpenRepository(repoPath)
	assert.NoError(t, err)
	defer gitRepo.Close()

	// Test a changed release
	release := &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v2.1.1",
		Target:       "master",
		Title:        "v2.1.1 is released",
		Note:         "v2.1.1 is released",
		IsDraft:      false,
		IsPrerelease: false,
		IsTag:        false,
	}
	_, err = createTag(gitRepo, release, "")
	assert.NoError(t, err)
	assert.NotEmpty(t, release.CreatedUnix)
	releaseCreatedUnix := release.CreatedUnix
	time.Sleep(2 * time.Second) // sleep 2 seconds to ensure a different timestamp
	release.Note = "Changed note"
	_, err = createTag(gitRepo, release, "")
	assert.NoError(t, err)
	assert.Equal(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))

	// Test a changed draft
	release = &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v2.2.1",
		Target:       "65f1bf2",
		Title:        "v2.2.1 is draft",
		Note:         "v2.2.1 is draft",
		IsDraft:      true,
		IsPrerelease: false,
		IsTag:        false,
	}
	_, err = createTag(gitRepo, release, "")
	assert.NoError(t, err)
	releaseCreatedUnix = release.CreatedUnix
	time.Sleep(2 * time.Second) // sleep 2 seconds to ensure a different timestamp
	release.Title = "Changed title"
	_, err = createTag(gitRepo, release, "")
	assert.NoError(t, err)
	assert.Less(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))

	// Test a changed pre-release
	release = &models.Release{
		RepoID:       repo.ID,
		Repo:         repo,
		PublisherID:  user.ID,
		Publisher:    user,
		TagName:      "v2.3.1",
		Target:       "65f1bf2",
		Title:        "v2.3.1 is pre-released",
		Note:         "v2.3.1 is pre-released",
		IsDraft:      false,
		IsPrerelease: true,
		IsTag:        false,
	}
	_, err = createTag(gitRepo, release, "")
	assert.NoError(t, err)
	releaseCreatedUnix = release.CreatedUnix
	time.Sleep(2 * time.Second) // sleep 2 seconds to ensure a different timestamp
	release.Title = "Changed title"
	release.Note = "Changed note"
	_, err = createTag(gitRepo, release, "")
	assert.NoError(t, err)
	assert.Equal(t, int64(releaseCreatedUnix), int64(release.CreatedUnix))
}

func TestCreateNewTag(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	user := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)

	assert.NoError(t, CreateNewTag(user, repo, "master", "v2.0",
		"v2.0 is released \n\n BUGFIX: .... \n\n 123"))
}
