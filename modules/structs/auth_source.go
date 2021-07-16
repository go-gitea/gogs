// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"encoding/json"
	"time"
)

// AuthSource represents an authentication source
type AuthSource struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// enum: LDAP (via BindDN),LDAP (simple auth),SMTP,PAM,OAuth2,SPNEGO with SSPI
	Type          string `json:"type"`
	IsActive      bool   `json:"isActive"`
	IsSyncEnabled bool   `json:"isSyncEnabled"`
	// swagger:strfmt date-time
	CreatedTime time.Time `json:"createdTime"`
	// swagger:strfmt date-time
	UpdatedTime time.Time       `json:"updatedTime"`
	Cfg         json.RawMessage `json:"config"`
}

// CreateAuthSource represents an authentication source creation request
type CreateAuthSource struct {
	// required: true
	Name string `json:"name" binding:"Required"`
	// required: true
	// enum: LDAP (via BindDN),LDAP (simple auth),SMTP,PAM,OAuth2,SPNEGO with SSPI
	Type          string `json:"type" binding:"Required"`
	IsActive      bool   `json:"isActive"`
	IsSyncEnabled bool   `json:"isSyncEnabled"`
	// required: true
	Cfg json.RawMessage `json:"config" binding:"Required"`
}
