// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"testing"

	"code.gitea.io/gitea/models"
	"github.com/stretchr/testify/assert"
)

func TestUser_ToUser(t *testing.T) {
	user1 := models.AssertExistsAndLoadBean(t, &models.User{ID: 1, IsAdmin: true}).(*models.User)

	apiUser := ToUser(user1, true, true)
	assert.True(t, apiUser.IsAdmin)

	user2 := models.AssertExistsAndLoadBean(t, &models.User{ID: 2, IsAdmin: false}).(*models.User)

	apiUser = ToUser(user2, true, true)
	assert.False(t, apiUser.IsAdmin)

	apiUser = ToUser(user1, false, false)
	assert.False(t, apiUser.IsAdmin)
}
