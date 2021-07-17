// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/auth/source/oauth2"
)

const (
	tplSettingsSecurity    base.TplName = "user/settings/security"
	tplSettingsTwofaEnroll base.TplName = "user/settings/twofa_enroll"
)

// Security render change user's password page and 2FA
func Security(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsSecurity"] = true
	ctx.Data["RequireU2F"] = true

	if ctx.Query("openid.return_to") != "" {
		settingsOpenIDVerify(ctx)
		return
	}

	loadSecurityData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsSecurity)
}

// DeleteAccountLink delete a single account link
func DeleteAccountLink(ctx *context.Context) {
	id := ctx.QueryInt64("id")
	if id <= 0 {
		ctx.Flash.Error("Account link id is not given")
	} else {
		if _, err := models.RemoveAccountLink(ctx.User, id); err != nil {
			ctx.Flash.Error("RemoveAccountLink: " + err.Error())
		} else {
			ctx.Flash.Success(ctx.Tr("settings.remove_account_link_success"))
		}
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/security",
	})
}

func loadSecurityData(ctx *context.Context) {
	enrolled := true
	_, err := models.GetTwoFactorByUID(ctx.User.ID)
	if err != nil {
		if models.IsErrTwoFactorNotEnrolled(err) {
			enrolled = false
		} else {
			ctx.ServerError("SettingsTwoFactor", err)
			return
		}
	}
	ctx.Data["TwofaEnrolled"] = enrolled
	if enrolled {
		ctx.Data["U2FRegistrations"], err = models.GetU2FRegistrationsByUID(ctx.User.ID)
		if err != nil {
			ctx.ServerError("GetU2FRegistrationsByUID", err)
			return
		}
	}

	tokens, err := models.ListAccessTokens(models.ListAccessTokensOptions{UserID: ctx.User.ID})
	if err != nil {
		ctx.ServerError("ListAccessTokens", err)
		return
	}
	ctx.Data["Tokens"] = tokens

	accountLinks, err := models.ListAccountLinks(ctx.User)
	if err != nil {
		ctx.ServerError("ListAccountLinks", err)
		return
	}

	// map the provider display name with the LoginSource
	sources := make(map[*models.LoginSource]string)
	for _, externalAccount := range accountLinks {
		if loginSource, err := models.GetLoginSourceByID(externalAccount.LoginSourceID); err == nil {
			var providerDisplayName string
			if loginSource.IsOAuth2() {
				providerTechnicalName := loginSource.Cfg.(*oauth2.Source).Provider
				providerDisplayName = oauth2.Providers[providerTechnicalName].DisplayName
			} else {
				providerDisplayName = loginSource.Name
			}
			sources[loginSource] = providerDisplayName
		}
	}
	ctx.Data["AccountLinks"] = sources

	openid, err := models.GetUserOpenIDs(ctx.User.ID)
	if err != nil {
		ctx.ServerError("GetUserOpenIDs", err)
		return
	}
	ctx.Data["OpenIDs"] = openid
}
