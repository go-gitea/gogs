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
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIListIssues(t *testing.T) {
	defer prepareTestEnv(t)()

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues?state=all&token=%s",
		owner.Name, repo.Name, token)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiIssues []*api.Issue
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, models.GetCount(t, &models.Issue{RepoID: repo.ID}))
	for _, apiIssue := range apiIssues {
		issue, err := models.GetIssueByID(apiIssue.ID)
		assert.NoError(t, err)
		assert.NoError(t, issue.LoadAttributes())
		assert.Equal(t, repo.ID, issue.RepoID)
		assert.Equal(t, issue.PosterID, apiIssue.Poster.ID)
		assert.Equal(t, int64(issue.UpdatedUnix), apiIssue.Updated.Unix())
		summary := issue.Reactions.Summary()
		for i, apiR := range apiIssue.Reactions {
			assert.Equal(t, summary[i].Type, apiR.Type)
			assert.EqualValues(t, summary[i].Users, apiR.Users)
		}
	}

	// test milestone filter
	req = NewRequestf(t, "GET", "/api/v1/repos/%s/%s/issues?state=all&type=all&milestones=ignore,milestone1,3,4&token=%s",
		owner.Name, repo.Name, token)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	if assert.Len(t, apiIssues, 2) {
		assert.EqualValues(t, 3, apiIssues[0].Milestone.ID)
		assert.EqualValues(t, 1, apiIssues[1].Milestone.ID)
	}

}

func TestAPICreateIssue(t *testing.T) {
	defer prepareTestEnv(t)()
	const body, title = "apiTestBody", "apiTestTitle"

	repoBefore := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repoBefore.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues?state=all&token=%s", owner.Name, repoBefore.Name, token)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateIssueOption{
		Body:     body,
		Title:    title,
		Assignee: owner.Name,
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var apiIssue api.Issue
	DecodeJSON(t, resp, &apiIssue)
	assert.Equal(t, apiIssue.Body, body)
	assert.Equal(t, apiIssue.Title, title)

	models.AssertExistsAndLoadBean(t, &models.Issue{
		RepoID:     repoBefore.ID,
		AssigneeID: owner.ID,
		Content:    body,
		Title:      title,
	})

	repoAfter := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	assert.Equal(t, repoBefore.NumIssues+1, repoAfter.NumIssues)
	assert.Equal(t, repoBefore.NumClosedIssues, repoAfter.NumClosedIssues)
}

func TestAPIEditIssue(t *testing.T) {
	defer prepareTestEnv(t)()

	issueBefore := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 10}).(*models.Issue)
	repoBefore := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issueBefore.RepoID}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repoBefore.OwnerID}).(*models.User)
	assert.NoError(t, issueBefore.LoadAttributes())
	assert.Equal(t, int64(1019307200), int64(issueBefore.DeadlineUnix))
	assert.Equal(t, api.StateOpen, issueBefore.State())

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	// update values of issue
	issueState := "closed"
	removeDeadline := true
	milestone := int64(4)
	body := "new content!"
	title := "new title from api set"

	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d?token=%s", owner.Name, repoBefore.Name, issueBefore.Index, token)
	req := NewRequestWithJSON(t, "PATCH", urlStr, api.EditIssueOption{
		State:          &issueState,
		RemoveDeadline: &removeDeadline,
		Milestone:      &milestone,
		Body:           &body,
		Title:          title,

		// ToDo change more
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var apiIssue api.Issue
	DecodeJSON(t, resp, &apiIssue)

	issueAfter := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 10}).(*models.Issue)
	repoAfter := models.AssertExistsAndLoadBean(t, &models.Repository{ID: issueBefore.RepoID}).(*models.Repository)

	// check deleted user
	assert.Equal(t, int64(500), issueAfter.PosterID)
	assert.NoError(t, issueAfter.LoadAttributes())
	assert.Equal(t, int64(-1), issueAfter.PosterID)
	assert.Equal(t, int64(-1), issueBefore.PosterID)
	assert.Equal(t, int64(-1), apiIssue.Poster.ID)

	// check repo change
	assert.Equal(t, repoBefore.NumClosedIssues+1, repoAfter.NumClosedIssues)

	// API response
	assert.Equal(t, api.StateClosed, apiIssue.State)
	assert.Equal(t, milestone, apiIssue.Milestone.ID)
	assert.Equal(t, body, apiIssue.Body)
	assert.True(t, apiIssue.Deadline == nil)
	assert.Equal(t, title, apiIssue.Title)

	// in database
	assert.Equal(t, api.StateClosed, issueAfter.State())
	assert.Equal(t, milestone, issueAfter.MilestoneID)
	assert.Equal(t, int64(0), int64(issueAfter.DeadlineUnix))
	assert.Equal(t, body, issueAfter.Content)
	assert.Equal(t, title, issueAfter.Title)
}

func TestAPISearchIssues(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session)

	link, _ := url.Parse("/api/v1/repos/issues/search")
	req := NewRequest(t, "GET", link.String())
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiIssues []*api.Issue
	DecodeJSON(t, resp, &apiIssues)

	assert.Len(t, apiIssues, 9)

	query := url.Values{}
	query.Add("token", token)
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 9)

	query.Add("state", "closed")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)

	query.Set("state", "all")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 10) //there are more but 10 is page item limit

	query.Add("page", "2")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 1)
}

func TestAPISearchIssuesWithLabels(t *testing.T) {
	defer prepareTestEnv(t)()

	session := loginUser(t, "user1")
	token := getTokenForLoggedInUser(t, session)

	link, _ := url.Parse("/api/v1/repos/issues/search")
	req := NewRequest(t, "GET", link.String())
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiIssues []*api.Issue
	DecodeJSON(t, resp, &apiIssues)

	assert.Len(t, apiIssues, 9)

	query := url.Values{}
	query.Add("token", token)
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 9)

	query.Add("labels", "label1")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)

	// multiple labels
	query.Set("labels", "label1,label2")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)

	// an org label
	query.Set("labels", "orglabel4")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 1)

	// org and repo label
	query.Set("labels", "label2,orglabel4")
	query.Add("state", "all")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)

	// org and repo label which share the same issue
	query.Set("labels", "label1,orglabel4")
	link.RawQuery = query.Encode()
	req = NewRequest(t, "GET", link.String())
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiIssues)
	assert.Len(t, apiIssues, 2)
}
