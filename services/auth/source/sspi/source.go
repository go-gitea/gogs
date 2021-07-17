// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sspi

import (
	"code.gitea.io/gitea/models"
	jsoniter "github.com/json-iterator/go"
)

//   _________ ___________________.___
//  /   _____//   _____/\______   \   |
//  \_____  \ \_____  \  |     ___/   |
//  /        \/        \ |    |   |   |
// /_______  /_______  / |____|   |___|
//         \/        \/

// Source holds configuration for SSPI single sign-on.
type Source struct {
	AutoCreateUsers      bool
	AutoActivateUsers    bool
	StripDomainNames     bool
	SeparatorReplacement string
	DefaultLanguage      string
}

// FromDB fills up an SSPIConfig from serialized format.
func (cfg *Source) FromDB(bs []byte) error {
	return models.JSONUnmarshalHandleDoubleEncode(bs, &cfg)
}

// ToDB exports an SSPIConfig to a serialized format.
func (cfg *Source) ToDB() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	return json.Marshal(cfg)
}

func init() {
	models.RegisterLoginTypeConfig(models.LoginSSPI, &Source{})
}
