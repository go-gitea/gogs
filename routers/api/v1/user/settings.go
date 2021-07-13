// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package user

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
)

// GetUserSettings returns user settings
func GetUserSettings(ctx *context.APIContext) {
	// swagger:operation GET /user/settings user getUserSettings
	// ---
	// summary: Get user settings
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserSettings"
	ctx.JSON(http.StatusOK, convert.User2UserSettings(ctx.User))
}

// UpdateUserSettings returns user settings
func UpdateUserSettings(ctx *context.APIContext) {
	// swagger:operation PATCH /user/settings user updateUserSettings
	// ---
	// summary: Update user settings
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/UserSettingsOptions"
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/UserSettings"

	form := web.GetForm(ctx).(*api.UserSettingsOptions)

	if form.FullName != nil {
		ctx.User.FullName = *form.FullName
	}
	if form.Description != nil {
		ctx.User.Description = *form.Description
	}
	if form.Website != nil {
		ctx.User.Website = *form.Website
	}
	if form.Location != nil {
		ctx.User.Location = *form.Location
	}
	if form.Language != nil {
		ctx.User.Language = *form.Language
	}
	if form.Theme != nil {
		ctx.User.Theme = *form.Theme
	}
	if form.DiffViewStyle != nil {
		ctx.User.DiffViewStyle = *form.DiffViewStyle
	}

	if form.HideEmail != nil {
		ctx.User.KeepEmailPrivate = *form.HideEmail
	}
	if form.HideActivity != nil {
		ctx.User.KeepActivityPrivate = *form.HideActivity
	}

	if err := models.UpdateUser(ctx.User); err != nil {
		ctx.InternalServerError(err)
		return
	}

	ctx.JSON(http.StatusOK, convert.User2UserSettings(ctx.User))
}
