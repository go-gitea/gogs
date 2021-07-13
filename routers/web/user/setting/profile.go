// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/modules/web/middleware"
	"code.gitea.io/gitea/services/forms"

	"github.com/unknwon/i18n"
)

const (
	tplSettingsProfile      base.TplName = "user/settings/profile"
	tplSettingsOrganization base.TplName = "user/settings/organization"
	tplSettingsRepositories base.TplName = "user/settings/repos"
)

// Profile render user's profile page
func Profile(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsProfile"] = true
	ctx.Data["AllowedUserVisibilityModes"] = setting.Service.AllowedUserVisibilityModesSlice.ToVisibleTypeSlice()

	ctx.HTML(http.StatusOK, tplSettingsProfile)
}

// HandleUsernameChange handle username changes from user settings and admin interface
func HandleUsernameChange(ctx *context.Context, user *models.User, newName string) error {
	// Non-local users are not allowed to change their username.
	if !user.IsLocal() {
		ctx.Flash.Error(ctx.Tr("form.username_change_not_local_user"))
		return fmt.Errorf(ctx.Tr("form.username_change_not_local_user"))
	}

	// Check if user name has been changed
	if user.LowerName != strings.ToLower(newName) {
		if err := models.ChangeUserName(user, newName); err != nil {
			switch {
			case models.IsErrUserAlreadyExist(err):
				ctx.Flash.Error(ctx.Tr("form.username_been_taken"))
			case models.IsErrEmailAlreadyUsed(err):
				ctx.Flash.Error(ctx.Tr("form.email_been_used"))
			case models.IsErrNameReserved(err):
				ctx.Flash.Error(ctx.Tr("user.form.name_reserved", newName))
			case models.IsErrNamePatternNotAllowed(err):
				ctx.Flash.Error(ctx.Tr("user.form.name_pattern_not_allowed", newName))
			case models.IsErrNameCharsNotAllowed(err):
				ctx.Flash.Error(ctx.Tr("user.form.name_chars_not_allowed", newName))
			default:
				ctx.ServerError("ChangeUserName", err)
			}
			return err
		}
	} else {
		if err := models.UpdateRepositoryOwnerNames(user.ID, newName); err != nil {
			ctx.ServerError("UpdateRepository", err)
			return err
		}
	}
	log.Trace("User name changed: %s -> %s", user.Name, newName)
	return nil
}

// ProfilePost response for change user's profile
func ProfilePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.UpdateProfileForm)
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsProfile"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplSettingsProfile)
		return
	}

	if len(form.Name) != 0 && ctx.User.Name != form.Name {
		log.Debug("Changing name for %s to %s", ctx.User.Name, form.Name)
		if err := HandleUsernameChange(ctx, ctx.User, form.Name); err != nil {
			ctx.Redirect(setting.AppSubURL + "/user/settings")
			return
		}
		ctx.User.Name = form.Name
		ctx.User.LowerName = strings.ToLower(form.Name)
	}

	ctx.User.FullName = form.FullName
	ctx.User.KeepEmailPrivate = form.KeepEmailPrivate
	ctx.User.Website = form.Website
	ctx.User.Location = form.Location
	if len(form.Language) != 0 {
		if !util.IsStringInSlice(form.Language, setting.Langs) {
			ctx.Flash.Error(ctx.Tr("settings.update_language_not_found", form.Language))
			ctx.Redirect(setting.AppSubURL + "/user/settings")
			return
		}
		ctx.User.Language = form.Language
	}
	ctx.User.Description = form.Description
	ctx.User.KeepActivityPrivate = form.KeepActivityPrivate
	ctx.User.Visibility = form.Visibility
	if err := models.UpdateUserSetting(ctx.User); err != nil {
		if _, ok := err.(models.ErrEmailAlreadyUsed); ok {
			ctx.Flash.Error(ctx.Tr("form.email_been_used"))
			ctx.Redirect(setting.AppSubURL + "/user/settings")
			return
		}
		ctx.ServerError("UpdateUser", err)
		return
	}

	// Update the language to the one we just set
	middleware.SetLocaleCookie(ctx.Resp, ctx.User.Language, 0)

	log.Trace("User settings updated: %s", ctx.User.Name)
	ctx.Flash.Success(i18n.Tr(ctx.User.Language, "settings.update_profile_success"))
	ctx.Redirect(setting.AppSubURL + "/user/settings")
}

// UpdateAvatarSetting update user's avatar
// FIXME: limit size.
func UpdateAvatarSetting(ctx *context.Context, form *forms.AvatarForm, ctxUser *models.User) error {
	ctxUser.UseCustomAvatar = form.Source == forms.AvatarLocal
	if len(form.Gravatar) > 0 {
		if form.Avatar != nil {
			ctxUser.Avatar = base.EncodeMD5(form.Gravatar)
		} else {
			ctxUser.Avatar = ""
		}
		ctxUser.AvatarEmail = form.Gravatar
	}

	if form.Avatar != nil && form.Avatar.Filename != "" {
		fr, err := form.Avatar.Open()
		if err != nil {
			return fmt.Errorf("Avatar.Open: %v", err)
		}
		defer fr.Close()

		if form.Avatar.Size > setting.Avatar.MaxFileSize {
			return errors.New(ctx.Tr("settings.uploaded_avatar_is_too_big"))
		}

		data, err := ioutil.ReadAll(fr)
		if err != nil {
			return fmt.Errorf("ioutil.ReadAll: %v", err)
		}

		st := typesniffer.DetectContentType(data)
		if !(st.IsImage() && !st.IsSvgImage()) {
			return errors.New(ctx.Tr("settings.uploaded_avatar_not_a_image"))
		}
		if err = ctxUser.UploadAvatar(data); err != nil {
			return fmt.Errorf("UploadAvatar: %v", err)
		}
	} else if ctxUser.UseCustomAvatar && ctxUser.Avatar == "" {
		// No avatar is uploaded but setting has been changed to enable,
		// generate a random one when needed.
		if err := ctxUser.GenerateRandomAvatar(); err != nil {
			log.Error("GenerateRandomAvatar[%d]: %v", ctxUser.ID, err)
		}
	}

	if err := models.UpdateUserCols(ctxUser, "avatar", "avatar_email", "use_custom_avatar"); err != nil {
		return fmt.Errorf("UpdateUser: %v", err)
	}

	return nil
}

