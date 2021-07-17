// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package smtp

import (
	"code.gitea.io/gitea/models"

	jsoniter "github.com/json-iterator/go"
)

//   _________   __________________________
//  /   _____/  /     \__    ___/\______   \
//  \_____  \  /  \ /  \|    |    |     ___/
//  /        \/    Y    \    |    |    |
// /_______  /\____|__  /____|    |____|
//         \/         \/

// Source holds configuration for the SMTP login source.
type Source struct {
	Auth           string
	Host           string
	Port           int
	AllowedDomains string `xorm:"TEXT"`
	TLS            bool
	SkipVerify     bool

	// reference to the loginSource
	loginSource *models.LoginSource
}

// FromDB fills up an SMTPConfig from serialized format.
func (source *Source) FromDB(bs []byte) error {
	return models.JSONUnmarshalHandleDoubleEncode(bs, &source)
}

// ToDB exports an SMTPConfig to a serialized format.
func (source *Source) ToDB() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(source)
}

// IsSkipVerify returns if SkipVerify is set
func (source *Source) IsSkipVerify() bool {
	return source.SkipVerify
}

// HasTLS returns true for SMTP
func (source *Source) HasTLS() bool {
	return true
}

// UseTLS returns if TLS is set
func (source *Source) UseTLS() bool {
	return source.TLS
}

// SetLoginSource sets the related LoginSource
func (source *Source) SetLoginSource(loginSource *models.LoginSource) {
	source.loginSource = loginSource
}

func init() {
	models.RegisterLoginTypeConfig(models.LoginSMTP, &Source{})
}
