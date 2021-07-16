// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func getExpectedContentsResponseForContents(ref, refType string) *api.ContentsResponse {
	treePath := "README.md"
	sha := "4b4851ad51df6a7d9f25c979345979eaeb5b349f"
	encoding := "base64"
	content := "IyByZXBvMQoKRGVzY3JpcHRpb24gZm9yIHJlcG8x"
	selfURL := setting.AppURL + "api/v1/repos/user2/repo1/contents/" + treePath + "?ref=" + ref
	htmlURL := setting.AppURL + "user2/repo1/src/" + refType + "/" + ref + "/" + treePath
	gitURL := setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/" + sha
	downloadURL := setting.AppURL + "user2/repo1/raw/" + refType + "/" + ref + "/" + treePath
	commit := "65f1bf27bc3bf70f64657658635e66094edbcb4d"
	return &api.ContentsResponse{
		Name:        treePath,
		Path:        treePath,
		SHA:         sha,
		Type:        "file",
		Size:        30,
		Commit:      commit,
		Encoding:    &encoding,
		Content:     &content,
		URL:         &selfURL,
		HTMLURL:     &htmlURL,
		GitURL:      &gitURL,
		DownloadURL: &downloadURL,
		Links: &api.FileLinksResponse{
			Self:    &selfURL,
			GitURL:  &gitURL,
			HTMLURL: &htmlURL,
		},
	}
}

func TestAPIGetContents(t *testing.T) {
	onGiteaRun(t, testAPIGetContents)
}

func testAPIGetContents(t *testing.T, u *url.URL) {
	/*** SETUP ***/
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)               // owner of the repo1 & repo16
	user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)               // owner of the repo3, is an org
	user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)               // owner of neither repos
	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)   // public repo
	repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)   // public repo
	repo16 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 16}).(*models.Repository) // private repo
	treePath := "README.md"

	// Get user2's token
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t)
	// Get user4's token
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t)

	// Make a new branch in repo1
	newBranch := "test_branch"
	err := repo_module.CreateNewBranch(user2, repo1, repo1.DefaultBranch, newBranch)
	assert.NoError(t, err)
	// Get the commit ID of the default branch
	gitRepo, err := git.OpenRepository(repo1.RepoPath())
	assert.NoError(t, err)
	defer gitRepo.Close()

	commitID, err := gitRepo.GetBranchCommitID(repo1.DefaultBranch)
	assert.NoError(t, err)
	// Make a new tag in repo1
	newTag := "test_tag"
	err = gitRepo.CreateTag(newTag, commitID)
	assert.NoError(t, err)
	/*** END SETUP ***/

	// ref is default ref
	ref := repo1.DefaultBranch
	refType := "branch"
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var contentsResponse api.ContentsResponse
	DecodeJSON(t, resp, &contentsResponse)
	assert.NotNil(t, contentsResponse)
	expectedContentsResponse := getExpectedContentsResponseForContents(ref, refType)
	assert.EqualValues(t, *expectedContentsResponse, contentsResponse)

	// No ref
	refType = "branch"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s", user2.Name, repo1.Name, treePath)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsResponse)
	assert.NotNil(t, contentsResponse)
	expectedContentsResponse = getExpectedContentsResponseForContents(repo1.DefaultBranch, refType)
	assert.EqualValues(t, *expectedContentsResponse, contentsResponse)

	// ref is the branch we created above  in setup
	ref = newBranch
	refType = "branch"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsResponse)
	assert.NotNil(t, contentsResponse)
	expectedContentsResponse = getExpectedContentsResponseForContents(ref, refType)
	assert.EqualValues(t, *expectedContentsResponse, contentsResponse)

	// ref is the new tag we created above in setup
	ref = newTag
	refType = "tag"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsResponse)
	assert.NotNil(t, contentsResponse)
	expectedContentsResponse = getExpectedContentsResponseForContents(ref, refType)
	assert.EqualValues(t, *expectedContentsResponse, contentsResponse)

	// ref is a commit
	ref = commitID
	refType = "commit"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &contentsResponse)
	assert.NotNil(t, contentsResponse)
	expectedContentsResponse = getExpectedContentsResponseForContents(ref, refType)
	assert.EqualValues(t, *expectedContentsResponse, contentsResponse)

	// Test file contents a file with a bad ref
	ref = "badref"
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?ref=%s", user2.Name, repo1.Name, treePath, ref)
	resp = session.MakeRequest(t, req, http.StatusNotFound)

	// Test accessing private ref with user token that does not have access - should fail
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token4)
	session.MakeRequest(t, req, http.StatusNotFound)

	// Test access private ref of owner of token
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/readme.md?token=%s", user2.Name, repo16.Name, token2)
	session.MakeRequest(t, req, http.StatusOK)

	// Test access of org user3 private repo file by owner user2
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/contents/%s?token=%s", user3.Name, repo3.Name, treePath, token2)
	session.MakeRequest(t, req, http.StatusOK)
}
