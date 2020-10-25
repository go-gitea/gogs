// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/webhook"

	"github.com/unknwon/com"
)

const (
	tplHooks        base.TplName = "repo/settings/webhook/base"
	tplHookNew      base.TplName = "repo/settings/webhook/new"
	tplOrgHookNew   base.TplName = "org/settings/hook_new"
	tplAdminHookNew base.TplName = "admin/hook_new"
)

// Webhooks render web hooks list page
func Webhooks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.hooks")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["BaseLink"] = ctx.Repo.RepoLink + "/settings/hooks"
	ctx.Data["Description"] = ctx.Tr("repo.settings.hooks_desc", "https://docs.gitea.io/en-us/webhooks/")

	ws, err := models.GetWebhooksByRepoID(ctx.Repo.Repository.ID, models.ListOptions{})
	if err != nil {
		ctx.ServerError("GetWebhooksByRepoID", err)
		return
	}
	ctx.Data["Webhooks"] = ws

	ctx.HTML(200, tplHooks)
}

type orgRepoCtx struct {
	OrgID           int64
	RepoID          int64
	IsAdmin         bool
	IsSystemWebhook bool
	Link            string
	NewTemplate     base.TplName
}

// getOrgRepoCtx determines whether this is a repo, organization, or admin (both default and system) context.
func getOrgRepoCtx(ctx *context.Context) (*orgRepoCtx, error) {
	if len(ctx.Repo.RepoLink) > 0 {
		return &orgRepoCtx{
			RepoID:      ctx.Repo.Repository.ID,
			Link:        path.Join(ctx.Repo.RepoLink, "settings/hooks"),
			NewTemplate: tplHookNew,
		}, nil
	}

	if len(ctx.Org.OrgLink) > 0 {
		return &orgRepoCtx{
			OrgID:       ctx.Org.Organization.ID,
			Link:        path.Join(ctx.Org.OrgLink, "settings/hooks"),
			NewTemplate: tplOrgHookNew,
		}, nil
	}

	if ctx.User.IsAdmin {
		// Are we looking at default webhooks?
		if ctx.Params(":configType") == "hooks" {
			return &orgRepoCtx{
				IsAdmin:     true,
				Link:        path.Join(setting.AppSubURL, "/admin/hooks"),
				NewTemplate: tplAdminHookNew,
			}, nil
		}

		// Must be system webhooks instead
		return &orgRepoCtx{
			IsAdmin:         true,
			IsSystemWebhook: true,
			Link:            path.Join(setting.AppSubURL, "/admin/system-hooks"),
			NewTemplate:     tplAdminHookNew,
		}, nil
	}

	return nil, errors.New("Unable to set OrgRepo context")
}

func checkHookType(ctx *context.Context) string {
	hookType := strings.ToLower(ctx.Params(":type"))
	if !com.IsSliceContainsStr(setting.Webhook.Types, hookType) {
		ctx.NotFound("checkHookType", nil)
		return ""
	}
	return hookType
}

// WebhooksNew render creating webhook page
func WebhooksNew(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if orCtx.IsAdmin && orCtx.IsSystemWebhook {
		ctx.Data["PageIsAdminSystemHooks"] = true
		ctx.Data["PageIsAdminSystemHooksNew"] = true
	} else if orCtx.IsAdmin {
		ctx.Data["PageIsAdminHooks"] = true
		ctx.Data["PageIsAdminHooksNew"] = true
	} else {
		ctx.Data["PageIsSettingsHooks"] = true
		ctx.Data["PageIsSettingsHooksNew"] = true
	}

	hookType := checkHookType(ctx)
	ctx.Data["HookType"] = hookType
	if ctx.Written() {
		return
	}
	if hookType == "discord" {
		ctx.Data["DiscordHook"] = map[string]interface{}{
			"Username": "Gitea",
			"IconURL":  setting.AppURL + "img/favicon.png",
		}
	} else if hookType == "workwechat" {
		ctx.Data["WorkwechatHook"] = map[string]interface{}{
			"ChatID": "gitea",
		}
	}
	ctx.Data["BaseLink"] = orCtx.Link

	ctx.HTML(200, orCtx.NewTemplate)
}

