// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestUser_ToUser(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())

	user1 := models.AssertExistsAndLoadBean(t, &models.User{ID: 1, IsAdmin: true}).(*models.User)

	apiUser := toUser(user1, true, true)
	assert.True(t, apiUser.IsAdmin)

	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2, IsAdmin: false}).(*models.User)

	apiUser = toUser(user2, true, true)
	assert.False(t, apiUser.IsAdmin)

	apiUser = toUser(user1, false, false)
	assert.False(t, apiUser.IsAdmin)
	assert.EqualValues(t, api.VisibleTypePublic.String(), apiUser.Visibility)

	user31 := models.AssertExistsAndLoadBean(t, &models.User{ID: 31, IsAdmin: false, Visibility: api.VisibleTypePrivate}).(*models.User)

	apiUser = toUser(user31, true, true)
	assert.False(t, apiUser.IsAdmin)
	assert.EqualValues(t, api.VisibleTypePrivate.String(), apiUser.Visibility)
}
