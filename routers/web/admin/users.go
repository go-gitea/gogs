// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/password"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/explore"
	router_user_setting "code.gitea.io/gitea/routers/web/user/setting"
	"code.gitea.io/gitea/services/forms"
	"code.gitea.io/gitea/services/mailer"
)

const (
	tplUsers    base.TplName = "admin/user/list"
	tplUserNew  base.TplName = "admin/user/new"
	tplUserEdit base.TplName = "admin/user/edit"
)

// Users show all the users
func Users(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true

	explore.RenderUserSearch(ctx, &models.SearchUserOptions{
		Actor: ctx.User,
		Type:  models.UserTypeIndividual,
		ListOptions: models.ListOptions{
			PageSize: setting.UI.Admin.UserPagingNum,
		},
		SearchByEmail: true,
	}, tplUsers)
}

// NewUser render adding a new user page
func NewUser(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users.new_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true
	ctx.Data["DefaultUserVisibilityMode"] = setting.Service.DefaultUserVisibilityMode
	ctx.Data["AllowedUserVisibilityModes"] = setting.Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice()

	ctx.Data["login_type"] = "0-0"

	sources, err := models.LoginSources()
	if err != nil {
		ctx.ServerError("LoginSources", err)
		return
	}
	ctx.Data["Sources"] = sources

	ctx.Data["CanSendEmail"] = setting.MailService != nil
	ctx.HTML(http.StatusOK, tplUserNew)
}

// NewUserPost response for adding a new user
func NewUserPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminCreateUserForm)
	ctx.Data["Title"] = ctx.Tr("admin.users.new_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true
	ctx.Data["DefaultUserVisibilityMode"] = setting.Service.DefaultUserVisibilityMode

	sources, err := models.LoginSources()
	if err != nil {
		ctx.ServerError("LoginSources", err)
		return
	}
	ctx.Data["Sources"] = sources

	ctx.Data["CanSendEmail"] = setting.MailService != nil

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplUserNew)
		return
	}

	u := &models.User{
		Name:      form.UserName,
		Email:     form.Email,
		Passwd:    form.Password,
		IsActive:  true,
		LoginType: models.LoginPlain,
	}

	if len(form.LoginType) > 0 {
		fields := strings.Split(form.LoginType, "-")
		if len(fields) == 2 {
			lType, _ := strconv.ParseInt(fields[0], 10, 0)
			u.LoginType = models.LoginType(lType)
			u.LoginSource, _ = strconv.ParseInt(fields[1], 10, 64)
			u.LoginName = form.LoginName
		}
	}
	if u.LoginType == models.LoginNoType || u.LoginType == models.LoginPlain {
		if len(form.Password) < setting.MinPasswordLength {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplUserNew, &form)
			return
		}
		if !password.IsComplexEnough(form.Password) {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(password.BuildComplexityError(ctx), tplUserNew, &form)
			return
		}
		pwned, err := password.IsPwned(ctx, form.Password)
		if pwned {
			ctx.Data["Err_Password"] = true
			errMsg := ctx.Tr("auth.password_pwned")
			if err != nil {
				log.Error(err.Error())
				errMsg = ctx.Tr("auth.password_pwned_err")
			}
			ctx.RenderWithErr(errMsg, tplUserNew, &form)
			return
		}
		u.MustChangePassword = form.MustChangePassword
	}

	if err := models.CreateUser(u, &models.CreateUserOverwriteOptions{Visibility: form.Visibility}); err != nil {
		switch {
		case models.IsErrUserAlreadyExist(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("form.username_been_taken"), tplUserNew, &form)
		case models.IsErrEmailAlreadyUsed(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplUserNew, &form)
		case models.IsErrEmailInvalid(err):
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_invalid"), tplUserNew, &form)
		case models.IsErrNameReserved(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_reserved", err.(models.ErrNameReserved).Name), tplUserNew, &form)
		case models.IsErrNamePatternNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tplUserNew, &form)
		case models.IsErrNameCharsNotAllowed(err):
			ctx.Data["Err_UserName"] = true
			ctx.RenderWithErr(ctx.Tr("user.form.name_chars_not_allowed", err.(models.ErrNameCharsNotAllowed).Name), tplUserNew, &form)
		default:
			ctx.ServerError("CreateUser", err)
		}
		return
	}
	log.Trace("Account created by admin (%s): %s", ctx.User.Name, u.Name)

	// Send email notification.
	if form.SendNotify {
		mailer.SendRegisterNotifyMail(u)
	}

	ctx.Flash.Success(ctx.Tr("admin.users.new_success", u.Name))
	ctx.Redirect(setting.AppSubURL + "/admin/users/" + fmt.Sprint(u.ID))
}

