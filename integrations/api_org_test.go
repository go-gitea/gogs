// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIOrgCreate(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		session := loginUser(t, "user1")

		token := getTokenForLoggedInUser(t, session)
		org := api.CreateOrgOption{
			UserName:    "user1_org",
			FullName:    "User1's organization",
			Description: "This organization created by user1",
			Website:     "https://try.gitea.io",
			Location:    "Shanghai",
			Visibility:  "limited",
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs?token="+token, &org)
		resp := session.MakeRequest(t, req, http.StatusCreated)

		var apiOrg api.Organization
		DecodeJSON(t, resp, &apiOrg)

		assert.Equal(t, org.UserName, apiOrg.UserName)
		assert.Equal(t, org.FullName, apiOrg.FullName)
		assert.Equal(t, org.Description, apiOrg.Description)
		assert.Equal(t, org.Website, apiOrg.Website)
		assert.Equal(t, org.Location, apiOrg.Location)
		assert.Equal(t, org.Visibility, apiOrg.Visibility)

		models.AssertExistsAndLoadBean(t, &models.User{
			Name:      org.UserName,
			LowerName: strings.ToLower(org.UserName),
			FullName:  org.FullName,
		})

		req = NewRequestf(t, "GET", "/api/v1/orgs/%s", org.UserName)
		resp = session.MakeRequest(t, req, http.StatusOK)
		DecodeJSON(t, resp, &apiOrg)
		assert.EqualValues(t, org.UserName, apiOrg.UserName)

		req = NewRequestf(t, "GET", "/api/v1/orgs/%s/repos", org.UserName)
		resp = session.MakeRequest(t, req, http.StatusOK)

		var repos []*api.Repository
		DecodeJSON(t, resp, &repos)
		for _, repo := range repos {
			assert.False(t, repo.Private)
		}

		req = NewRequestf(t, "GET", "/api/v1/orgs/%s/members", org.UserName)
		resp = session.MakeRequest(t, req, http.StatusOK)

		// user1 on this org is public
		var users []*api.User
		DecodeJSON(t, resp, &users)
		assert.EqualValues(t, 1, len(users))
		assert.EqualValues(t, "user1", users[0].UserName)
	})
}

func TestAPIOrgEdit(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		session := loginUser(t, "user1")

		token := getTokenForLoggedInUser(t, session)
		org := api.EditOrgOption{
			FullName:    "User3 organization new full name",
			Description: "A new description",
			Website:     "https://try.gitea.io/new",
			Location:    "Beijing",
			Visibility:  "private",
		}
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/user3?token="+token, &org)
		resp := session.MakeRequest(t, req, http.StatusOK)

		var apiOrg api.Organization
		DecodeJSON(t, resp, &apiOrg)

		assert.Equal(t, "user3", apiOrg.UserName)
		assert.Equal(t, org.FullName, apiOrg.FullName)
		assert.Equal(t, org.Description, apiOrg.Description)
		assert.Equal(t, org.Website, apiOrg.Website)
		assert.Equal(t, org.Location, apiOrg.Location)
		assert.Equal(t, org.Visibility, apiOrg.Visibility)
	})
}

func TestAPIOrgEditBadVisibility(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		session := loginUser(t, "user1")

		token := getTokenForLoggedInUser(t, session)
		org := api.EditOrgOption{
			FullName:    "User3 organization new full name",
			Description: "A new description",
			Website:     "https://try.gitea.io/new",
			Location:    "Beijing",
			Visibility:  "badvisibility",
		}
		req := NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/user3?token="+token, &org)
		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})
}

func TestAPIOrgDeny(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		setting.Service.RequireSignInView = true
		defer func() {
			setting.Service.RequireSignInView = false
		}()

		orgName := "user1_org"
		req := NewRequestf(t, "GET", "/api/v1/orgs/%s", orgName)
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequestf(t, "GET", "/api/v1/orgs/%s/repos", orgName)
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequestf(t, "GET", "/api/v1/orgs/%s/members", orgName)
		MakeRequest(t, req, http.StatusNotFound)
	})
}

func TestAPIGetAll(t *testing.T) {
	defer prepareTestEnv(t)()

	req := NewRequestf(t, "GET", "/api/v1/orgs")
	resp := MakeRequest(t, req, http.StatusOK)

	var apiOrgList []*api.Organization
	DecodeJSON(t, resp, &apiOrgList)

	assert.Len(t, apiOrgList, 7)
	assert.Equal(t, "org25", apiOrgList[0].FullName)
	assert.Equal(t, "public", apiOrgList[0].Visibility)
}
