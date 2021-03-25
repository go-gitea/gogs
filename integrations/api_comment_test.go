// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIListRepoComments(t *testing.T) {
	defer prepareTestEnv(t)()

	comment := models.AssertExistsAndLoadBean(t, &models.Comment{},
		models.Cond("type = ?", models.CommentTypeComment)).(*models.Comment)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: comment.IssueID}).(*models.Issue)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issue.RepoID}).(*models.Repository)
	repoOwner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	link, _ := url.Parse(fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments", repoOwner.Name, repo.Name))
	req := NewRequest(t, "GET", link.String())
	resp := session.MakeRequest(t, req, http.StatusOK)

	var apiComments []*api.Comment
	DecodeJSON(t, resp, &apiComments)
	assert.Len(t, apiComments, 2)
	for _, apiComment := range apiComments {
		c := &models.Comment{ID: apiComment.ID}
		models.AssertExistsAndLoadBean(t, c,
			models.Cond("type = ?", models.CommentTypeComment))
		models.AssertExistsAndLoadBean(t, &models.Issue{ID: c.IssueID, RepoID: repo.ID})
	}

	// test before and since filters
	query := url.Values{}
	before := "2000-01-01T00:00:11+00:00" // unix: 946684811
	since := "2000-01-01T00:00:12+00:00"  // unix: 946684812
	query.Add("before", before)
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiComments)
	assert.Len(t, apiComments, 1)
	assert.EqualValues(t, 2, apiComments[0].ID)

	query.Del("before")
	query.Add("since", since)
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiComments)
	assert.Len(t, apiComments, 1)
	assert.EqualValues(t, 3, apiComments[0].ID)
}

func TestAPIListIssueComments(t *testing.T) {
	defer prepareTestEnv(t)()

	comment := models.AssertExistsAndLoadBean(t, &models.Comment{},
		models.Cond("type = ?", models.CommentTypeComment)).(*models.Comment)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: comment.IssueID}).(*models.Issue)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issue.RepoID}).(*models.Repository)
	repoOwner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/%d/comments",
		repoOwner.Name, repo.Name, issue.Index)
	resp := session.MakeRequest(t, req, http.StatusOK)

	var comments []*api.Comment
	DecodeJSON(t, resp, &comments)
	expectedCount := models.GetCount(t, &models.Comment{IssueID: issue.ID},
		models.Cond("type = ?", models.CommentTypeComment))
	assert.EqualValues(t, expectedCount, len(comments))
}

func TestAPICreateComment(t *testing.T) {
	defer prepareTestEnv(t)()
	const commentBody = "Comment body"

	issue := models.AssertExistsAndLoadBean(t, &models.Issue{}).(*models.Issue)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issue.RepoID}).(*models.Repository)
	repoOwner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/comments?token=%s",
		repoOwner.Name, repo.Name, issue.Index, token)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"body": commentBody,
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)

	var updatedComment api.Comment
	DecodeJSON(t, resp, &updatedComment)
	assert.EqualValues(t, commentBody, updatedComment.Body)
	models.AssertExistsAndLoadBean(t, &models.Comment{ID: updatedComment.ID, IssueID: issue.ID, Content: commentBody})
}

func TestAPIGetComment(t *testing.T) {
	defer prepareTestEnv(t)()

	comment := models.AssertExistsAndLoadBean(t, &models.Comment{ID: 2}).(*models.Comment)
	assert.NoError(t, comment.LoadIssue())
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: comment.Issue.RepoID}).(*models.Repository)
	repoOwner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/comments/%d", repoOwner.Name, repo.Name, comment.ID)
	resp := session.MakeRequest(t, req, http.StatusOK)
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues/comments/%d?token=%s", repoOwner.Name, repo.Name, comment.ID, token)
	resp = session.MakeRequest(t, req, http.StatusOK)

	var apiComment api.Comment
	DecodeJSON(t, resp, &apiComment)

	assert.NoError(t, comment.LoadPoster())
	expect := convert.ToComment(comment)

	assert.Equal(t, expect.ID, apiComment.ID)
	assert.Equal(t, expect.Poster.FullName, apiComment.Poster.FullName)
	assert.Equal(t, expect.Body, apiComment.Body)
	assert.Equal(t, expect.Created.Unix(), apiComment.Created.Unix())
}

func TestAPIEditComment(t *testing.T) {
	defer prepareTestEnv(t)()
	const newCommentBody = "This is the new comment body"

	comment := models.AssertExistsAndLoadBean(t, &models.Comment{},
		models.Cond("type = ?", models.CommentTypeComment)).(*models.Comment)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: comment.IssueID}).(*models.Issue)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issue.RepoID}).(*models.Repository)
	repoOwner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments/%d?token=%s",
		repoOwner.Name, repo.Name, comment.ID, token)
	req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
		"body": newCommentBody,
	})
	resp := session.MakeRequest(t, req, http.StatusOK)

	var updatedComment api.Comment
	DecodeJSON(t, resp, &updatedComment)
	assert.EqualValues(t, comment.ID, updatedComment.ID)
	assert.EqualValues(t, newCommentBody, updatedComment.Body)
	models.AssertExistsAndLoadBean(t, &models.Comment{ID: comment.ID, IssueID: issue.ID, Content: newCommentBody})
}

func TestAPIDeleteComment(t *testing.T) {
	defer prepareTestEnv(t)()

	comment := models.AssertExistsAndLoadBean(t, &models.Comment{},
		models.Cond("type = ?", models.CommentTypeComment)).(*models.Comment)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: comment.IssueID}).(*models.Issue)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issue.RepoID}).(*models.Repository)
	repoOwner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, repoOwner.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/issues/comments/%d?token=%s",
		repoOwner.Name, repo.Name, comment.ID, token)
	session.MakeRequest(t, req, http.StatusNoContent)

	models.AssertNotExistsBean(t, &models.Comment{ID: comment.ID})
}