func prepareUserInfo(ctx *context.Context) *models.User {
	u, err := models.GetUserByID(ctx.ParamsInt64(":userid"))
	if err != nil {
		ctx.ServerError("GetUserByID", err)
		return nil
	}
	ctx.Data["User"] = u

	if u.LoginSource > 0 {
		ctx.Data["LoginSource"], err = models.GetLoginSourceByID(u.LoginSource)
		if err != nil {
			ctx.ServerError("GetLoginSourceByID", err)
			return nil
		}
	} else {
		ctx.Data["LoginSource"] = &models.LoginSource{}
	}

	sources, err := models.LoginSources()
	if err != nil {
		ctx.ServerError("LoginSources", err)
		return nil
	}
	ctx.Data["Sources"] = sources

	ctx.Data["TwoFactorEnabled"] = true
	_, err = models.GetTwoFactorByUID(u.ID)
	if err != nil {
		if !models.IsErrTwoFactorNotEnrolled(err) {
			ctx.ServerError("IsErrTwoFactorNotEnrolled", err)
			return nil
		}
		ctx.Data["TwoFactorEnabled"] = false
	}

	return u
}

// EditUser show editing user page
func EditUser(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.users.edit_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true
	ctx.Data["DisableRegularOrgCreation"] = setting.Admin.DisableRegularOrgCreation
	ctx.Data["DisableMigrations"] = setting.Repository.DisableMigrations
	ctx.Data["AllowedUserVisibilityModes"] = setting.Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice()

	prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	ctx.HTML(http.StatusOK, tplUserEdit)
}

