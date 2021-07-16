// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"

	xorm "xorm.io/xorm/convert"
)

// ListAuthSources returns list of existing authentication sources
func ListAuthSources(ctx *context.APIContext) {
	// swagger:operation GET /admin/auths admin adminAuthsSourcesList
	// ---
	// summary: List existing authentication sources
	// produces:
	// - application/json
	// responses:
	//   "200":
	//     "$ref": "#/responses/AuthSourcesList"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	sources, err := models.LoginSources()
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	result, err := convert.ToAuthSources(sources)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusOK, result)
}

// CreateAuthSource creates new authentication source
func CreateAuthSource(ctx *context.APIContext) {
	// swagger:operation POST /admin/auths admin adminCreateAuthSource
	// ---
	// summary: Create new authentication source
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateAuthSource"
	// responses:
	//   "201":
	//     "$ref": "#/responses/AuthSource"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	authSource := web.GetForm(ctx).(*api.CreateAuthSource)
	var config xorm.Conversion
	var loginType models.LoginType
	for key, val := range models.LoginNames {
		if authSource.Type == val {
			loginType = key
			switch key {
			case models.LoginLDAP:
				config = &models.LDAPConfig{}
			case models.LoginSMTP:
				config = &models.SMTPConfig{}
			case models.LoginPAM:
				config = &models.PAMConfig{}
			case models.LoginDLDAP:
				config = &models.LDAPConfig{}
			case models.LoginOAuth2:
				config = &models.OAuth2Config{}
			case models.LoginSSPI:
				config = &models.SSPIConfig{}
			}
			break
		}
	}
	if loginType == 0 {
		ctx.Error(http.StatusBadRequest, "", "Authentication source type is invalid")
		return
	}
	if err := config.FromDB(authSource.Cfg); err != nil {
		ctx.InternalServerError(err)
		return
	}

	source := &models.LoginSource{
		Type:          loginType,
		Cfg:           config,
		Name:          authSource.Name,
		IsActived:     authSource.IsActive,
		IsSyncEnabled: authSource.IsSyncEnabled,
		CreatedUnix:   timeutil.TimeStampNow(),
		UpdatedUnix:   timeutil.TimeStampNow(),
	}
	if err := models.CreateLoginSource(source); err != nil {
		ctx.InternalServerError(err)
		return
	}
	result, err := convert.ToAuthSource(source)
	if err != nil {
		ctx.InternalServerError(err)
		return
	}
	ctx.JSON(http.StatusCreated, result)
}
