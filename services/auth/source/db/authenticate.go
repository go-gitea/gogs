// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
)

// Authenticate authenticates the provided user against the DB
func Authenticate(user *models.User, login, password string) (*models.User, error) {
	if user == nil {
		return nil, models.ErrUserNotExist{Name: login}
	}

	if !user.IsPasswordSet() || !user.ValidatePassword(password) {
		return nil, models.ErrUserNotExist{UID: user.ID, Name: user.Name}
	}

	// Update password hash if server password hash algorithm have changed
	if user.PasswdHashAlgo != setting.PasswordHashAlgo {
		if err := user.SetPassword(password); err != nil {
			return nil, err
		}
		if err := models.UpdateUserCols(user, "passwd", "passwd_hash_algo", "salt"); err != nil {
			return nil, err
		}
	}

	// WARN: DON'T check user.IsActive, that will be checked on reqSign so that
	// user could be hint to resend confirm email.
	if user.ProhibitLogin {
		return nil, models.ErrUserProhibitLogin{
			UID:  user.ID,
			Name: user.Name,
		}
	}

	return user, nil
}
