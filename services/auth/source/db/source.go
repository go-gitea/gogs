// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import "code.gitea.io/gitea/models"

// Source is a password authentication service
type Source struct{}

// FromDB fills up an OAuth2Config from serialized format.
func (source *Source) FromDB(bs []byte) error {
	return nil
}

// ToDB exports an SMTPConfig to a serialized format.
func (source *Source) ToDB() ([]byte, error) {
	return nil, nil
}

// Authenticate queries if login/password is valid against the PAM,
// and create a local user if success when enabled.
func (source *Source) Authenticate(user *models.User, login, password string) (*models.User, error) {
	return Authenticate(user, login, password)
}

func init() {
	models.RegisterLoginTypeConfig(models.LoginNoType, &Source{})
	models.RegisterLoginTypeConfig(models.LoginPlain, &Source{})
}
