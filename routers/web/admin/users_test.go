// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"

	"github.com/stretchr/testify/assert"
)

func TestNewUserPost_MustChangePassword(t *testing.T) {

	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "admin/users/new")

	u := models.AssertExistsAndLoadBean(t, &models.User{
		IsAdmin: true,
		ID:      2,
	}).(*models.User)

	ctx.User = u

	username := "gitea"
	email := "gitea@gitea.io"

	form := forms.AdminCreateUserForm{
		LoginType:          "local",
		LoginName:          "local",
		UserName:           username,
		Email:              email,
		Password:           "abc123ABC!=$",
		SendNotify:         false,
		MustChangePassword: true,
	}

	web.SetForm(ctx, &form)
	NewUserPost(ctx)

	assert.NotEmpty(t, ctx.Flash.SuccessMsg)

	u, err := models.GetUserByName(username)

	assert.NoError(t, err)
	assert.Equal(t, username, u.Name)
	assert.Equal(t, email, u.Email)
	assert.True(t, u.MustChangePassword)
}

func TestNewUserPost_MustChangePasswordFalse(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "admin/users/new")

	u := models.AssertExistsAndLoadBean(t, &models.User{
		IsAdmin: true,
		ID:      2,
	}).(*models.User)

	ctx.User = u

	username := "gitea"
	email := "gitea@gitea.io"

	form := forms.AdminCreateUserForm{
		LoginType:          "local",
		LoginName:          "local",
		UserName:           username,
		Email:              email,
		Password:           "abc123ABC!=$",
		SendNotify:         false,
		MustChangePassword: false,
	}

	web.SetForm(ctx, &form)
	NewUserPost(ctx)

	assert.NotEmpty(t, ctx.Flash.SuccessMsg)

	u, err := models.GetUserByName(username)

	assert.NoError(t, err)
	assert.Equal(t, username, u.Name)
	assert.Equal(t, email, u.Email)
	assert.False(t, u.MustChangePassword)
}

func TestNewUserPost_InvalidEmail(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "admin/users/new")

	u := models.AssertExistsAndLoadBean(t, &models.User{
		IsAdmin: true,
		ID:      2,
	}).(*models.User)

	ctx.User = u

	username := "gitea"
	email := "gitea@gitea.io\r\n"

	form := forms.AdminCreateUserForm{
		LoginType:          "local",
		LoginName:          "local",
		UserName:           username,
		Email:              email,
		Password:           "abc123ABC!=$",
		SendNotify:         false,
		MustChangePassword: false,
	}

	web.SetForm(ctx, &form)
	NewUserPost(ctx)

	assert.NotEmpty(t, ctx.Flash.ErrorMsg)
}

func TestNewUserPost_VisibilityDefaultPublic(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "admin/users/new")

	u := models.AssertExistsAndLoadBean(t, &models.User{
		IsAdmin: true,
		ID:      2,
	}).(*models.User)

	ctx.User = u

	username := "gitea"
	email := "gitea@gitea.io"

	form := forms.AdminCreateUserForm{
		LoginType:          "local",
		LoginName:          "local",
		UserName:           username,
		Email:              email,
		Password:           "abc123ABC!=$",
		SendNotify:         false,
		MustChangePassword: false,
	}

	web.SetForm(ctx, &form)
	NewUserPost(ctx)

	assert.NotEmpty(t, ctx.Flash.SuccessMsg)

	u, err := models.GetUserByName(username)

	assert.NoError(t, err)
	assert.Equal(t, username, u.Name)
	assert.Equal(t, email, u.Email)
	// As default user visibility
	assert.Equal(t, setting.Service.DefaultUserVisibilityMode, u.Visibility)
}

func TestNewUserPost_VisibilityPrivate(t *testing.T) {
	models.PrepareTestEnv(t)
	ctx := test.MockContext(t, "admin/users/new")

	u := models.AssertExistsAndLoadBean(t, &models.User{
		IsAdmin: true,
		ID:      2,
	}).(*models.User)

	ctx.User = u

	username := "gitea"
	email := "gitea@gitea.io"

	form := forms.AdminCreateUserForm{
		LoginType:          "local",
		LoginName:          "local",
		UserName:           username,
		Email:              email,
		Password:           "abc123ABC!=$",
		SendNotify:         false,
		MustChangePassword: false,
		Visibility:         api.VisibleTypePrivate,
	}

	web.SetForm(ctx, &form)
	NewUserPost(ctx)

	assert.NotEmpty(t, ctx.Flash.SuccessMsg)

	u, err := models.GetUserByName(username)

	assert.NoError(t, err)
	assert.Equal(t, username, u.Name)
	assert.Equal(t, email, u.Email)
	// As default user visibility
	assert.True(t, u.Visibility.IsPrivate())
}
