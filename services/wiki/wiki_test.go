// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package wiki

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	models.MainTest(m, filepath.Join("..", ".."))
}

func TestWikiNameToSubURL(t *testing.T) {
	type test struct {
		Expected string
		WikiName string
	}
	for _, test := range []test{
		{"wiki-name", "wiki name"},
		{"wiki-name", "wiki-name"},
		{"name-with%2Fslash", "name with/slash"},
		{"name-with%25percent", "name with%percent"},
	} {
		assert.Equal(t, test.Expected, NameToSubURL(test.WikiName))
	}
}

func TestNormalizeWikiName(t *testing.T) {
	type test struct {
		Expected string
		WikiName string
	}
	for _, test := range []test{
		{"wiki name", "wiki name"},
		{"wiki name", "wiki-name"},
		{"name with/slash", "name with/slash"},
		{"name with%percent", "name-with%percent"},
		{"%2F", "%2F"},
	} {
		assert.Equal(t, test.Expected, NormalizeWikiName(test.WikiName))
	}
}

func TestWikiNameToFilename(t *testing.T) {
	type test struct {
		Expected string
		WikiName string
	}
	for _, test := range []test{
		{"wiki-name.md", "wiki name"},
		{"wiki-name.md", "wiki-name"},
		{"name-with%2Fslash.md", "name with/slash"},
		{"name-with%25percent.md", "name with%percent"},
	} {
		assert.Equal(t, test.Expected, NameToFilename(test.WikiName))
	}
}

func TestWikiFilenameToName(t *testing.T) {
	type test struct {
		Expected string
		Filename string
	}
	for _, test := range []test{
		{"hello world", "hello-world.md"},
		{"symbols/?*", "symbols%2F%3F%2A.md"},
	} {
		name, err := FilenameToName(test.Filename)
		assert.NoError(t, err)
		assert.Equal(t, test.Expected, name)
	}
	for _, badFilename := range []string{
		"nofileextension",
		"wrongfileextension.txt",
	} {
		_, err := FilenameToName(badFilename)
		assert.Error(t, err)
		assert.True(t, models.IsErrWikiInvalidFileName(err))
	}
	_, err := FilenameToName("badescaping%%.md")
	assert.Error(t, err)
	assert.False(t, models.IsErrWikiInvalidFileName(err))
}

func TestWikiNameToFilenameToName(t *testing.T) {
	// converting from wiki name to filename, then back to wiki name should
	// return the original (normalized) name
	for _, name := range []string{
		"wiki-name",
		"wiki name",
		"wiki name with/slash",
		"$$$%%%^^&&!@#$(),.<>",
	} {
		filename := NameToFilename(name)
		resultName, err := FilenameToName(filename)
		assert.NoError(t, err)
		assert.Equal(t, NormalizeWikiName(name), resultName)
	}
}

func TestRepository_InitWiki(t *testing.T) {
	models.PrepareTestEnv(t)
	// repo1 already has a wiki
	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	assert.NoError(t, InitWiki(repo1))

	// repo2 does not already have a wiki
	repo2 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	assert.NoError(t, InitWiki(repo2))
	assert.True(t, repo2.HasWiki())
}

func TestRepository_AddWikiPage(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	const wikiContent = "This is the wiki content"
	const commitMsg = "Commit message"
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	doer := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	for _, wikiName := range []string{
		"Another page",
		"Here's a <tag> and a/slash",
	} {
		wikiName := wikiName
		t.Run("test wiki exist: "+wikiName, func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, AddWikiPage(doer, repo, wikiName, wikiContent, commitMsg))
			// Now need to show that the page has been added:
			gitRepo, err := git.OpenRepository(repo.WikiPath())
			assert.NoError(t, err)
			defer gitRepo.Close()
			masterTree, err := gitRepo.GetTree("master")
			assert.NoError(t, err)
			wikiPath := NameToFilename(wikiName)
			entry, err := masterTree.GetTreeEntryByPath(wikiPath)
			assert.NoError(t, err)
			assert.Equal(t, wikiPath, entry.Name(), "%s not added correctly", wikiName)
		})
	}

	t.Run("check wiki already exist", func(t *testing.T) {
		t.Parallel()
		// test for already-existing wiki name
		err := AddWikiPage(doer, repo, "Home", wikiContent, commitMsg)
		assert.Error(t, err)
		assert.True(t, models.IsErrWikiAlreadyExist(err))
	})

	t.Run("check wiki reserved name", func(t *testing.T) {
		t.Parallel()
		// test for reserved wiki name
		err := AddWikiPage(doer, repo, "_edit", wikiContent, commitMsg)
		assert.Error(t, err)
		assert.True(t, models.IsErrWikiReservedName(err))
	})
}

func TestRepository_EditWikiPage(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	const newWikiContent = "This is the new content"
	const commitMsg = "Commit message"
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	doer := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	for _, newWikiName := range []string{
		"Home", // same name as before
		"New home",
		"New/name/with/slashes",
	} {
		models.PrepareTestEnv(t)
		assert.NoError(t, EditWikiPage(doer, repo, "Home", newWikiName, newWikiContent, commitMsg))

		// Now need to show that the page has been added:
		gitRepo, err := git.OpenRepository(repo.WikiPath())
		assert.NoError(t, err)
		masterTree, err := gitRepo.GetTree("master")
		assert.NoError(t, err)
		wikiPath := NameToFilename(newWikiName)
		entry, err := masterTree.GetTreeEntryByPath(wikiPath)
		assert.NoError(t, err)
		assert.Equal(t, wikiPath, entry.Name(), "%s not editted correctly", newWikiName)

		if newWikiName != "Home" {
			_, err := masterTree.GetTreeEntryByPath("Home.md")
			assert.Error(t, err)
		}
		gitRepo.Close()
	}
}

func TestRepository_DeleteWikiPage(t *testing.T) {
	models.PrepareTestEnv(t)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	doer := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	assert.NoError(t, DeleteWikiPage(doer, repo, "Home"))

	// Now need to show that the page has been added:
	gitRepo, err := git.OpenRepository(repo.WikiPath())
	assert.NoError(t, err)
	defer gitRepo.Close()
	masterTree, err := gitRepo.GetTree("master")
	assert.NoError(t, err)
	wikiPath := NameToFilename("Home")
	_, err = masterTree.GetTreeEntryByPath(wikiPath)
	assert.Error(t, err)
}
