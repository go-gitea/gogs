// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"

	"github.com/stretchr/testify/assert"
	"github.com/unknwon/i18n"
)

type ldapUser struct {
	UserName     string
	Password     string
	FullName     string
	Email        string
	OtherEmails  []string
	IsAdmin      bool
	IsRestricted bool
	SSHKeys      []string
}

var gitLDAPUsers = []ldapUser{
	{
		UserName:    "professor",
		Password:    "professor",
		FullName:    "Hubert Farnsworth",
		Email:       "professor@planetexpress.com",
		OtherEmails: []string{"hubert@planetexpress.com"},
		IsAdmin:     true,
	},
	{
		UserName: "hermes",
		Password: "hermes",
		FullName: "Conrad Hermes",
		Email:    "hermes@planetexpress.com",
		SSHKeys: []string{
			"SHA256:qLY06smKfHoW/92yXySpnxFR10QFrLdRjf/GNPvwcW8",
			"SHA256:QlVTuM5OssDatqidn2ffY+Lc4YA5Fs78U+0KOHI51jQ",
			"SHA256:DXdeUKYOJCSSmClZuwrb60hUq7367j4fA+udNC3FdRI",
		},
		IsAdmin: true,
	},
	{
		UserName: "fry",
		Password: "fry",
		FullName: "Philip Fry",
		Email:    "fry@planetexpress.com",
	},
	{
		UserName:     "leela",
		Password:     "leela",
		FullName:     "Leela Turanga",
		Email:        "leela@planetexpress.com",
		IsRestricted: true,
	},
	{
		UserName: "bender",
		Password: "bender",
		FullName: "Bender Rodríguez",
		Email:    "bender@planetexpress.com",
	},
}

var otherLDAPUsers = []ldapUser{
	{
		UserName: "zoidberg",
		Password: "zoidberg",
		FullName: "John Zoidberg",
		Email:    "zoidberg@planetexpress.com",
	},
	{
		UserName: "amy",
		Password: "amy",
		FullName: "Amy Kroker",
		Email:    "amy@planetexpress.com",
	},
}

func skipLDAPTests() bool {
	return os.Getenv("TEST_LDAP") != "1"
}

func getLDAPServerHost() string {
	host := os.Getenv("TEST_LDAP_HOST")
	if len(host) == 0 {
		host = "ldap"
	}
	return host
}

func addAuthSourceLDAP(t *testing.T, sshKeyAttribute string) {
	session := loginUser(t, "user1")
	csrf := GetCSRF(t, session, "/admin/auths/new")
	req := NewRequestWithValues(t, "POST", "/admin/auths/new", map[string]string{
		"_csrf":                    csrf,
		"type":                     "2",
		"name":                     "ldap",
		"host":                     getLDAPServerHost(),
		"port":                     "389",
		"bind_dn":                  "uid=gitea,ou=service,dc=planetexpress,dc=com",
		"bind_password":            "password",
		"user_base":                "ou=people,dc=planetexpress,dc=com",
		"filter":                   "(&(objectClass=inetOrgPerson)(memberOf=cn=git,ou=people,dc=planetexpress,dc=com)(uid=%s))",
		"admin_filter":             "(memberOf=cn=admin_staff,ou=people,dc=planetexpress,dc=com)",
		"restricted_filter":        "(uid=leela)",
		"attribute_username":       "uid",
		"attribute_name":           "givenName",
		"attribute_surname":        "sn",
		"attribute_mail":           "mail",
		"attribute_ssh_public_key": sshKeyAttribute,
		"is_sync_enabled":          "on",
		"is_active":                "on",
		"team_group_map_enabled":   "on",
		"team_group_map_removal":   "on",
		"group_dn":                 "ou=people,dc=planetexpress,dc=com",
		"group_member_uid":         "member",
		"user_uid":                 "DN",
		"team_group_map":           "{\"cn=ship_crew,ou=people,dc=planetexpress,dc=com\": {\"org26\": [\"team11\"]},\"cn=admin_staff,ou=people,dc=planetexpress,dc=com\": {\"non-existent\": [\"non-existent\"]},\"cn=non-existent,ou=people,dc=planetexpress,dc=com\": {\"non-existent\": [\"non-existent\"]}}",
	})
	session.MakeRequest(t, req, http.StatusFound)
}

