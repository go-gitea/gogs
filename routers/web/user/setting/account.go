// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"errors"
	"net/http"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/password"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/mailer"
)

const (
	tplSettingsAccount base.TplName = "user/settings/account"
)

// Account renders change user's password, user's email and user suicide page
func Account(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsAccount"] = true
	ctx.Data["Email"] = ctx.User.Email

	loadAccountData(ctx)

	ctx.HTML(http.StatusOK, tplSettingsAccount)
}

// AccountPost response for change user's password
func AccountPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.ChangePasswordForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsAccount"] = true

	if ctx.HasError() {
		loadAccountData(ctx)

		ctx.HTML(http.StatusOK, tplSettingsAccount)
		return
	}

	if len(form.Password) < setting.MinPasswordLength {
		ctx.Flash.Error(ctx.Tr("auth.password_too_short", setting.MinPasswordLength))
	} else if ctx.User.IsPasswordSet() && !ctx.User.ValidatePassword(form.OldPassword) {
		ctx.Flash.Error(ctx.Tr("settings.password_incorrect"))
	} else if form.Password != form.Retype {
		ctx.Flash.Error(ctx.Tr("form.password_not_match"))
	} else if !password.IsComplexEnough(form.Password) {
		ctx.Flash.Error(password.BuildComplexityError(ctx))
	} else if pwned, err := password.IsPwned(ctx, form.Password); pwned || err != nil {
		errMsg := ctx.Tr("auth.password_pwned")
		if err != nil {
			log.Error(err.Error())
			errMsg = ctx.Tr("auth.password_pwned_err")
		}
		ctx.Flash.Error(errMsg)
	} else {
		var err error
		if err = ctx.User.SetPassword(form.Password); err != nil {
			ctx.ServerError("UpdateUser", err)
			return
		}
		if err := models.UpdateUserCols(ctx.User, "salt", "passwd_hash_algo", "passwd"); err != nil {
			ctx.ServerError("UpdateUser", err)
			return
		}
		log.Trace("User password updated: %s", ctx.User.Name)
		ctx.Flash.Success(ctx.Tr("settings.change_password_success"))
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings/account")
}

// EmailPost response for change user's email
func EmailPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AddEmailForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsAccount"] = true

	// Make emailaddress primary.
	if ctx.Query("_method") == "PRIMARY" {
		if err := models.MakeEmailPrimary(&models.EmailAddress{ID: ctx.QueryInt64("id")}); err != nil {
			ctx.ServerError("MakeEmailPrimary", err)
			return
		}

		log.Trace("Email made primary: %s", ctx.User.Name)
		ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		return
	}
	// Send activation Email
	if ctx.Query("_method") == "SENDACTIVATION" {
		var address string
		if ctx.Cache.IsExist("MailResendLimit_" + ctx.User.LowerName) {
			log.Error("Send activation: activation still pending")
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
			return
		}

		id := ctx.QueryInt64("id")
		email, err := models.GetEmailAddressByID(ctx.User.ID, id)
		if err != nil {
			log.Error("GetEmailAddressByID(%d,%d) error: %v", ctx.User.ID, id, err)
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
			return
		}
		if email == nil {
			log.Warn("Send activation failed: EmailAddress[%d] not found for user: %-v", id, ctx.User)
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
			return
		}
		if email.IsActivated {
			log.Debug("Send activation failed: email %s is already activated for user: %-v", email.Email, ctx.User)
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
			return
		}
		if email.IsPrimary {
			if ctx.User.IsActive && !setting.Service.RegisterEmailConfirm {
				log.Debug("Send activation failed: email %s is already activated for user: %-v", email.Email, ctx.User)
				ctx.Redirect(setting.AppSubURL + "/user/settings/account")
				return
			}
			// Only fired when the primary email is inactive (Wrong state)
			mailer.SendActivateAccountMail(ctx.Locale, ctx.User)
		} else {
			mailer.SendActivateEmailMail(ctx.User, email)
		}
		address = email.Email

		if err := ctx.Cache.Put("MailResendLimit_"+ctx.User.LowerName, ctx.User.LowerName, 180); err != nil {
			log.Error("Set cache(MailResendLimit) fail: %v", err)
		}
		ctx.Flash.Info(ctx.Tr("settings.add_email_confirmation_sent", address, timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale.Language())))
		ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		return
	}
	// Set Email Notification Preference
	if ctx.Query("_method") == "NOTIFICATION" {
		preference := ctx.Query("preference")
		if !(preference == models.EmailNotificationsEnabled ||
			preference == models.EmailNotificationsOnMention ||
			preference == models.EmailNotificationsDisabled) {
			log.Error("Email notifications preference change returned unrecognized option %s: %s", preference, ctx.User.Name)
			ctx.ServerError("SetEmailPreference", errors.New("option unrecognized"))
			return
		}
		if err := ctx.User.SetEmailNotifications(preference); err != nil {
			log.Error("Set Email Notifications failed: %v", err)
			ctx.ServerError("SetEmailNotifications", err)
			return
		}
		log.Trace("Email notifications preference made %s: %s", preference, ctx.User.Name)
		ctx.Flash.Success(ctx.Tr("settings.email_preference_set_success"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		return
	}

	if ctx.HasError() {
		loadAccountData(ctx)

		ctx.HTML(http.StatusOK, tplSettingsAccount)
		return
	}

	email := &models.EmailAddress{
		UID:         ctx.User.ID,
		Email:       form.Email,
		IsActivated: !setting.Service.RegisterEmailConfirm,
	}
	if err := models.AddEmailAddress(email); err != nil {
		if models.IsErrEmailAlreadyUsed(err) {
			loadAccountData(ctx)

			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplSettingsAccount, &form)
			return
		} else if models.IsErrEmailInvalid(err) {
			loadAccountData(ctx)

			ctx.RenderWithErr(ctx.Tr("form.email_invalid"), tplSettingsAccount, &form)
			return
		}
		ctx.ServerError("AddEmailAddress", err)
		return
	}

	// Send confirmation email
	if setting.Service.RegisterEmailConfirm {
		mailer.SendActivateEmailMail(ctx.User, email)
		if err := ctx.Cache.Put("MailResendLimit_"+ctx.User.LowerName, ctx.User.LowerName, 180); err != nil {
			log.Error("Set cache(MailResendLimit) fail: %v", err)
		}
		ctx.Flash.Info(ctx.Tr("settings.add_email_confirmation_sent", email.Email, timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, ctx.Locale.Language())))
	} else {
		ctx.Flash.Success(ctx.Tr("settings.add_email_success"))
	}

	log.Trace("Email address added: %s", email.Email)
	ctx.Redirect(setting.AppSubURL + "/user/settings/account")
}

