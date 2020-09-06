// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

//////////////////// Application

func TestOAuth2Application_GenerateClientSecret(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	app := AssertExistsAndLoadBean(t, &OAuth2Application{ID: 1}).(*OAuth2Application)
	secret, err := app.GenerateClientSecret()
	assert.NoError(t, err)
	assert.True(t, len(secret) > 0)
	AssertExistsAndLoadBean(t, &OAuth2Application{ID: 1, ClientSecret: app.ClientSecret})
}

func BenchmarkOAuth2Application_GenerateClientSecret(b *testing.B) {
	assert.NoError(b, PrepareTestDatabase())
	app := AssertExistsAndLoadBean(b, &OAuth2Application{ID: 1}).(*OAuth2Application)
	for i := 0; i < b.N; i++ {
		_, _ = app.GenerateClientSecret()
	}
}

func TestOAuth2Application_ContainsRedirectURI(t *testing.T) {
	app := &OAuth2Application{
		RedirectURIs: []string{"a", "b", "c"},
	}
	assert.True(t, app.ContainsRedirectURI("a"))
	assert.True(t, app.ContainsRedirectURI("b"))
	assert.True(t, app.ContainsRedirectURI("c"))
	assert.False(t, app.ContainsRedirectURI("d"))
}

func TestOAuth2Application_ValidateClientSecret(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	app := AssertExistsAndLoadBean(t, &OAuth2Application{ID: 1}).(*OAuth2Application)
	secret, err := app.GenerateClientSecret()
	assert.NoError(t, err)
	assert.True(t, app.ValidateClientSecret([]byte(secret)))
	assert.False(t, app.ValidateClientSecret([]byte("fewijfowejgfiowjeoifew")))
}

func TestGetOAuth2ApplicationByClientID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	app, err := GetOAuth2ApplicationByClientID("da7da3ba-9a13-4167-856f-3899de0b0138")
	assert.NoError(t, err)
	assert.Equal(t, "da7da3ba-9a13-4167-856f-3899de0b0138", app.ClientID)

	app, err = GetOAuth2ApplicationByClientID("invalid client id")
	assert.Error(t, err)
	assert.Nil(t, app)
}

func TestCreateOAuth2Application(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	app, err := CreateOAuth2Application(CreateOAuth2ApplicationOptions{Name: "newapp", UserID: 1})
	assert.NoError(t, err)
	assert.Equal(t, "newapp", app.Name)
	assert.Len(t, app.ClientID, 36)
	AssertExistsAndLoadBean(t, &OAuth2Application{Name: "newapp"})
}

func TestOAuth2Application_LoadUser(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	app := AssertExistsAndLoadBean(t, &OAuth2Application{ID: 1}).(*OAuth2Application)
	assert.NoError(t, app.LoadUser())
	assert.NotNil(t, app.User)
}

func TestOAuth2Application_TableName(t *testing.T) {
	assert.Equal(t, os.Getenv("TABLE_NAME_PREFIX")+"oauth2_application", new(OAuth2Application).TableName())
}

func TestOAuth2Application_GetGrantByUserID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	app := AssertExistsAndLoadBean(t, &OAuth2Application{ID: 1}).(*OAuth2Application)
	grant, err := app.GetGrantByUserID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), grant.UserID)

	grant, err = app.GetGrantByUserID(34923458)
	assert.NoError(t, err)
	assert.Nil(t, grant)
}

func TestOAuth2Application_CreateGrant(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	app := AssertExistsAndLoadBean(t, &OAuth2Application{ID: 1}).(*OAuth2Application)
	grant, err := app.CreateGrant(2)
	assert.NoError(t, err)
	assert.NotNil(t, grant)
	assert.Equal(t, int64(2), grant.UserID)
	assert.Equal(t, int64(1), grant.ApplicationID)
}

//////////////////// Grant

func TestGetOAuth2GrantByID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	grant, err := GetOAuth2GrantByID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), grant.ID)

	grant, err = GetOAuth2GrantByID(34923458)
	assert.NoError(t, err)
	assert.Nil(t, grant)
}

func TestOAuth2Grant_IncreaseCounter(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	grant := AssertExistsAndLoadBean(t, &OAuth2Grant{ID: 1, Counter: 1}).(*OAuth2Grant)
	assert.NoError(t, grant.IncreaseCounter())
	assert.Equal(t, int64(2), grant.Counter)
	AssertExistsAndLoadBean(t, &OAuth2Grant{ID: 1, Counter: 2})
}

