// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func getCreateFileOptions() api.CreateFileOptions {
	content := "This is new text"
	contentEncoded := base64.StdEncoding.EncodeToString([]byte(content))
	return api.CreateFileOptions{
		FileOptions: api.FileOptions{
			BranchName:    "master",
			NewBranchName: "master",
			Message:       "Making this new file new/file.txt",
			Author: api.Identity{
				Name:  "Anne Doe",
				Email: "annedoe@example.com",
			},
			Committer: api.Identity{
				Name:  "John Doe",
				Email: "johndoe@example.com",
			},
			Dates: api.CommitDateOptions{
				Author:    time.Unix(946684810, 0),
				Committer: time.Unix(978307190, 0),
			},
		},
		Content: contentEncoded,
	}
}

func getExpectedFileResponseForCreate(commitID, treePath string) *api.FileResponse {
	sha := "a635aa942442ddfdba07468cf9661c08fbdf0ebf"
	encoding := "base64"
	content := "VGhpcyBpcyBuZXcgdGV4dA=="
	selfURL := setting.AppURL + "api/v1/repos/user2/repo1/contents/" + treePath + "?ref=master"
	htmlURL := setting.AppURL + "user2/repo1/src/branch/master/" + treePath
	gitURL := setting.AppURL + "api/v1/repos/user2/repo1/git/blobs/" + sha
	downloadURL := setting.AppURL + "user2/repo1/raw/branch/master/" + treePath
	return &api.FileResponse{
		Content: &api.ContentsResponse{
			Name:        filepath.Base(treePath),
			Path:        treePath,
			SHA:         sha,
			Size:        16,
			Type:        "file",
			Commit:      commitID,
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
		},
		Commit: &api.FileCommitResponse{
			CommitMeta: api.CommitMeta{
				URL: setting.AppURL + "api/v1/repos/user2/repo1/git/commits/" + commitID,
				SHA: commitID,
			},
			HTMLURL: setting.AppURL + "user2/repo1/commit/" + commitID,
			Author: &api.CommitUser{
				Identity: api.Identity{
					Name:  "Anne Doe",
					Email: "annedoe@example.com",
				},
				Date: "2000-01-01T00:00:10Z",
			},
			Committer: &api.CommitUser{
				Identity: api.Identity{
					Name:  "John Doe",
					Email: "johndoe@example.com",
				},
				Date: "2000-12-31T23:59:50Z",
			},
			Message: "Updates README.md\n",
		},
		Verification: &api.PayloadCommitVerification{
			Verified:  false,
			Reason:    "gpg.error.not_signed_commit",
			Signature: "",
			Payload:   "",
		},
	}
}

func BenchmarkAPICreateFileSmall(b *testing.B) {
	onGiteaRunTB(b, func(t testing.TB, u *url.URL) {
		b := t.(*testing.B)
		user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)             // owner of the repo1 & repo16
		repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository) // public repo

		for n := 0; n < b.N; n++ {
			treePath := fmt.Sprintf("update/file%d.txt", n)
			createFileInBranch(user2, repo1, treePath, repo1.DefaultBranch, treePath)
		}
	})
}

func BenchmarkAPICreateFileMedium(b *testing.B) {
	data := make([]byte, 10*1024*1024)

	onGiteaRunTB(b, func(t testing.TB, u *url.URL) {
		b := t.(*testing.B)
		user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)             // owner of the repo1 & repo16
		repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository) // public repo

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			treePath := fmt.Sprintf("update/file%d.txt", n)
			copy(data, treePath)
			createFileInBranch(user2, repo1, treePath, repo1.DefaultBranch, treePath)
		}
	})
}