func TestLDAPUserSignin(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	defer prepareTestEnv(t)()
	addAuthSourceLDAP(t, "")

	u := gitLDAPUsers[0]

	session := loginUserWithPassword(t, u.UserName, u.Password)
	req := NewRequest(t, "GET", "/user/settings")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	assert.Equal(t, u.UserName, htmlDoc.GetInputValueByName("name"))
	assert.Equal(t, u.FullName, htmlDoc.GetInputValueByName("full_name"))
	assert.Equal(t, u.Email, htmlDoc.Find(`label[for="email"]`).Siblings().First().Text())
}

func TestLDAPUserSync(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	defer prepareTestEnv(t)()
	addAuthSourceLDAP(t, "")
	models.SyncExternalUsers(context.Background(), true)

	session := loginUser(t, "user1")
	// Check if users exists
	for _, u := range gitLDAPUsers {
		req := NewRequest(t, "GET", "/admin/users?q="+u.UserName)
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)

		tr := htmlDoc.doc.Find("table.table tbody tr")
		if !assert.True(t, tr.Length() == 1) {
			continue
		}
		tds := tr.Find("td")
		if !assert.True(t, tds.Length() > 0) {
			continue
		}
		assert.Equal(t, u.UserName, strings.TrimSpace(tds.Find("td:nth-child(2) a").Text()))
		assert.Equal(t, u.Email, strings.TrimSpace(tds.Find("td:nth-child(3) span").Text()))
		if u.IsAdmin {
			assert.True(t, tds.Find("td:nth-child(5) svg").HasClass("octicon-check"))
		} else {
			assert.True(t, tds.Find("td:nth-child(5) svg").HasClass("octicon-x"))
		}
		if u.IsRestricted {
			assert.True(t, tds.Find("td:nth-child(6) svg").HasClass("octicon-check"))
		} else {
			assert.True(t, tds.Find("td:nth-child(6) svg").HasClass("octicon-x"))
		}
	}

	// Check if no users exist
	for _, u := range otherLDAPUsers {
		req := NewRequest(t, "GET", "/admin/users?q="+u.UserName)
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)

		tr := htmlDoc.doc.Find("table.table tbody tr")
		assert.True(t, tr.Length() == 0)
	}
}

func TestLDAPUserSigninFailed(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	defer prepareTestEnv(t)()
	addAuthSourceLDAP(t, "")

	u := otherLDAPUsers[0]

	testLoginFailed(t, u.UserName, u.Password, i18n.Tr("en", "form.username_password_incorrect"))
}

func TestLDAPUserSSHKeySync(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	defer prepareTestEnv(t)()
	addAuthSourceLDAP(t, "sshPublicKey")

	models.SyncExternalUsers(context.Background(), true)

	// Check if users has SSH keys synced
	for _, u := range gitLDAPUsers {
		if len(u.SSHKeys) == 0 {
			continue
		}
		session := loginUserWithPassword(t, u.UserName, u.Password)

		req := NewRequest(t, "GET", "/user/settings/keys")
		resp := session.MakeRequest(t, req, http.StatusOK)

		htmlDoc := NewHTMLParser(t, resp.Body)

		divs := htmlDoc.doc.Find(".key.list .print.meta")

		syncedKeys := make([]string, divs.Length())
		for i := 0; i < divs.Length(); i++ {
			syncedKeys[i] = strings.TrimSpace(divs.Eq(i).Text())
		}

		assert.ElementsMatch(t, u.SSHKeys, syncedKeys, "Unequal number of keys synchronized for user: %s", u.UserName)
	}
}

func TestLDAPGroupTeamSyncAddMember(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	defer prepareTestEnv(t)()
	addAuthSourceLDAP(t, "")
	org, err := models.GetOrgByName("org26")
	assert.NoError(t, err)
	team, err := models.GetTeam(org.ID, "team11")
	assert.NoError(t, err)
	models.SyncExternalUsers(context.Background(), true)
	for _, gitLDAPUser := range gitLDAPUsers {
		user := models.AssertExistsAndLoadBean(t, &models.User{
			Name: gitLDAPUser.UserName,
		}).(*models.User)
		usersOrgs, err := models.GetOrgsByUserID(user.ID, true)
		assert.NoError(t, err)
		allOrgTeams, err := models.GetUserOrgTeams(org.ID, user.ID)
		assert.NoError(t, err)
		if user.Name == "fry" || user.Name == "leela" || user.Name == "bender" {
			// assert members of LDAP group "cn=ship_crew" are added to mapped teams
			assert.Equal(t, len(usersOrgs), 1, "User should be member of one organization")
			assert.Equal(t, usersOrgs[0].Name, "org26", "Membership should be added to the right organization")
			isMember, err := models.IsTeamMember(usersOrgs[0].ID, team.ID, user.ID)
			assert.NoError(t, err)
			assert.True(t, isMember, "Membership should be added to the right team")
			err = team.RemoveMember(user.ID)
			assert.NoError(t, err)
		} else {
			// assert members of LDAP group "cn=admin_staff" keep initial team membership since mapped team does not exist
			assert.Empty(t, usersOrgs, "User should be member of no organization")
			isMember, err := models.IsTeamMember(org.ID, team.ID, user.ID)
			assert.NoError(t, err)
			assert.False(t, isMember, "User should no be added to this team")
			assert.Empty(t, allOrgTeams, "User should not be added to any team")
		}
	}
}