// ParseHookEvent convert web form content to models.HookEvent
func ParseHookEvent(form auth.WebhookForm) *models.HookEvent {
	return &models.HookEvent{
		PushOnly:       form.PushOnly(),
		SendEverything: form.SendEverything(),
		ChooseEvents:   form.ChooseEvents(),
		HookEvents: models.HookEvents{
			Create:               form.Create,
			Delete:               form.Delete,
			Fork:                 form.Fork,
			Issues:               form.Issues,
			IssueAssign:          form.IssueAssign,
			IssueLabel:           form.IssueLabel,
			IssueMilestone:       form.IssueMilestone,
			IssueComment:         form.IssueComment,
			Release:              form.Release,
			Push:                 form.Push,
			PullRequest:          form.PullRequest,
			PullRequestAssign:    form.PullRequestAssign,
			PullRequestLabel:     form.PullRequestLabel,
			PullRequestMilestone: form.PullRequestMilestone,
			PullRequestComment:   form.PullRequestComment,
			PullRequestReview:    form.PullRequestReview,
			PullRequestSync:      form.PullRequestSync,
			Repository:           form.Repository,
		},
		BranchFilter: form.BranchFilter,
	}
}

// GiteaHooksNewPost response for creating Gitea webhook
func GiteaHooksNewPost(ctx *context.Context, form auth.NewWebhookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}
	ctx.Data["HookType"] = models.GITEA.Name()

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}
	ctx.Data["BaseLink"] = orCtx.Link

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	contentType := models.ContentTypeJSON
	if models.HookContentType(form.ContentType) == models.ContentTypeForm {
		contentType = models.ContentTypeForm
	}

	w := &models.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		HTTPMethod:      form.HTTPMethod,
		ContentType:     contentType,
		Secret:          form.Secret,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		HookTaskType:    models.GITEA,
		OrgID:           orCtx.OrgID,
		IsSystemWebhook: orCtx.IsSystemWebhook,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.ServerError("CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link)
}

// GogsHooksNewPost response for creating webhook
func GogsHooksNewPost(ctx *context.Context, form auth.NewGogshookForm) {
	newGogsWebhookPost(ctx, form, models.GOGS)
}

// newGogsWebhookPost response for creating gogs hook
func newGogsWebhookPost(ctx *context.Context, form auth.NewGogshookForm, kind models.HookTaskType) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.add_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}
	ctx.Data["HookType"] = models.GOGS.Name()

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}
	ctx.Data["BaseLink"] = orCtx.Link

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	contentType := models.ContentTypeJSON
	if models.HookContentType(form.ContentType) == models.ContentTypeForm {
		contentType = models.ContentTypeForm
	}

	w := &models.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     contentType,
		Secret:          form.Secret,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		HookTaskType:    kind,
		OrgID:           orCtx.OrgID,
		IsSystemWebhook: orCtx.IsSystemWebhook,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.ServerError("CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link)
}

