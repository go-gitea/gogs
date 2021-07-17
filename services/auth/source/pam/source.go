// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pam

import (
	"code.gitea.io/gitea/models"

	jsoniter "github.com/json-iterator/go"
)

// __________  _____      _____
// \______   \/  _  \    /     \
//  |     ___/  /_\  \  /  \ /  \
//  |    |  /    |    \/    Y    \
//  |____|  \____|__  /\____|__  /
//                  \/         \/

// Source holds configuration for the PAM login source.
type Source struct {
	ServiceName string // pam service (e.g. system-auth)
	EmailDomain string

	// reference to the loginSource
	loginSource *models.LoginSource
}

// FromDB fills up a PAMConfig from serialized format.
func (source *Source) FromDB(bs []byte) error {
	return models.JSONUnmarshalHandleDoubleEncode(bs, &source)
}

// ToDB exports a PAMConfig to a serialized format.
func (source *Source) ToDB() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(source)
}

// SetLoginSource sets the related LoginSource
func (source *Source) SetLoginSource(loginSource *models.LoginSource) {
	source.loginSource = loginSource
}

func init() {
	models.RegisterLoginTypeConfig(models.LoginPAM, &Source{})
}