// DeleteEmail response for delete user's email
func DeleteEmail(ctx *context.Context) {
	if err := models.DeleteEmailAddress(&models.EmailAddress{ID: ctx.QueryInt64("id"), UID: ctx.User.ID}); err != nil {
		ctx.ServerError("DeleteEmail", err)
		return
	}
	log.Trace("Email address deleted: %s", ctx.User.Name)

	ctx.Flash.Success(ctx.Tr("settings.email_deletion_success"))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/user/settings/account",
	})
}

// DeleteAccount render user suicide page and response for delete user himself
func DeleteAccount(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsAccount"] = true

	if _, err := auth.UserSignIn(ctx.User.Name, ctx.Query("password")); err != nil {
		if models.IsErrUserNotExist(err) {
			loadAccountData(ctx)

			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_password"), tplSettingsAccount, nil)
		} else {
			ctx.ServerError("UserSignIn", err)
		}
		return
	}

	if err := models.DeleteUser(ctx.User); err != nil {
		switch {
		case models.IsErrUserOwnRepos(err):
			ctx.Flash.Error(ctx.Tr("form.still_own_repo"))
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		case models.IsErrUserHasOrgs(err):
			ctx.Flash.Error(ctx.Tr("form.still_has_org"))
			ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		default:
			ctx.ServerError("DeleteUser", err)
		}
	} else {
		log.Trace("Account deleted: %s", ctx.User.Name)
		ctx.Redirect(setting.AppSubURL + "/")
	}
}

// UpdateUIThemePost is used to update users' specific theme
func UpdateUIThemePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.UpdateThemeForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsAccount"] = true

	if ctx.HasError() {
		ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		return
	}

	if !form.IsThemeExists() {
		ctx.Flash.Error(ctx.Tr("settings.theme_update_error"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		return
	}

	if err := ctx.User.UpdateTheme(form.Theme); err != nil {
		ctx.Flash.Error(ctx.Tr("settings.theme_update_error"))
		ctx.Redirect(setting.AppSubURL + "/user/settings/account")
		return
	}

	log.Trace("Update user theme: %s", ctx.User.Name)
	ctx.Flash.Success(ctx.Tr("settings.theme_update_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings/account")
}

func loadAccountData(ctx *context.Context) {
	emlist, err := models.GetEmailAddresses(ctx.User.ID)
	if err != nil {
		ctx.ServerError("GetEmailAddresses", err)
		return
	}
	type UserEmail struct {
		models.EmailAddress
		CanBePrimary bool
	}
	pendingActivation := ctx.Cache.IsExist("MailResendLimit_" + ctx.User.LowerName)
	emails := make([]*UserEmail, len(emlist))
	for i, em := range emlist {
		var email UserEmail
		email.EmailAddress = *em
		email.CanBePrimary = em.IsActivated
		emails[i] = &email
	}
	ctx.Data["Emails"] = emails
	ctx.Data["EmailNotificationsPreference"] = ctx.User.EmailNotifications()
	ctx.Data["ActivationsPending"] = pendingActivation
	ctx.Data["CanAddEmails"] = !pendingActivation || !setting.Service.RegisterEmailConfirm

	if setting.Service.UserDeleteWithCommentsMaxTime != 0 {
		ctx.Data["UserDeleteWithCommentsMaxTime"] = setting.Service.UserDeleteWithCommentsMaxTime.String()
		ctx.Data["UserDeleteWithComments"] = ctx.User.CreatedUnix.AsTime().Add(setting.Service.UserDeleteWithCommentsMaxTime).After(time.Now())
	}
}
