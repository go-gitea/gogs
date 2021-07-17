// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

// GetActiveOAuth2ProviderLoginSources returns all actived LoginOAuth2 sources
func GetActiveOAuth2ProviderLoginSources() ([]*LoginSource, error) {
	sources := make([]*LoginSource, 0, 1)
	if err := x.Where("is_active = ? and type = ?", true, LoginOAuth2).Find(&sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// GetActiveOAuth2LoginSourceByName returns a OAuth2 LoginSource based on the given name
func GetActiveOAuth2LoginSourceByName(name string) (*LoginSource, error) {
	loginSource := new(LoginSource)
	has, err := x.Where("name = ? and type = ? and is_active = ?", name, LoginOAuth2, true).Get(loginSource)
	if !has || err != nil {
		return nil, err
	}

	return loginSource, nil
}