func TestOAuth2Grant_GenerateNewAuthorizationCode(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	grant := AssertExistsAndLoadBean(t, &OAuth2Grant{ID: 1}).(*OAuth2Grant)
	code, err := grant.GenerateNewAuthorizationCode("https://example2.com/callback", "CjvyTLSdR47G5zYenDA-eDWW4lRrO8yvjcWwbD_deOg", "S256")
	assert.NoError(t, err)
	assert.NotNil(t, code)
	assert.True(t, len(code.Code) > 32) // secret length > 32
}

func TestOAuth2Grant_TableName(t *testing.T) {
	assert.Equal(t, os.Getenv("TABLE_NAME_PREFIX")+"oauth2_grant", new(OAuth2Grant).TableName())
}

func TestGetOAuth2GrantsByUserID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	result, err := GetOAuth2GrantsByUserID(1)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(1), result[0].ID)
	assert.Equal(t, result[0].ApplicationID, result[0].Application.ID)

	result, err = GetOAuth2GrantsByUserID(34134)
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestRevokeOAuth2Grant(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	assert.NoError(t, RevokeOAuth2Grant(1, 1))
	AssertNotExistsBean(t, &OAuth2Grant{ID: 1, UserID: 1})
}

//////////////////// Authorization Code

func TestGetOAuth2AuthorizationByCode(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	code, err := GetOAuth2AuthorizationByCode("authcode")
	assert.NoError(t, err)
	assert.NotNil(t, code)
	assert.Equal(t, "authcode", code.Code)
	assert.Equal(t, int64(1), code.ID)

	code, err = GetOAuth2AuthorizationByCode("does not exist")
	assert.NoError(t, err)
	assert.Nil(t, code)
}

func TestOAuth2AuthorizationCode_ValidateCodeChallenge(t *testing.T) {
	// test plain
	code := &OAuth2AuthorizationCode{
		CodeChallengeMethod: "plain",
		CodeChallenge:       "test123",
	}
	assert.True(t, code.ValidateCodeChallenge("test123"))
	assert.False(t, code.ValidateCodeChallenge("ierwgjoergjio"))

	// test S256
	code = &OAuth2AuthorizationCode{
		CodeChallengeMethod: "S256",
		CodeChallenge:       "CjvyTLSdR47G5zYenDA-eDWW4lRrO8yvjcWwbD_deOg",
	}
	assert.True(t, code.ValidateCodeChallenge("N1Zo9-8Rfwhkt68r1r29ty8YwIraXR8eh_1Qwxg7yQXsonBt"))
	assert.False(t, code.ValidateCodeChallenge("wiogjerogorewngoenrgoiuenorg"))

	// test unknown
	code = &OAuth2AuthorizationCode{
		CodeChallengeMethod: "monkey",
		CodeChallenge:       "foiwgjioriogeiogjerger",
	}
	assert.False(t, code.ValidateCodeChallenge("foiwgjioriogeiogjerger"))

	// test no code challenge
	code = &OAuth2AuthorizationCode{
		CodeChallengeMethod: "",
		CodeChallenge:       "foierjiogerogerg",
	}
	assert.True(t, code.ValidateCodeChallenge(""))
}

func TestOAuth2AuthorizationCode_GenerateRedirectURI(t *testing.T) {
	code := &OAuth2AuthorizationCode{
		RedirectURI: "https://example.com/callback",
		Code:        "thecode",
	}

	redirect, err := code.GenerateRedirectURI("thestate")
	assert.NoError(t, err)
	assert.Equal(t, redirect.String(), "https://example.com/callback?code=thecode&state=thestate")

	redirect, err = code.GenerateRedirectURI("")
	assert.NoError(t, err)
	assert.Equal(t, redirect.String(), "https://example.com/callback?code=thecode")
}

func TestOAuth2AuthorizationCode_Invalidate(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	code := AssertExistsAndLoadBean(t, &OAuth2AuthorizationCode{Code: "authcode"}).(*OAuth2AuthorizationCode)
	assert.NoError(t, code.Invalidate())
	AssertNotExistsBean(t, &OAuth2AuthorizationCode{Code: "authcode"})
}

func TestOAuth2AuthorizationCode_TableName(t *testing.T) {
	assert.Equal(t, os.Getenv("TABLE_NAME_PREFIX")+"oauth2_authorization_code", new(OAuth2AuthorizationCode).TableName())
}
