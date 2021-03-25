// Copyright 2019 The Gitea Authors. All rights reserved.
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

func TestAPIRepoTopic(t *testing.T) {
	defer prepareTestEnv(t)()
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User) // owner of repo2
	user3 := models.AssertExistsAndLoadBean(t, &models.User{ID: 3}).(*models.User) // owner of repo3
	user4 := models.AssertExistsAndLoadBean(t, &models.User{ID: 4}).(*models.User) // write access to repo 3
	repo2 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 2}).(*models.Repository)
	repo3 := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 3}).(*models.Repository)

	// Get user2's token
	session := loginUser(t, user2.Name)
	token2 := getTokenForLoggedInUser(t, session)

	// Test read topics using login
	url := fmt.Sprintf("/api/v1/repos/%s/%s/topics", user2.Name, repo2.Name)
	req := NewRequest(t, "GET", url)
	res := session.MakeRequest(t, req, http.StatusOK)
	var topics *api.TopicName
	DecodeJSON(t, res, &topics)
	assert.ElementsMatch(t, []string{"topicname1", "topicname2"}, topics.TopicNames)

	// Log out user2
	session = emptyTestSession(t)
	url = fmt.Sprintf("/api/v1/repos/%s/%s/topics?token=%s", user2.Name, repo2.Name, token2)

	// Test delete a topic
	req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/topics/%s?token=%s", user2.Name, repo2.Name, "Topicname1", token2)
	res = session.MakeRequest(t, req, http.StatusNoContent)

	// Test add an existing topic
	req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s?token=%s", user2.Name, repo2.Name, "Golang", token2)
	res = session.MakeRequest(t, req, http.StatusNoContent)

	// Test add a topic
	req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s?token=%s", user2.Name, repo2.Name, "topicName3", token2)
	res = session.MakeRequest(t, req, http.StatusNoContent)

	// Test read topics using token
	req = NewRequest(t, "GET", url)
	res = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.ElementsMatch(t, []string{"topicname2", "golang", "topicname3"}, topics.TopicNames)

	// Test replace topics
	newTopics := []string{"   windows ", "   ", "MAC  "}
	req = NewRequestWithJSON(t, "PUT", url, &api.RepoTopicOptions{
		Topics: newTopics,
	})
	res = session.MakeRequest(t, req, http.StatusNoContent)
	req = NewRequest(t, "GET", url)
	res = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.ElementsMatch(t, []string{"windows", "mac"}, topics.TopicNames)

	// Test replace topics with something invalid
	newTopics = []string{"topicname1", "topicname2", "topicname!"}
	req = NewRequestWithJSON(t, "PUT", url, &api.RepoTopicOptions{
		Topics: newTopics,
	})
	res = session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	req = NewRequest(t, "GET", url)
	res = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.ElementsMatch(t, []string{"windows", "mac"}, topics.TopicNames)

	// Test with some topics multiple times, less than 25 unique
	newTopics = []string{"t1", "t2", "t1", "t3", "t4", "t5", "t6", "t7", "t8", "t9", "t10", "t11", "t12", "t13", "t14", "t15", "t16", "17", "t18", "t19", "t20", "t21", "t22", "t23", "t24", "t25"}
	req = NewRequestWithJSON(t, "PUT", url, &api.RepoTopicOptions{
		Topics: newTopics,
	})
	res = session.MakeRequest(t, req, http.StatusNoContent)
	req = NewRequest(t, "GET", url)
	res = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.Equal(t, 25, len(topics.TopicNames))

	// Test writing more topics than allowed
	newTopics = append(newTopics, "t26")
	req = NewRequestWithJSON(t, "PUT", url, &api.RepoTopicOptions{
		Topics: newTopics,
	})
	res = session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	// Test add a topic when there is already maximum
	req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s?token=%s", user2.Name, repo2.Name, "t26", token2)
	res = session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	// Test delete a topic that repo doesn't have
	req = NewRequestf(t, "DELETE", "/api/v1/repos/%s/%s/topics/%s?token=%s", user2.Name, repo2.Name, "Topicname1", token2)
	res = session.MakeRequest(t, req, http.StatusNotFound)

	// Get user4's token
	session = loginUser(t, user4.Name)
	token4 := getTokenForLoggedInUser(t, session)
	session = emptyTestSession(t)

	// Test read topics with write access
	url = fmt.Sprintf("/api/v1/repos/%s/%s/topics?token=%s", user3.Name, repo3.Name, token4)
	req = NewRequest(t, "GET", url)
	res = session.MakeRequest(t, req, http.StatusOK)
	DecodeJSON(t, res, &topics)
	assert.Equal(t, 0, len(topics.TopicNames))

	// Test add a topic to repo with write access (requires repo admin access)
	req = NewRequestf(t, "PUT", "/api/v1/repos/%s/%s/topics/%s?token=%s", user3.Name, repo3.Name, "topicName", token4)
	res = session.MakeRequest(t, req, http.StatusForbidden)
}
