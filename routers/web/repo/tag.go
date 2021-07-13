// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

// Tags render the page to protect tags
func Tags(ctx *context.Context) {
	if setTagsContext(ctx) != nil {
		return
	}

	ctx.HTML(http.StatusOK, tplTags)
}

// NewProtectedTagPost handles creation of a protect tag
func NewProtectedTagPost(ctx *context.Context) {
	if setTagsContext(ctx) != nil {
		return
	}

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplTags)
		return
	}

	repo := ctx.Repo.Repository
	form := web.GetForm(ctx).(*forms.ProtectTagForm)

	pt := &models.ProtectedTag{
		RepoID:      repo.ID,
		NamePattern: strings.TrimSpace(form.NamePattern),
	}

	if strings.TrimSpace(form.AllowlistUsers) != "" {
		pt.AllowlistUserIDs, _ = base.StringsToInt64s(strings.Split(form.AllowlistUsers, ","))
	}
	if strings.TrimSpace(form.AllowlistTeams) != "" {
		pt.AllowlistTeamIDs, _ = base.StringsToInt64s(strings.Split(form.AllowlistTeams, ","))
	}

	if err := models.InsertProtectedTag(pt); err != nil {
		ctx.ServerError("InsertProtectedTag", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(setting.AppSubURL + ctx.Req.URL.Path)
}

// EditProtectedTag render the page to edit a protect tag
func EditProtectedTag(ctx *context.Context) {
	if setTagsContext(ctx) != nil {
		return
	}

	ctx.Data["PageIsEditProtectedTag"] = true

	pt := selectProtectedTagByContext(ctx)
	if pt == nil {
		return
	}

	ctx.Data["name_pattern"] = pt.NamePattern
	ctx.Data["allowlist_users"] = strings.Join(base.Int64sToStrings(pt.AllowlistUserIDs), ",")
	ctx.Data["allowlist_teams"] = strings.Join(base.Int64sToStrings(pt.AllowlistTeamIDs), ",")

	ctx.HTML(http.StatusOK, tplTags)
}

// EditProtectedTagPost handles creation of a protect tag
func EditProtectedTagPost(ctx *context.Context) {
	if setTagsContext(ctx) != nil {
		return
	}

	ctx.Data["PageIsEditProtectedTag"] = true

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplTags)
		return
	}

	pt := selectProtectedTagByContext(ctx)
	if pt == nil {
		return
	}

	form := web.GetForm(ctx).(*forms.ProtectTagForm)

	pt.NamePattern = strings.TrimSpace(form.NamePattern)
	pt.AllowlistUserIDs, _ = base.StringsToInt64s(strings.Split(form.AllowlistUsers, ","))
	pt.AllowlistTeamIDs, _ = base.StringsToInt64s(strings.Split(form.AllowlistTeams, ","))

	if err := models.UpdateProtectedTag(pt); err != nil {
		ctx.ServerError("UpdateProtectedTag", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(ctx.Repo.Repository.Link() + "/settings/tags")
}

// DeleteProtectedTagPost handles deletion of a protected tag
func DeleteProtectedTagPost(ctx *context.Context) {
	pt := selectProtectedTagByContext(ctx)
	if pt == nil {
		return
	}

	if err := models.DeleteProtectedTag(pt); err != nil {
		ctx.ServerError("DeleteProtectedTag", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
	ctx.Redirect(ctx.Repo.Repository.Link() + "/settings/tags")
}

func setTagsContext(ctx *context.Context) error {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsTags"] = true

	protectedTags, err := ctx.Repo.Repository.GetProtectedTags()
	if err != nil {
		ctx.ServerError("GetProtectedTags", err)
		return err
	}
	ctx.Data["ProtectedTags"] = protectedTags

	users, err := ctx.Repo.Repository.GetReaders()
	if err != nil {
		ctx.ServerError("Repo.Repository.GetReaders", err)
		return err
	}
	ctx.Data["Users"] = users

	if ctx.Repo.Owner.IsOrganization() {
		teams, err := ctx.Repo.Owner.TeamsWithAccessToRepo(ctx.Repo.Repository.ID, models.AccessModeRead)
		if err != nil {
			ctx.ServerError("Repo.Owner.TeamsWithAccessToRepo", err)
			return err
		}
		ctx.Data["Teams"] = teams
	}

	return nil
}

func selectProtectedTagByContext(ctx *context.Context) *models.ProtectedTag {
	id := ctx.QueryInt64("id")
	if id == 0 {
		id = ctx.ParamsInt64(":id")
	}

	tag, err := models.GetProtectedTagByID(id)
	if err != nil {
		ctx.ServerError("GetProtectedTagByID", err)
		return nil
	}

	if tag != nil && tag.RepoID == ctx.Repo.Repository.ID {
		return tag
	}

	ctx.NotFound("", fmt.Errorf("ProtectedTag[%v] not associated to repository %v", id, ctx.Repo.Repository))

	return nil
}
