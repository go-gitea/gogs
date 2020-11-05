// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplSettingsApplications base.TplName = "user/settings/applications"
)

// Applications render manage access token page
func Applications(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	loadApplicationsData(ctx)

	ctx.HTML(200, tplSettingsApplications)
}

// ApplicationsPost response for add user's access token
func ApplicationsPost(ctx *context.Context, form auth.NewAccessTokenForm) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsApplications"] = true

	if ctx.HasError() {
		loadApplicationsData(ctx)

		ctx.HTML(200, tplSettingsApplications)
		return
	}

	if !setting.API.EnableSwagger {
		ctx.ServerError("AccessToken", fmt.Errorf("cannot modify access tokens; swagger disabled"))
		return
	}

	t := &models.AccessToken{
		UID:  ctx.User.ID,
		Name: form.Name,
	}

	exist, err := models.AccessTokenByNameExists(t)
	if err != nil {
		ctx.ServerError("AccessTokenByNameExists", err)
		return
	}
	if exist {
		ctx.Flash.Error(ctx.Tr("settings.generate_token_name_duplicate", t.Name))
		ctx.Redirect(setting.AppSubURL + "/user/settings/applications")
		return
	}

	if err := models.NewAccessToken(t); err != nil {
		ctx.ServerError("NewAccessToken", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("settings.generate_token_success"))
	ctx.Flash.Info(t.Token)

	ctx.Redirect(setting.AppSubURL + "/user/settings/applications")
}

// DeleteApplication response for delete user access token
func DeleteApplication(ctx *context.Context) {
	if !setting.API.EnableSwagger {
		ctx.ServerError("DeleteAccessToken", fmt.Errorf("cannot delete access token; swagger disabled"))
		return
	}

	if err := models.DeleteAccessTokenByID(ctx.QueryInt64("id"), ctx.User.ID); err != nil {
		ctx.Flash.Error("DeleteAccessTokenByID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("settings.delete_token_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/applications",
	})
}

func loadApplicationsData(ctx *context.Context) {
	if setting.API.EnableSwagger {
		tokens, err := models.ListAccessTokens(models.ListAccessTokensOptions{UserID: ctx.User.ID})
		if err != nil {
			ctx.ServerError("ListAccessTokens", err)
			return
		}
		ctx.Data["Tokens"] = tokens
	}
	ctx.Data["EnableOAuth2"] = setting.OAuth2.Enable
	if setting.OAuth2.Enable {
		var err error
		ctx.Data["Applications"], err = models.GetOAuth2ApplicationsByUserID(ctx.User.ID)
		if err != nil {
			ctx.ServerError("GetOAuth2ApplicationsByUserID", err)
			return
		}
		ctx.Data["Grants"], err = models.GetOAuth2GrantsByUserID(ctx.User.ID)
		if err != nil {
			ctx.ServerError("GetOAuth2GrantsByUserID", err)
			return
		}
	}
}
