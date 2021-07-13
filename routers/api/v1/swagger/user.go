// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package swagger

import (
	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// User
// swagger:response User
type swaggerResponseUser struct {
	// in:body
	Body api.User `json:"body"`
}

// UserList
// swagger:response UserList
type swaggerResponseUserList struct {
	// in:body
	Body []api.User `json:"body"`
}

// EmailList
// swagger:response EmailList
type swaggerResponseEmailList struct {
	// in:body
	Body []api.Email `json:"body"`
}

// swagger:model EditUserOption
type swaggerModelEditUserOption struct {
	// in:body
	Options api.EditUserOption
}

// UserHeatmapData
// swagger:response UserHeatmapData
type swaggerResponseUserHeatmapData struct {
	// in:body
	Body []models.UserHeatmapData `json:"body"`
}

// UserSettings
// swagger:response UserSettings
type swaggerResponseUserSettings struct {
	// in:body
	Body []api.UserSettings `json:"body"`
}
