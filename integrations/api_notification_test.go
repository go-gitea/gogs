// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPINotification(t *testing.T) {
	defer prepareTestEnv(t)()

	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	thread5 := models.AssertExistsAndLoadBean(t, &models.Notification{ID: 5}).(*models.Notification)
	assert.NoError(t, thread5.LoadAttributes())
	session := loginUser(t, user2.Name)
	token := getTokenForLoggedInUser(t, session)

	// -- GET /notifications --
	// test filter
	since := "2000-01-01T00%3A50%3A01%2B00%3A00" // 946687801
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications?since=%s&token=%s", since, token))
	resp := session.MakeRequest(t, req, http.StatusOK)
	var apiNL []api.NotificationThread
	DecodeJSON(t, resp, &apiNL)

	assert.Len(t, apiNL, 1)
	assert.EqualValues(t, 5, apiNL[0].ID)

	// test filter
	before := "2000-01-01T01%3A06%3A59%2B00%3A00" // 946688819

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications?all=%s&before=%s&token=%s", "true", before, token))
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiNL)

	assert.Len(t, apiNL, 3)
	assert.EqualValues(t, 4, apiNL[0].ID)
	assert.EqualValues(t, true, apiNL[0].Unread)
	assert.EqualValues(t, false, apiNL[0].Pinned)
	assert.EqualValues(t, 3, apiNL[1].ID)
	assert.EqualValues(t, false, apiNL[1].Unread)
	assert.EqualValues(t, true, apiNL[1].Pinned)
	assert.EqualValues(t, 2, apiNL[2].ID)
	assert.EqualValues(t, false, apiNL[2].Unread)
	assert.EqualValues(t, false, apiNL[2].Pinned)

	// -- GET /repos/{owner}/{repo}/notifications --
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/notifications?status-types=unread&token=%s", user2.Name, repo1.Name, token))
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiNL)

	assert.Len(t, apiNL, 1)
	assert.EqualValues(t, 4, apiNL[0].ID)

	// -- GET /notifications/threads/{id} --
	// get forbidden
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications/threads/%d?token=%s", 1, token))
	resp = session.MakeRequest(t, req, http.StatusForbidden)

	// get own
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications/threads/%d?token=%s", thread5.ID, token))
	resp = session.MakeRequest(t, req, http.StatusOK)
	var apiN api.NotificationThread
	DecodeJSON(t, resp, &apiN)

	assert.EqualValues(t, 5, apiN.ID)
	assert.EqualValues(t, false, apiN.Pinned)
	assert.EqualValues(t, true, apiN.Unread)
	assert.EqualValues(t, "issue4", apiN.Subject.Title)
	assert.EqualValues(t, "Issue", apiN.Subject.Type)
	assert.EqualValues(t, thread5.Issue.APIURL(), apiN.Subject.URL)
	assert.EqualValues(t, thread5.Repository.HTMLURL(), apiN.Repository.HTMLURL)

	new := struct {
		New int64 `json:"new"`
	}{}

	// -- check notifications --
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications/new?token=%s", token))
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &new)
	assert.True(t, new.New > 0)

	// -- mark notifications as read --
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications?status-types=unread&token=%s", token))
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiNL)
	assert.Len(t, apiNL, 2)

	lastReadAt := "2000-01-01T00%3A50%3A01%2B00%3A00" // 946687801 <- only Notification 4 is in this filter ...
	req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/notifications?last_read_at=%s&token=%s", user2.Name, repo1.Name, lastReadAt, token))
	resp = session.MakeRequest(t, req, http.StatusResetContent)

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications?status-types=unread&token=%s", token))
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiNL)
	assert.Len(t, apiNL, 1)

	// -- PATCH /notifications/threads/{id} --
	req = NewRequest(t, "PATCH", fmt.Sprintf("/api/v1/notifications/threads/%d?token=%s", thread5.ID, token))
	resp = session.MakeRequest(t, req, http.StatusResetContent)

	assert.Equal(t, models.NotificationStatusUnread, thread5.Status)
	thread5 = models.AssertExistsAndLoadBean(t, &models.Notification{ID: 5}).(*models.Notification)
	assert.Equal(t, models.NotificationStatusRead, thread5.Status)

	// -- check notifications --
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications/new?token=%s", token))
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &new)
	assert.True(t, new.New == 0)
}
