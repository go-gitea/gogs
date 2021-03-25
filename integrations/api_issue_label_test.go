// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIModifyLabels(t *testing.T) {
	assert.NoError(t, models.LoadFixtures())

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)
	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/labels?token=%s", owner.Name, repo.Name, token)

	// CreateLabel
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:        "TestL 1",
		Color:       "abcdef",
		Description: "test label",
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	apiLabel := new(api.Label)
	DecodeJSON(t, resp, &apiLabel)
	dbLabel := models.AssertExistsAndLoadBean(t, &models.Label{ID: apiLabel.ID, RepoID: repo.ID}).(*models.Label)
	assert.EqualValues(t, dbLabel.Name, apiLabel.Name)
	assert.EqualValues(t, strings.TrimLeft(dbLabel.Color, "#"), apiLabel.Color)

	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:        "TestL 2",
		Color:       "#123456",
		Description: "jet another test label",
	})
	session.MakeRequest(t, req, http.StatusCreated)
	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:  "WrongTestL",
		Color: "#12345g",
	})
	session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	// ListLabels
	req = NewRequest(t, "GET", urlStr)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	assert.Len(t, apiLabels, 2)

	// GetLabel
	singleURLStr := fmt.Sprintf("/api/v1/repos/%s/%s/labels/%d?token=%s", owner.Name, repo.Name, dbLabel.ID, token)
	req = NewRequest(t, "GET", singleURLStr)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiLabel)
	assert.EqualValues(t, strings.TrimLeft(dbLabel.Color, "#"), apiLabel.Color)

	// EditLabel
	newName := "LabelNewName"
	newColor := "09876a"
	newColorWrong := "09g76a"
	req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
		Name:  &newName,
		Color: &newColor,
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiLabel)
	assert.EqualValues(t, newColor, apiLabel.Color)
	req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
		Color: &newColorWrong,
	})
	session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	// DeleteLabel
	req = NewRequest(t, "DELETE", singleURLStr)
	resp = session.MakeRequest(t, req, http.StatusNoContent)
}

func TestAPIAddIssueLabels(t *testing.T) {
	assert.NoError(t, models.LoadFixtures())

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{RepoID: repo.ID}).(*models.Issue)
	_ = models.AssertExistsAndLoadBean(t, &models.Label{RepoID: repo.ID, ID: 2}).(*models.Label)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels?token=%s",
		repo.OwnerName, repo.Name, issue.Index, token)
	req := NewRequestWithJSON(t, "POST", urlStr, &api.IssueLabelsOption{
		Labels: []int64{1, 2},
	})
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	assert.Len(t, apiLabels, models.GetCount(t, &models.IssueLabel{IssueID: issue.ID}))

	models.AssertExistsAndLoadBean(t, &models.IssueLabel{IssueID: issue.ID, LabelID: 2})
}

func TestAPIReplaceIssueLabels(t *testing.T) {
	assert.NoError(t, models.LoadFixtures())

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{RepoID: repo.ID}).(*models.Issue)
	label := models.AssertExistsAndLoadBean(t, &models.Label{RepoID: repo.ID}).(*models.Label)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/labels?token=%s",
		owner.Name, repo.Name, issue.Index, token)
	req := NewRequestWithJSON(t, "PUT", urlStr, &api.IssueLabelsOption{
		Labels: []int64{label.ID},
	})
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	if assert.Len(t, apiLabels, 1) {
		assert.EqualValues(t, label.ID, apiLabels[0].ID)
	}

	models.AssertCount(t, &models.IssueLabel{IssueID: issue.ID}, 1)
	models.AssertExistsAndLoadBean(t, &models.IssueLabel{IssueID: issue.ID, LabelID: label.ID})
}

func TestAPIModifyOrgLabels(t *testing.T) {
	assert.NoError(t, models.LoadFixtures())

	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: repo.OwnerID}).(*models.User)
	user := "user1"
	session := loginUser(t, user)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/orgs/%s/labels?token=%s", owner.Name, token)

	// CreateLabel
	req := NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:        "TestL 1",
		Color:       "abcdef",
		Description: "test label",
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	apiLabel := new(api.Label)
	DecodeJSON(t, resp, &apiLabel)
	dbLabel := models.AssertExistsAndLoadBean(t, &models.Label{ID: apiLabel.ID, OrgID: owner.ID}).(*models.Label)
	assert.EqualValues(t, dbLabel.Name, apiLabel.Name)
	assert.EqualValues(t, strings.TrimLeft(dbLabel.Color, "#"), apiLabel.Color)

	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:        "TestL 2",
		Color:       "#123456",
		Description: "jet another test label",
	})
	session.MakeRequest(t, req, http.StatusCreated)
	req = NewRequestWithJSON(t, "POST", urlStr, &api.CreateLabelOption{
		Name:  "WrongTestL",
		Color: "#12345g",
	})
	session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	// ListLabels
	req = NewRequest(t, "GET", urlStr)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var apiLabels []*api.Label
	DecodeJSON(t, resp, &apiLabels)
	assert.Len(t, apiLabels, 4)

	// GetLabel
	singleURLStr := fmt.Sprintf("/api/v1/orgs/%s/labels/%d?token=%s", owner.Name, dbLabel.ID, token)
	req = NewRequest(t, "GET", singleURLStr)
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiLabel)
	assert.EqualValues(t, strings.TrimLeft(dbLabel.Color, "#"), apiLabel.Color)

	// EditLabel
	newName := "LabelNewName"
	newColor := "09876a"
	newColorWrong := "09g76a"
	req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
		Name:  &newName,
		Color: &newColor,
	})
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiLabel)
	assert.EqualValues(t, newColor, apiLabel.Color)
	req = NewRequestWithJSON(t, "PATCH", singleURLStr, &api.EditLabelOption{
		Color: &newColorWrong,
	})
	session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	// DeleteLabel
	req = NewRequest(t, "DELETE", singleURLStr)
	resp = session.MakeRequest(t, req, http.StatusNoContent)
}
