// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
)

func TestAPIAdminCreateAndDeleteSSHKey(t *testing.T) {
	defer prepareTestEnv(t)()
	// user1 is an admin user
	session := loginUser(t, "user1")
	keyOwner := models.AssertExistsAndLoadBean(t, &models.User{Name: "user2"}).(*models.User)

	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s/keys?token=%s", keyOwner.Name, token)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"key":   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
		"title": "test-key",
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)

	var newPublicKey api.PublicKey
	DecodeJSON(t, resp, &newPublicKey)
	models.AssertExistsAndLoadBean(t, &models.PublicKey{
		ID:          newPublicKey.ID,
		Name:        newPublicKey.Title,
		Content:     newPublicKey.Key,
		Fingerprint: newPublicKey.Fingerprint,
		OwnerID:     keyOwner.ID,
	})

	req = NewRequestf(t, "DELETE", "/api/v1/admin/users/%s/keys/%d?token=%s",
		keyOwner.Name, newPublicKey.ID, token)
	session.MakeRequest(t, req, http.StatusNoContent)
	models.AssertNotExistsBean(t, &models.PublicKey{ID: newPublicKey.ID})
}

func TestAPIAdminDeleteMissingSSHKey(t *testing.T) {
	defer prepareTestEnv(t)()
	// user1 is an admin user
	session := loginUser(t, "user1")

	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "DELETE", "/api/v1/admin/users/user1/keys/%d?token=%s", models.NonexistentID, token)
	session.MakeRequest(t, req, http.StatusNotFound)
}

func TestAPIAdminDeleteUnauthorizedKey(t *testing.T) {
	defer prepareTestEnv(t)()
	adminUsername := "user1"
	normalUsername := "user2"
	session := loginUser(t, adminUsername)

	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s/keys?token=%s", adminUsername, token)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"key":   "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC4cn+iXnA4KvcQYSV88vGn0Yi91vG47t1P7okprVmhNTkipNRIHWr6WdCO4VDr/cvsRkuVJAsLO2enwjGWWueOO6BodiBgyAOZ/5t5nJNMCNuLGT5UIo/RI1b0WRQwxEZTRjt6mFNw6lH14wRd8ulsr9toSWBPMOGWoYs1PDeDL0JuTjL+tr1SZi/EyxCngpYszKdXllJEHyI79KQgeD0Vt3pTrkbNVTOEcCNqZePSVmUH8X8Vhugz3bnE0/iE9Pb5fkWO9c4AnM1FgI/8Bvp27Fw2ShryIXuR6kKvUqhVMTuOSDHwu6A8jLE5Owt3GAYugDpDYuwTVNGrHLXKpPzrGGPE/jPmaLCMZcsdkec95dYeU3zKODEm8UQZFhmJmDeWVJ36nGrGZHL4J5aTTaeFUJmmXDaJYiJ+K2/ioKgXqnXvltu0A9R8/LGy4nrTJRr4JMLuJFoUXvGm1gXQ70w2LSpk6yl71RNC0hCtsBe8BP8IhYCM0EP5jh7eCMQZNvM= nocomment\n",
		"title": "test-key",
	})
	resp := session.MakeRequest(t, req, http.StatusCreated)
	var newPublicKey api.PublicKey
	DecodeJSON(t, resp, &newPublicKey)

	session = loginUser(t, normalUsername)
	token = getTokenForLoggedInUser(t, session)
	req = NewRequestf(t, "DELETE", "/api/v1/admin/users/%s/keys/%d?token=%s",
		adminUsername, newPublicKey.ID, token)
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAPISudoUser(t *testing.T) {
	defer prepareTestEnv(t)()
	adminUsername := "user1"
	normalUsername := "user2"
	session := loginUser(t, adminUsername)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/user?sudo=%s&token=%s", normalUsername, token)
	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var user api.User
	DecodeJSON(t, resp, &user)

	assert.Equal(t, normalUsername, user.UserName)
}