// DiscordHooksNewPost response for creating discord hook
func DiscordHooksNewPost(ctx *context.Context, form auth.NewDiscordHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}
	ctx.Data["HookType"] = models.DISCORD.Name()

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&webhook.DiscordMeta{
		Username: form.Username,
		IconURL:  form.IconURL,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w := &models.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     models.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		HookTaskType:    models.DISCORD,
		Meta:            string(meta),
		OrgID:           orCtx.OrgID,
		IsSystemWebhook: orCtx.IsSystemWebhook,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.ServerError("CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link)
}

// DingtalkHooksNewPost response for creating dingtalk hook
func DingtalkHooksNewPost(ctx *context.Context, form auth.NewDingtalkHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}
	ctx.Data["HookType"] = models.DINGTALK.Name()

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	w := &models.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     models.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		HookTaskType:    models.DINGTALK,
		Meta:            "",
		OrgID:           orCtx.OrgID,
		IsSystemWebhook: orCtx.IsSystemWebhook,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.ServerError("CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link)
}

// TelegramHooksNewPost response for creating telegram hook
func TelegramHooksNewPost(ctx *context.Context, form auth.NewTelegramHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}
	ctx.Data["HookType"] = models.TELEGRAM.Name()

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&webhook.TelegramMeta{
		BotToken: form.BotToken,
		ChatID:   form.ChatID,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w := &models.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%s", form.BotToken, form.ChatID),
		ContentType:     models.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		HookTaskType:    models.TELEGRAM,
		Meta:            string(meta),
		OrgID:           orCtx.OrgID,
		IsSystemWebhook: orCtx.IsSystemWebhook,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.ServerError("CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link)
}

// MatrixHooksNewPost response for creating a Matrix hook
func MatrixHooksNewPost(ctx *context.Context, form auth.NewMatrixHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}
	ctx.Data["HookType"] = models.MATRIX.Name()

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&webhook.MatrixMeta{
		HomeserverURL: form.HomeserverURL,
		Room:          form.RoomID,
		AccessToken:   form.AccessToken,
		MessageType:   form.MessageType,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w := &models.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/send/m.room.message", form.HomeserverURL, form.RoomID),
		ContentType:     models.ContentTypeJSON,
		HTTPMethod:      "PUT",
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		HookTaskType:    models.MATRIX,
		Meta:            string(meta),
		OrgID:           orCtx.OrgID,
		IsSystemWebhook: orCtx.IsSystemWebhook,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.ServerError("CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link)
}

// MSTeamsHooksNewPost response for creating MS Teams hook
func MSTeamsHooksNewPost(ctx *context.Context, form auth.NewMSTeamsHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}
	ctx.Data["HookType"] = models.MSTEAMS.Name()

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	w := &models.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     models.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		HookTaskType:    models.MSTEAMS,
		Meta:            "",
		OrgID:           orCtx.OrgID,
		IsSystemWebhook: orCtx.IsSystemWebhook,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.ServerError("CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link)
}

// SlackHooksNewPost response for creating slack hook
func SlackHooksNewPost(ctx *context.Context, form auth.NewSlackHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}
	ctx.Data["HookType"] = models.SLACK.Name()

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	if form.HasInvalidChannel() {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_webhook.invalid_channel_name"))
		ctx.Redirect(orCtx.Link + "/slack/new")
		return
	}

	meta, err := json.Marshal(&webhook.SlackMeta{
		Channel:  strings.TrimSpace(form.Channel),
		Username: form.Username,
		IconURL:  form.IconURL,
		Color:    form.Color,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w := &models.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     models.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		HookTaskType:    models.SLACK,
		Meta:            string(meta),
		OrgID:           orCtx.OrgID,
		IsSystemWebhook: orCtx.IsSystemWebhook,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.ServerError("CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link)
}

// FeishuHooksNewPost response for creating feishu hook
func FeishuHooksNewPost(ctx *context.Context, form auth.NewFeishuHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}
	ctx.Data["HookType"] = models.FEISHU.Name()

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	w := &models.Webhook{
		RepoID:          orCtx.RepoID,
		URL:             form.PayloadURL,
		ContentType:     models.ContentTypeJSON,
		HookEvent:       ParseHookEvent(form.WebhookForm),
		IsActive:        form.Active,
		HookTaskType:    models.FEISHU,
		Meta:            "",
		OrgID:           orCtx.OrgID,
		IsSystemWebhook: orCtx.IsSystemWebhook,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.ServerError("CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link)
}

// WorkwechatHooksNewPost response for creating work wechat hook
func WorkwechatHooksNewPost(ctx *context.Context, form auth.NewWorkwechatHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksNew"] = true
	ctx.Data["Webhook"] = models.Webhook{HookEvent: &models.HookEvent{}}

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return
	}

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}
	meta, err := json.Marshal(&webhook.WorkwechatMeta{
		ChatID: form.ChatID,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}
	w := &models.Webhook{
		RepoID:       orCtx.RepoID,
		URL:          form.PayloadURL,
		ContentType:  models.ContentTypeJSON,
		HookEvent:    ParseHookEvent(form.WebhookForm),
		IsActive:     form.Active,
		HookTaskType: models.WORKWECHAT,
		Meta:         string(meta),
		OrgID:        orCtx.OrgID,
	}
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.ServerError("CreateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_hook_success"))
	ctx.Redirect(orCtx.Link)
}

func checkWebhook(ctx *context.Context) (*orgRepoCtx, *models.Webhook) {
	ctx.Data["RequireHighlightJS"] = true

	orCtx, err := getOrgRepoCtx(ctx)
	if err != nil {
		ctx.ServerError("getOrgRepoCtx", err)
		return nil, nil
	}
	ctx.Data["BaseLink"] = orCtx.Link

	var w *models.Webhook
	if orCtx.RepoID > 0 {
		w, err = models.GetWebhookByRepoID(ctx.Repo.Repository.ID, ctx.ParamsInt64(":id"))
	} else if orCtx.OrgID > 0 {
		w, err = models.GetWebhookByOrgID(ctx.Org.Organization.ID, ctx.ParamsInt64(":id"))
	} else if orCtx.IsSystemWebhook {
		w, err = models.GetSystemWebhook(ctx.ParamsInt64(":id"))
	} else {
		w, err = models.GetDefaultWebhook(ctx.ParamsInt64(":id"))
	}
	if err != nil {
		if models.IsErrWebhookNotExist(err) {
			ctx.NotFound("GetWebhookByID", nil)
		} else {
			ctx.ServerError("GetWebhookByID", err)
		}
		return nil, nil
	}

	ctx.Data["HookType"] = w.HookTaskType.Name()
	switch w.HookTaskType {
	case models.SLACK:
		ctx.Data["SlackHook"] = webhook.GetSlackHook(w)
	case models.DISCORD:
		ctx.Data["DiscordHook"] = webhook.GetDiscordHook(w)
	case models.TELEGRAM:
		ctx.Data["TelegramHook"] = webhook.GetTelegramHook(w)
	case models.MATRIX:
		ctx.Data["MatrixHook"] = webhook.GetMatrixHook(w)
	case models.WORKWECHAT:
		ctx.Data["WorkwechatHook"] = webhook.GetWorkwechatHook(w)
	}

	ctx.Data["History"], err = w.History(1)
	if err != nil {
		ctx.ServerError("History", err)
	}
	return orCtx, w
}

// WebHooksEdit render editing web hook page
func WebHooksEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.update_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	ctx.HTML(200, orCtx.NewTemplate)
}

// WebHooksEditPost response for editing web hook
func WebHooksEditPost(ctx *context.Context, form auth.NewWebhookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.update_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	contentType := models.ContentTypeJSON
	if models.HookContentType(form.ContentType) == models.ContentTypeForm {
		contentType = models.ContentTypeForm
	}

	w.URL = form.PayloadURL
	w.ContentType = contentType
	w.Secret = form.Secret
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	w.HTTPMethod = form.HTTPMethod
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.ServerError("WebHooksEditPost", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// GogsHooksEditPost response for editing gogs hook
func GogsHooksEditPost(ctx *context.Context, form auth.NewGogshookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.update_webhook")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	contentType := models.ContentTypeJSON
	if models.HookContentType(form.ContentType) == models.ContentTypeForm {
		contentType = models.ContentTypeForm
	}

	w.URL = form.PayloadURL
	w.ContentType = contentType
	w.Secret = form.Secret
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.ServerError("GogsHooksEditPost", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// SlackHooksEditPost response for editing slack hook
func SlackHooksEditPost(ctx *context.Context, form auth.NewSlackHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	if form.HasInvalidChannel() {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_webhook.invalid_channel_name"))
		ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
		return
	}

	meta, err := json.Marshal(&webhook.SlackMeta{
		Channel:  strings.TrimSpace(form.Channel),
		Username: form.Username,
		IconURL:  form.IconURL,
		Color:    form.Color,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w.URL = form.PayloadURL
	w.Meta = string(meta)
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.ServerError("UpdateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// DiscordHooksEditPost response for editing discord hook
func DiscordHooksEditPost(ctx *context.Context, form auth.NewDiscordHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	meta, err := json.Marshal(&webhook.DiscordMeta{
		Username: form.Username,
		IconURL:  form.IconURL,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w.URL = form.PayloadURL
	w.Meta = string(meta)
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.ServerError("UpdateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// DingtalkHooksEditPost response for editing dingtalk hook
func DingtalkHooksEditPost(ctx *context.Context, form auth.NewDingtalkHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	w.URL = form.PayloadURL
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.ServerError("UpdateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// TelegramHooksEditPost response for editing discord hook
func TelegramHooksEditPost(ctx *context.Context, form auth.NewTelegramHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}
	meta, err := json.Marshal(&webhook.TelegramMeta{
		BotToken: form.BotToken,
		ChatID:   form.ChatID,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}
	w.Meta = string(meta)
	w.URL = fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%s", form.BotToken, form.ChatID)
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.ServerError("UpdateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// MatrixHooksEditPost response for editing a Matrix hook
func MatrixHooksEditPost(ctx *context.Context, form auth.NewMatrixHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}
	meta, err := json.Marshal(&webhook.MatrixMeta{
		HomeserverURL: form.HomeserverURL,
		Room:          form.RoomID,
		AccessToken:   form.AccessToken,
		MessageType:   form.MessageType,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}
	w.Meta = string(meta)
	w.URL = fmt.Sprintf("%s/_matrix/client/r0/rooms/%s/send/m.room.message", form.HomeserverURL, form.RoomID)

	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.ServerError("UpdateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// MSTeamsHooksEditPost response for editing MS Teams hook
func MSTeamsHooksEditPost(ctx *context.Context, form auth.NewMSTeamsHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	w.URL = form.PayloadURL
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.ServerError("UpdateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// FeishuHooksEditPost response for editing feishu hook
func FeishuHooksEditPost(ctx *context.Context, form auth.NewFeishuHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}

	w.URL = form.PayloadURL
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.ServerError("UpdateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// WorkwechatHooksEditPost response for editing work wechat hook
func WorkwechatHooksEditPost(ctx *context.Context, form auth.NewWorkwechatHookForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsHooks"] = true
	ctx.Data["PageIsSettingsHooksEdit"] = true

	orCtx, w := checkWebhook(ctx)
	if ctx.Written() {
		return
	}
	ctx.Data["Webhook"] = w

	if ctx.HasError() {
		ctx.HTML(200, orCtx.NewTemplate)
		return
	}
	meta, err := json.Marshal(&webhook.WorkwechatMeta{
		ChatID: form.ChatID,
	})
	if err != nil {
		ctx.ServerError("Marshal", err)
		return
	}

	w.URL = form.PayloadURL
	w.Meta = string(meta)
	w.HookEvent = ParseHookEvent(form.WebhookForm)
	w.IsActive = form.Active
	if err := w.UpdateEvent(); err != nil {
		ctx.ServerError("UpdateEvent", err)
		return
	} else if err := models.UpdateWebhook(w); err != nil {
		ctx.ServerError("UpdateWebhook", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.update_hook_success"))
	ctx.Redirect(fmt.Sprintf("%s/%d", orCtx.Link, w.ID))
}

// TestWebhook test if web hook is work fine
func TestWebhook(ctx *context.Context) {
	hookID := ctx.ParamsInt64(":id")
	w, err := models.GetWebhookByRepoID(ctx.Repo.Repository.ID, hookID)
	if err != nil {
		ctx.Flash.Error("GetWebhookByID: " + err.Error())
		ctx.Status(500)
		return
	}

	// Grab latest commit or fake one if it's empty repository.
	commit := ctx.Repo.Commit
	if commit == nil {
		ghost := models.NewGhostUser()
		commit = &git.Commit{
			ID:            git.MustIDFromString(git.EmptySHA),
			Author:        ghost.NewGitSig(),
			Committer:     ghost.NewGitSig(),
			CommitMessage: "This is a fake commit",
		}
	}

	apiUser := convert.ToUser(ctx.User, true, true)
	p := &api.PushPayload{
		Ref:    git.BranchPrefix + ctx.Repo.Repository.DefaultBranch,
		Before: commit.ID.String(),
		After:  commit.ID.String(),
		Commits: []*api.PayloadCommit{
			{
				ID:      commit.ID.String(),
				Message: commit.Message(),
				URL:     ctx.Repo.Repository.HTMLURL() + "/commit/" + commit.ID.String(),
				Author: &api.PayloadUser{
					Name:  commit.Author.Name,
					Email: commit.Author.Email,
				},
				Committer: &api.PayloadUser{
					Name:  commit.Committer.Name,
					Email: commit.Committer.Email,
				},
			},
		},
		Repo:   ctx.Repo.Repository.APIFormat(models.AccessModeNone),
		Pusher: apiUser,
		Sender: apiUser,
	}
	if err := webhook.PrepareWebhook(w, ctx.Repo.Repository, models.HookEventPush, p); err != nil {
		ctx.Flash.Error("PrepareWebhook: " + err.Error())
		ctx.Status(500)
	} else {
		ctx.Flash.Info(ctx.Tr("repo.settings.webhook.test_delivery_success"))
		ctx.Status(200)
	}
}

// DeleteWebhook delete a webhook
func DeleteWebhook(ctx *context.Context) {
	if err := models.DeleteWebhookByRepoID(ctx.Repo.Repository.ID, ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeleteWebhookByRepoID: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.webhook_deletion_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/settings/hooks",
	})
}
