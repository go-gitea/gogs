// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	"github.com/stretchr/testify/assert"
)

func TestAPITeamUser(t *testing.T) {
	prepareTestEnv(t)

	normalUsername := "user2"
	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequest(t, "GET", "/api/v1/teams/1/members/user1?token="+token)
	session.MakeRequest(t, req, http.StatusNotFound)

	req = NewRequest(t, "GET", "/api/v1/teams/1/members/user2?token="+token)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var user2 *api.User
	DecodeJSON(t, resp, &user2)
	user := models.AssertExistsAndLoadBean(t, &models.User{Name: "user2"}).(*models.User)

	assert.Equal(t, user.APIFormat(), user2)
}
