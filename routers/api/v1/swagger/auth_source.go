// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import api "code.gitea.io/gitea/modules/structs"

// AuthSourcesList
// swagger:response AuthSourcesList
type swaggerAuthSourcesList struct {
	// in:body
	Body []api.AuthSource `json:"body"`
}

// AuthSource
// swagger:response AuthSource
type swaggerAuthSource struct {
	// in:body
	Body api.AuthSource `json:"body"`
}

// CreateAuthSource
// swagger:response CreateAuthSource
type swaggerCreateAuthSource struct {
	// in:body
	CreateAuthSource api.CreateAuthSource
}