func TestLDAPGroupTeamSyncRemoveMember(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	defer prepareTestEnv(t)()
	addAuthSourceLDAP(t, "")
	models.SyncExternalUsers(context.Background(), true)
	org, err := models.GetOrgByName("org26")
	assert.NoError(t, err)
	team, err := models.GetTeam(org.ID, "team11")
	assert.NoError(t, err)
	user, err := models.GetUserByName("professor")
	assert.NoError(t, err)
	err = org.AddMember(user.ID)
	assert.NoError(t, err)
	err = team.AddMember(user.ID)
	assert.NoError(t, err)
	isMember, err := models.IsOrganizationMember(org.ID, user.ID)
	assert.NoError(t, err)
	assert.True(t, isMember, "User should be member of this organization")
	isMember, err = models.IsTeamMember(org.ID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.True(t, isMember, "User should be member of this team")
	// assert team member "professor" gets removed from "team11"
	models.SyncExternalUsers(context.Background(), true)
	isMember, err = models.IsOrganizationMember(org.ID, user.ID)
	assert.NoError(t, err)
	assert.False(t, isMember, "User membership should have been removed from organization")
	isMember, err = models.IsTeamMember(org.ID, team.ID, user.ID)
	assert.NoError(t, err)
	assert.False(t, isMember, "User membership should have been removed from team")
}

func addBrokenLDAPMapAuthSource(t *testing.T, sshKeyAttribute string) {
	session := loginUser(t, "user1")
	csrf := GetCSRF(t, session, "/admin/auths/new")
	req := NewRequestWithValues(t, "POST", "/admin/auths/new", map[string]string{
		"_csrf":                    csrf,
		"type":                     "2",
		"name":                     "ldap",
		"host":                     getLDAPServerHost(),
		"port":                     "389",
		"bind_dn":                  "uid=gitea,ou=service,dc=planetexpress,dc=com",
		"bind_password":            "password",
		"user_base":                "ou=people,dc=planetexpress,dc=com",
		"filter":                   "(&(objectClass=inetOrgPerson)(memberOf=cn=git,ou=people,dc=planetexpress,dc=com)(uid=%s))",
		"admin_filter":             "(memberOf=cn=admin_staff,ou=people,dc=planetexpress,dc=com)",
		"restricted_filter":        "(uid=leela)",
		"attribute_username":       "uid",
		"attribute_name":           "givenName",
		"attribute_surname":        "sn",
		"attribute_mail":           "mail",
		"attribute_ssh_public_key": sshKeyAttribute,
		"is_sync_enabled":          "on",
		"is_active":                "on",
		"team_group_map_enabled":   "on",
		"team_group_map_removal":   "on",
		"group_dn":                 "ou=people,dc=planetexpress,dc=com",
		"group_member_uid":         "member",
		"user_uid":                 "DN",
		"team_group_map":           "{\"NOT_A_VALID_JSON\"[\"MISSING_DOUBLE_POINT\"]}",
	})
	session.MakeRequest(t, req, http.StatusFound)
}

// Login should work even if Team Group Map contains a broken JSON
func TestBrokenLDAPMapUserSignin(t *testing.T) {
	if skipLDAPTests() {
		t.Skip()
		return
	}
	defer prepareTestEnv(t)()
	addBrokenLDAPMapAuthSource(t, "")

	u := gitLDAPUsers[0]

	session := loginUserWithPassword(t, u.UserName, u.Password)
	req := NewRequest(t, "GET", "/user/settings")
	resp := session.MakeRequest(t, req, http.StatusOK)

	htmlDoc := NewHTMLParser(t, resp.Body)

	assert.Equal(t, u.UserName, htmlDoc.GetInputValueByName("name"))
	assert.Equal(t, u.FullName, htmlDoc.GetInputValueByName("full_name"))
	assert.Equal(t, u.Email, htmlDoc.Find(`label[for="email"]`).Siblings().First().Text())
}