func TestAPICreateFile(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, u *url.URL) {
		user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)               // owner of the repo1 & repo16
		user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User)               // owner of the repo3, is an org
		user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User)               // owner of neither repos
		repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)   // public repo
		repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)   // public repo
		repo16 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 16}).(*models.Repository) // private repo
		fileID := 0

		// Get user2's token
		session := loginUser(t, user2.Name)
		token2 := getTokenForLoggedInUser(t, session)
		session = emptyTestSession(t)
		// Get user4's token
		session = loginUser(t, user4.Name)
		token4 := getTokenForLoggedInUser(t, session)
		session = emptyTestSession(t)

		// Test creating a file in repo1 which user2 owns, try both with branch and empty branch
		for _, branch := range [...]string{
			"master", // Branch
			"",       // Empty branch
		} {
			createFileOptions := getCreateFileOptions()
			createFileOptions.BranchName = branch
			fileID++
			treePath := fmt.Sprintf("new/file%d.txt", fileID)
			url := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
			req := NewRequestWithJSON(t, "POST", url, &createFileOptions)
			resp := session.MakeRequest(t, req, http.StatusCreated)
			gitRepo, _ := git.OpenRepository(repo1.RepoPath())
			commitID, _ := gitRepo.GetBranchCommitID(createFileOptions.NewBranchName)
			expectedFileResponse := getExpectedFileResponseForCreate(commitID, treePath)
			var fileResponse api.FileResponse
			DecodeJSON(t, resp, &fileResponse)
			assert.EqualValues(t, expectedFileResponse.Content, fileResponse.Content)
			assert.EqualValues(t, expectedFileResponse.Commit.SHA, fileResponse.Commit.SHA)
			assert.EqualValues(t, expectedFileResponse.Commit.HTMLURL, fileResponse.Commit.HTMLURL)
			assert.EqualValues(t, expectedFileResponse.Commit.Author.Email, fileResponse.Commit.Author.Email)
			assert.EqualValues(t, expectedFileResponse.Commit.Author.Name, fileResponse.Commit.Author.Name)
			assert.EqualValues(t, expectedFileResponse.Commit.Author.Date, fileResponse.Commit.Author.Date)
			assert.EqualValues(t, expectedFileResponse.Commit.Committer.Email, fileResponse.Commit.Committer.Email)
			assert.EqualValues(t, expectedFileResponse.Commit.Committer.Name, fileResponse.Commit.Committer.Name)
			assert.EqualValues(t, expectedFileResponse.Commit.Committer.Date, fileResponse.Commit.Committer.Date)
			gitRepo.Close()
		}

		// Test creating a file in a new branch
		createFileOptions := getCreateFileOptions()
		createFileOptions.BranchName = repo1.DefaultBranch
		createFileOptions.NewBranchName = "new_branch"
		fileID++
		treePath := fmt.Sprintf("new/file%d.txt", fileID)
		url := fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req := NewRequestWithJSON(t, "POST", url, &createFileOptions)
		resp := session.MakeRequest(t, req, http.StatusCreated)
		var fileResponse api.FileResponse
		DecodeJSON(t, resp, &fileResponse)
		expectedSHA := "a635aa942442ddfdba07468cf9661c08fbdf0ebf"
		expectedHTMLURL := fmt.Sprintf(setting.AppURL+"user2/repo1/src/branch/new_branch/new/file%d.txt", fileID)
		expectedDownloadURL := fmt.Sprintf(setting.AppURL+"user2/repo1/raw/branch/new_branch/new/file%d.txt", fileID)
		assert.EqualValues(t, expectedSHA, fileResponse.Content.SHA)
		assert.EqualValues(t, expectedHTMLURL, *fileResponse.Content.HTMLURL)
		assert.EqualValues(t, expectedDownloadURL, *fileResponse.Content.DownloadURL)
		assert.EqualValues(t, createFileOptions.Message+"\n", fileResponse.Commit.Message)

		// Test creating a file without a message
		createFileOptions = getCreateFileOptions()
		createFileOptions.Message = ""
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		resp = session.MakeRequest(t, req, http.StatusCreated)
		DecodeJSON(t, resp, &fileResponse)
		expectedMessage := "Add '" + treePath + "'\n"
		assert.EqualValues(t, expectedMessage, fileResponse.Commit.Message)

		// Test trying to create a file that already exists, should fail
		createFileOptions = getCreateFileOptions()
		treePath = "README.md"
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token2)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		resp = session.MakeRequest(t, req, http.StatusUnprocessableEntity)
		expectedAPIError := context.APIError{
			Message: "repository file already exists [path: " + treePath + "]",
			URL:     setting.API.SwaggerURL,
		}
		var apiError context.APIError
		DecodeJSON(t, resp, &apiError)
		assert.Equal(t, expectedAPIError, apiError)

		// Test creating a file in repo1 by user4 who does not have write access
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token4)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		session.MakeRequest(t, req, http.StatusNotFound)

		// Tests a repo with no token given so will fail
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user2.Name, repo16.Name, treePath)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		session.MakeRequest(t, req, http.StatusNotFound)

		// Test using access token for a private repo that the user of the token owns
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo16.Name, treePath, token2)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		session.MakeRequest(t, req, http.StatusCreated)

		// Test using org repo "user3/repo3" where user2 is a collaborator
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user3.Name, repo3.Name, treePath, token2)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		session.MakeRequest(t, req, http.StatusCreated)

		// Test using org repo "user3/repo3" with no user token
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s", user3.Name, repo3.Name, treePath)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		session.MakeRequest(t, req, http.StatusNotFound)

		// Test using repo "user2/repo1" where user4 is a NOT collaborator
		createFileOptions = getCreateFileOptions()
		fileID++
		treePath = fmt.Sprintf("new/file%d.txt", fileID)
		url = fmt.Sprintf("/api/v1/repos/%s/%s/contents/%s?token=%s", user2.Name, repo1.Name, treePath, token4)
		req = NewRequestWithJSON(t, "POST", url, &createFileOptions)
		session.MakeRequest(t, req, http.StatusForbidden)
	})
}
