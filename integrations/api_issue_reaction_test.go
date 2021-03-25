// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIIssuesReactions(t *testing.T) {
	defer prepareTestEnv(t)()

	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 1}).(*models.Issue)
	_ = issue.LoadRepo()
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: issue.Repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/reactions?token=%s",
		owner.Name, issue.Repo.Name, issue.Index, token)

	// Try to add not allowed reaction
	req := NewRequestWithJSON(t, "POST", urlStr, &api.EditReactionOption{
		Reaction: "wrong",
	})
	resp := session.MakeRequest(t, req, http.StatusForbidden)

	// Delete not allowed reaction
	req = NewRequestWithJSON(t, "DELETE", urlStr, &api.EditReactionOption{
		Reaction: "zzz",
	})
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Add allowed reaction
	req = NewRequestWithJSON(t, "POST", urlStr, &api.EditReactionOption{
		Reaction: "rocket",
	})
	resp = session.MakeRequest(t, req, http.StatusCreated)
	var apiNewReaction api.Reaction
	DecodeJSON(t, resp, &apiNewReaction)

	// Add existing reaction
	resp = session.MakeRequest(t, req, http.StatusForbidden)

	// Get end result of reaction list of issue #1
	req = NewRequestf(t, "GET", urlStr)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var apiReactions []*api.Reaction
	DecodeJSON(t, resp, &apiReactions)
	expectResponse := make(map[int]api.Reaction)
	expectResponse[0] = api.Reaction{
		User:     convert.ToUser(user2, true, true),
		Reaction: "eyes",
		Created:  time.Unix(1573248003, 0),
	}
	expectResponse[1] = apiNewReaction
	assert.Len(t, apiReactions, 2)
	for i, r := range apiReactions {
		assert.Equal(t, expectResponse[i].Reaction, r.Reaction)
		assert.Equal(t, expectResponse[i].Created.Unix(), r.Created.Unix())
		assert.Equal(t, expectResponse[i].User.ID, r.User.ID)
	}
}

func TestAPICommentReactions(t *testing.T) {
	defer prepareTestEnv(t)()

	comment := models.AssertExistsAndLoadBean(t, &models.Comment{ID: 2}).(*models.Comment)
	_ = comment.LoadIssue()
	issue := comment.Issue
	_ = issue.LoadRepo()
	owner := models.AssertExistsAndLoadBean(t, &models.User{ID: issue.Repo.OwnerID}).(*models.User)

	session := loginUser(t, owner.Name)
	token := getTokenForLoggedInUser(t, session)

	user1 := models.AssertExistsAndLoadBean(t, &models.User{ID: 1}).(*models.User)
	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/comments/%d/reactions?token=%s",
		owner.Name, issue.Repo.Name, comment.ID, token)

	// Try to add not allowed reaction
	req := NewRequestWithJSON(t, "POST", urlStr, &api.EditReactionOption{
		Reaction: "wrong",
	})
	resp := session.MakeRequest(t, req, http.StatusForbidden)

	// Delete none existing reaction
	req = NewRequestWithJSON(t, "DELETE", urlStr, &api.EditReactionOption{
		Reaction: "eyes",
	})
	resp = session.MakeRequest(t, req, http.StatusOK)

	// Add allowed reaction
	req = NewRequestWithJSON(t, "POST", urlStr, &api.EditReactionOption{
		Reaction: "+1",
	})
	resp = session.MakeRequest(t, req, http.StatusCreated)
	var apiNewReaction api.Reaction
	DecodeJSON(t, resp, &apiNewReaction)

	// Add existing reaction
	resp = session.MakeRequest(t, req, http.StatusForbidden)

	// Get end result of reaction list of issue #1
	req = NewRequestf(t, "GET", urlStr)
	resp = session.MakeRequest(t, req, http.StatusOK)
	var apiReactions []*api.Reaction
	DecodeJSON(t, resp, &apiReactions)
	expectResponse := make(map[int]api.Reaction)
	expectResponse[0] = api.Reaction{
		User:     convert.ToUser(user2, true, true),
		Reaction: "laugh",
		Created:  time.Unix(1573248004, 0),
	}
	expectResponse[1] = api.Reaction{
		User:     convert.ToUser(user1, true, true),
		Reaction: "laugh",
		Created:  time.Unix(1573248005, 0),
	}
	expectResponse[2] = apiNewReaction
	assert.Len(t, apiReactions, 3)
	for i, r := range apiReactions {
		assert.Equal(t, expectResponse[i].Reaction, r.Reaction)
		assert.Equal(t, expectResponse[i].Created.Unix(), r.Created.Unix())
		assert.Equal(t, expectResponse[i].User.ID, r.User.ID)
	}
}
