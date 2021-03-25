// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/eventsource"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestEventSourceManagerRun(t *testing.T) {
	defer prepareTestEnv(t)()
	manager := eventsource.GetManager()

	eventChan := manager.Register(2)
	defer func() {
		manager.Unregister(2, eventChan)
		// ensure the eventChan is closed
		for {
			_, ok := <-eventChan
			if !ok {
				break
			}
		}
	}()
	expectNotificationCountEvent := func(count int64) func() bool {
		return func() bool {
			select {
			case event, ok := <-eventChan:
				if !ok {
					return false
				}
				data, ok := event.Data.(models.UserIDCount)
				if !ok {
					return false
				}
				return event.Name == "notification-count" && data.Count == count
			default:
				return false
			}
		}
	}

	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo1 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1}).(*models.Repository)
	thread5 := models.AssertExistsAndLoadBean(t, &models.Notification{ID: 5}).(*models.Notification)
	assert.NoError(t, thread5.LoadAttributes())
	session := loginUser(t, user2.Name)
	token := getTokenForLoggedInUser(t, session)

	var apiNL []api.NotificationThread

	// -- mark notifications as read --
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications?status-types=unread&token=%s", token))
	resp := session.MakeRequest(t, req, http.StatusOK)

	DecodeJSON(t, resp, &apiNL)
	assert.Len(t, apiNL, 2)

	lastReadAt := "2000-01-01T00%3A50%3A01%2B00%3A00" // 946687801 <- only Notification 4 is in this filter ...
	req = NewRequest(t, "PUT", fmt.Sprintf("/api/v1/repos/%s/%s/notifications?last_read_at=%s&token=%s", user2.Name, repo1.Name, lastReadAt, token))
	resp = session.MakeRequest(t, req, http.StatusResetContent)

	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/notifications?token=%s&status-types=unread", token))
	resp = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, resp, &apiNL)
	assert.Len(t, apiNL, 1)

	assert.Eventually(t, expectNotificationCountEvent(1), 30*time.Second, 1*time.Second)
}
