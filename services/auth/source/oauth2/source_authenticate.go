// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/services/auth/source/db"
)

// Authenticate falls back to the db authenticator
func (source *Source) Authenticate(user *models.User, login, password string) (*models.User, error) {
	return db.Authenticate(user, login, password)
}