func TestAPISudoUserForbidden(t *testing.T) {
	defer prepareTestEnv(t)()
	adminUsername := "user1"
	normalUsername := "user2"

	session := loginUser(t, normalUsername)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/user?sudo=%s&token=%s", adminUsername, token)
	req := NewRequest(t, "GET", urlStr)
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAPIListUsers(t *testing.T) {
	defer prepareTestEnv(t)()
	adminUsername := "user1"
	session := loginUser(t, adminUsername)
	token := getTokenForLoggedInUser(t, session)

	urlStr := fmt.Sprintf("/api/v1/admin/users?token=%s", token)
	req := NewRequest(t, "GET", urlStr)
	resp := session.MakeRequest(t, req, http.StatusOK)
	var users []api.User
	DecodeJSON(t, resp, &users)

	found := false
	for _, user := range users {
		if user.UserName == adminUsername {
			found = true
		}
	}
	assert.True(t, found)
	numberOfUsers := models.GetCount(t, &models.User{}, "type = 0")
	assert.Equal(t, numberOfUsers, len(users))
}

func TestAPIListUsersNotLoggedIn(t *testing.T) {
	defer prepareTestEnv(t)()
	req := NewRequest(t, "GET", "/api/v1/admin/users")
	MakeRequest(t, req, http.StatusUnauthorized)
}

func TestAPIListUsersNonAdmin(t *testing.T) {
	defer prepareTestEnv(t)()
	nonAdminUsername := "user2"
	session := loginUser(t, nonAdminUsername)
	token := getTokenForLoggedInUser(t, session)
	req := NewRequestf(t, "GET", "/api/v1/admin/users?token=%s", token)
	session.MakeRequest(t, req, http.StatusForbidden)
}

func TestAPICreateUserInvalidEmail(t *testing.T) {
	defer prepareTestEnv(t)()
	adminUsername := "user1"
	session := loginUser(t, adminUsername)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/admin/users?token=%s", token)
	req := NewRequestWithValues(t, "POST", urlStr, map[string]string{
		"email":                "invalid_email@domain.com\r\n",
		"full_name":            "invalid user",
		"login_name":           "invalidUser",
		"must_change_password": "true",
		"password":             "password",
		"send_notify":          "true",
		"source_id":            "0",
		"username":             "invalidUser",
	})
	session.MakeRequest(t, req, http.StatusUnprocessableEntity)
}

func TestAPIEditUser(t *testing.T) {
	defer prepareTestEnv(t)()
	adminUsername := "user1"
	session := loginUser(t, adminUsername)
	token := getTokenForLoggedInUser(t, session)
	urlStr := fmt.Sprintf("/api/v1/admin/users/%s?token=%s", "user2", token)

	req := NewRequestWithValues(t, "PATCH", urlStr, map[string]string{
		// required
		"login_name": "user2",
		"source_id":  "0",
		// to change
		"full_name": "Full Name User 2",
	})
	session.MakeRequest(t, req, http.StatusOK)

	empty := ""
	req = NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{
		LoginName: "user2",
		SourceID:  0,
		Email:     &empty,
	})
	resp := session.MakeRequest(t, req, http.StatusUnprocessableEntity)

	errMap := make(map[string]interface{})
	json.Unmarshal(resp.Body.Bytes(), &errMap)
	assert.EqualValues(t, "email is not allowed to be empty string", errMap["message"].(string))

	user2 := models.AssertExistsAndLoadBean(t, &models.User{LoginName: "user2"}).(*models.User)
	assert.False(t, user2.IsRestricted)
	bTrue := true
	req = NewRequestWithJSON(t, "PATCH", urlStr, api.EditUserOption{
		// required
		LoginName: "user2",
		SourceID:  0,
		// to change
		Restricted: &bTrue,
	})
	session.MakeRequest(t, req, http.StatusOK)
	user2 = models.AssertExistsAndLoadBean(t, &models.User{LoginName: "user2"}).(*models.User)
	assert.True(t, user2.IsRestricted)
}
