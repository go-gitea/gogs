// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestAPIAdminOrgCreate(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		session := loginUser(t, "user1")
		token := getTokenForLoggedInUser(t, session)

		org := api.CreateOrgOption{
			UserName:    "user2_org",
			FullName:    "User2's organization",
			Description: "This organization created by admin for user2",
			Website:     "https://try.gitea.io",
			Location:    "Shanghai",
			Visibility:  "private",
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/admin/users/user2/orgs?token="+token, &org)
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
	})
}

func TestAPIAdminOrgCreateBadVisibility(t *testing.T) {
	onGiteaRun(t, func(*testing.T, *url.URL) {
		session := loginUser(t, "user1")
		token := getTokenForLoggedInUser(t, session)

		org := api.CreateOrgOption{
			UserName:    "user2_org",
			FullName:    "User2's organization",
			Description: "This organization created by admin for user2",
			Website:     "https://try.gitea.io",
			Location:    "Shanghai",
			Visibility:  "notvalid",
		}
		req := NewRequestWithJSON(t, "POST", "/api/v1/admin/users/user2/orgs?token="+token, &org)
		session.MakeRequest(t, req, http.StatusUnprocessableEntity)
	})
}

func TestAPIAdminOrgCreateNotAdmin(t *testing.T) {
	defer prepareTestEnv(t)()
	nonAdminUsername := "user2"
	session := loginUser(t, nonAdminUsername)
	token := getTokenForLoggedInUser(t, session)
	org := api.CreateOrgOption{
		UserName:    "user2_org",
		FullName:    "User2's organization",
		Description: "This organization created by admin for user2",
		Website:     "https://try.gitea.io",
		Location:    "Shanghai",
		Visibility:  "public",
	}
	req := NewRequestWithJSON(t, "POST", "/api/v1/admin/users/user2/orgs?token="+token, &org)
	session.MakeRequest(t, req, http.StatusForbidden)
}