// EditUserPost response for editing user
func EditUserPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AdminEditUserForm)
	ctx.Data["Title"] = ctx.Tr("admin.users.edit_account")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminUsers"] = true
	ctx.Data["DisableMigrations"] = setting.Repository.DisableMigrations

	u := prepareUserInfo(ctx)
	if ctx.Written() {
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplUserEdit)
		return
	}

	fields := strings.Split(form.LoginType, "-")
	if len(fields) == 2 {
		loginType, _ := strconv.ParseInt(fields[0], 10, 0)
		loginSource, _ := strconv.ParseInt(fields[1], 10, 64)

		if u.LoginSource != loginSource {
			u.LoginSource = loginSource
			u.LoginType = models.LoginType(loginType)
		}
	}

	if len(form.Password) > 0 && (u.IsLocal() || u.IsOAuth2()) {
		var err error
		if len(form.Password) < setting.MinPasswordLength {
			ctx.Data["Err_Password"] = true
			ctx.RenderWithErr(ctx.Tr("auth.password_too_short", setting.MinPasswordLength), tplUserEdit, &form)
			return
		}
		if !password.IsComplexEnough(form.Password) {
			ctx.RenderWithErr(password.BuildComplexityError(ctx), tplUserEdit, &form)
			return
		}
		pwned, err := password.IsPwned(ctx, form.Password)
		if pwned {
			ctx.Data["Err_Password"] = true
			errMsg := ctx.Tr("auth.password_pwned")
			if err != nil {
				log.Error(err.Error())
				errMsg = ctx.Tr("auth.password_pwned_err")
			}
			ctx.RenderWithErr(errMsg, tplUserNew, &form)
			return
		}
		if u.Salt, err = models.GetUserSalt(); err != nil {
			ctx.ServerError("UpdateUser", err)
			return
		}
		if err = u.SetPassword(form.Password); err != nil {
			ctx.ServerError("SetPassword", err)
			return
		}
	}

	if len(form.UserName) != 0 && u.Name != form.UserName {
		if err := router_user_setting.HandleUsernameChange(ctx, u, form.UserName); err != nil {
			ctx.Redirect(setting.AppSubURL + "/admin/users")
			return
		}
		u.Name = form.UserName
		u.LowerName = strings.ToLower(form.UserName)
	}

	if form.Reset2FA {
		tf, err := models.GetTwoFactorByUID(u.ID)
		if err != nil && !models.IsErrTwoFactorNotEnrolled(err) {
			ctx.ServerError("GetTwoFactorByUID", err)
			return
		}

		if err = models.DeleteTwoFactorByID(tf.ID, u.ID); err != nil {
			ctx.ServerError("DeleteTwoFactorByID", err)
			return
		}
	}

	u.LoginName = form.LoginName
	u.FullName = form.FullName
	u.Email = form.Email
	u.Website = form.Website
	u.Location = form.Location
	u.MaxRepoCreation = form.MaxRepoCreation
	u.IsActive = form.Active
	u.IsAdmin = form.Admin
	u.IsRestricted = form.Restricted
	u.AllowGitHook = form.AllowGitHook
	u.AllowImportLocal = form.AllowImportLocal
	u.AllowCreateOrganization = form.AllowCreateOrganization

	u.Visibility = form.Visibility

	// skip self Prohibit Login
	if ctx.User.ID == u.ID {
		u.ProhibitLogin = false
	} else {
		u.ProhibitLogin = form.ProhibitLogin
	}

	if err := models.UpdateUser(u); err != nil {
		if models.IsErrEmailAlreadyUsed(err) {
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_been_used"), tplUserEdit, &form)
		} else if models.IsErrEmailInvalid(err) {
			ctx.Data["Err_Email"] = true
			ctx.RenderWithErr(ctx.Tr("form.email_invalid"), tplUserEdit, &form)
		} else {
			ctx.ServerError("UpdateUser", err)
		}
		return
	}
	log.Trace("Account profile updated by admin (%s): %s", ctx.User.Name, u.Name)

	ctx.Flash.Success(ctx.Tr("admin.users.update_profile_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/users/" + ctx.Params(":userid"))
}

// DeleteUser response for deleting a user
func DeleteUser(ctx *context.Context) {
	u, err := models.GetUserByID(ctx.ParamsInt64(":userid"))
	if err != nil {
		ctx.ServerError("GetUserByID", err)
		return
	}

	if err = models.DeleteUser(u); err != nil {
		switch {
		case models.IsErrUserOwnRepos(err):
			ctx.Flash.Error(ctx.Tr("admin.users.still_own_repo"))
			ctx.JSON(http.StatusOK, map[string]interface{}{
				"redirect": setting.AppSubURL + "/admin/users/" + ctx.Params(":userid"),
			})
		case models.IsErrUserHasOrgs(err):
			ctx.Flash.Error(ctx.Tr("admin.users.still_has_org"))
			ctx.JSON(http.StatusOK, map[string]interface{}{
				"redirect": setting.AppSubURL + "/admin/users/" + ctx.Params(":userid"),
			})
		default:
			ctx.ServerError("DeleteUser", err)
		}
		return
	}
	log.Trace("Account deleted by admin (%s): %s", ctx.User.Name, u.Name)

	ctx.Flash.Success(ctx.Tr("admin.users.deletion_success"))
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": setting.AppSubURL + "/admin/users",
	})
}