// AvatarPost response for change user's avatar request
func AvatarPost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AvatarForm)
	if err := UpdateAvatarSetting(ctx, form, ctx.User); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("settings.update_avatar_success"))
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings")
}

// DeleteAvatar render delete avatar page
func DeleteAvatar(ctx *context.Context) {
	if err := ctx.User.DeleteAvatar(); err != nil {
		ctx.Flash.Error(err.Error())
	}

	ctx.Redirect(setting.AppSubURL + "/user/settings")
}

// Organization render all the organization of the user
func Organization(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsOrganization"] = true

	opts := models.FindOrgOptions{
		ListOptions: models.ListOptions{
			PageSize: setting.UI.Admin.UserPagingNum,
			Page:     ctx.QueryInt("page"),
		},
		Actor:  ctx.User,
		UserID: ctx.User.ID,
	}

	if opts.Page <= 0 {
		opts.Page = 1
	}

	orgs, err := models.FindOrgs(opts)
	if err != nil {
		ctx.ServerError("FindOrgs", err)
		return
	}
	total, err := models.CountOrgs(opts)
	if err != nil {
		ctx.ServerError("CountOrgs", err)
		return
	}
	ctx.Data["Orgs"] = orgs
	pager := context.NewPagination(int(total), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplSettingsOrganization)
}

// Repos display a list of all repositories of the user
func Repos(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("settings")
	ctx.Data["PageIsSettingsRepos"] = true
	ctx.Data["allowAdopt"] = ctx.IsUserSiteAdmin() || setting.Repository.AllowAdoptionOfUnadoptedRepositories
	ctx.Data["allowDelete"] = ctx.IsUserSiteAdmin() || setting.Repository.AllowDeleteOfUnadoptedRepositories

	opts := models.ListOptions{
		PageSize: setting.UI.Admin.UserPagingNum,
		Page:     ctx.QueryInt("page"),
	}

	if opts.Page <= 0 {
		opts.Page = 1
	}
	start := (opts.Page - 1) * opts.PageSize
	end := start + opts.PageSize

	adoptOrDelete := ctx.IsUserSiteAdmin() || (setting.Repository.AllowAdoptionOfUnadoptedRepositories && setting.Repository.AllowDeleteOfUnadoptedRepositories)

	ctxUser := ctx.User
	count := 0

	if adoptOrDelete {
		repoNames := make([]string, 0, setting.UI.Admin.UserPagingNum)
		repos := map[string]*models.Repository{}
		// We're going to iterate by pagesize.
		root := filepath.Join(models.UserPath(ctxUser.Name))
		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !info.IsDir() || path == root {
				return nil
			}
			name := info.Name()
			if !strings.HasSuffix(name, ".git") {
				return filepath.SkipDir
			}
			name = name[:len(name)-4]
			if models.IsUsableRepoName(name) != nil || strings.ToLower(name) != name {
				return filepath.SkipDir
			}
			if count >= start && count < end {
				repoNames = append(repoNames, name)
			}
			count++
			return filepath.SkipDir
		}); err != nil {
			ctx.ServerError("filepath.Walk", err)
			return
		}

		if err := ctxUser.GetRepositories(models.ListOptions{Page: 1, PageSize: setting.UI.Admin.UserPagingNum}, repoNames...); err != nil {
			ctx.ServerError("GetRepositories", err)
			return
		}
		for _, repo := range ctxUser.Repos {
			if repo.IsFork {
				if err := repo.GetBaseRepo(); err != nil {
					ctx.ServerError("GetBaseRepo", err)
					return
				}
			}
			repos[repo.LowerName] = repo
		}
		ctx.Data["Dirs"] = repoNames
		ctx.Data["ReposMap"] = repos
	} else {
		var err error
		var count64 int64
		ctxUser.Repos, count64, err = models.GetUserRepositories(&models.SearchRepoOptions{Actor: ctxUser, Private: true, ListOptions: opts})

		if err != nil {
			ctx.ServerError("GetRepositories", err)
			return
		}
		count = int(count64)
		repos := ctxUser.Repos

		for i := range repos {
			if repos[i].IsFork {
				if err := repos[i].GetBaseRepo(); err != nil {
					ctx.ServerError("GetBaseRepo", err)
					return
				}
			}
		}

		ctx.Data["Repos"] = repos
	}
	ctx.Data["Owner"] = ctxUser
	pager := context.NewPagination(int(count), opts.PageSize, opts.Page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager
	ctx.HTML(http.StatusOK, tplSettingsRepositories)
}
