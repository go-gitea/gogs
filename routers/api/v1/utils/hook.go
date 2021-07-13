// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/routers/utils"
	"code.gitea.io/gitea/services/webhook"
	jsoniter "github.com/json-iterator/go"
)

// GetOrgHook get an organization's webhook. If there is an error, write to
// `ctx` accordingly and return the error
func GetOrgHook(ctx *context.APIContext, orgID, hookID int64) (*models.Webhook, error) {
	w, err := models.GetWebhookByOrgID(orgID, hookID)
	if err != nil {
		if models.IsErrWebhookNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetWebhookByOrgID", err)
		}
		return nil, err
	}
	return w, nil
}

// GetRepoHook get a repo's webhook. If there is an error, write to `ctx`
// accordingly and return the error
func GetRepoHook(ctx *context.APIContext, repoID, hookID int64) (*models.Webhook, error) {
	w, err := models.GetWebhookByRepoID(repoID, hookID)
	if err != nil {
		if models.IsErrWebhookNotExist(err) {
			ctx.NotFound()
		} else {
			ctx.Error(http.StatusInternalServerError, "GetWebhookByID", err)
		}
		return nil, err
	}
	return w, nil
}

// CheckCreateHookOption check if a CreateHookOption form is valid. If invalid,
// write the appropriate error to `ctx`. Return whether the form is valid
func CheckCreateHookOption(ctx *context.APIContext, form *api.CreateHookOption) bool {
	if !webhook.IsValidHookTaskType(form.Type) {
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Invalid hook type: %s", form.Type))
		return false
	}
	for _, name := range []string{"url", "content_type"} {
		if _, ok := form.Config[name]; !ok {
			ctx.Error(http.StatusUnprocessableEntity, "", "Missing config option: "+name)
			return false
		}
	}
	if !models.IsValidHookContentType(form.Config["content_type"]) {
		ctx.Error(http.StatusUnprocessableEntity, "", "Invalid content type")
		return false
	}
	return true
}

// AddOrgHook add a hook to an organization. Writes to `ctx` accordingly
func AddOrgHook(ctx *context.APIContext, form *api.CreateHookOption) {
	org := ctx.Org.Organization
	hook, ok := addHook(ctx, form, org.ID, 0)
	if ok {
		ctx.JSON(http.StatusCreated, convert.ToHook(org.HomeLink(), hook))
	}
}

// AddRepoHook add a hook to a repo. Writes to `ctx` accordingly
func AddRepoHook(ctx *context.APIContext, form *api.CreateHookOption) {
	repo := ctx.Repo
	hook, ok := addHook(ctx, form, 0, repo.Repository.ID)
	if ok {
		ctx.JSON(http.StatusCreated, convert.ToHook(repo.RepoLink, hook))
	}
}

func issuesHook(events []string, event string) bool {
	return util.IsStringInSlice(event, events, true) || util.IsStringInSlice(string(models.HookEventIssues), events, true)
}

func pullHook(events []string, event string) bool {
	return util.IsStringInSlice(event, events, true) || util.IsStringInSlice(string(models.HookEventPullRequest), events, true)
}

// addHook add the hook specified by `form`, `orgID` and `repoID`. If there is
// an error, write to `ctx` accordingly. Return (webhook, ok)
func addHook(ctx *context.APIContext, form *api.CreateHookOption, orgID, repoID int64) (*models.Webhook, bool) {
	if len(form.Events) == 0 {
		form.Events = []string{"push"}
	}
	w := &models.Webhook{
		OrgID:       orgID,
		RepoID:      repoID,
		URL:         form.Config["url"],
		ContentType: models.ToHookContentType(form.Config["content_type"]),
		Secret:      form.Config["secret"],
		HTTPMethod:  "POST",
		HookEvent: &models.HookEvent{
			ChooseEvents: true,
			HookEvents: models.HookEvents{
				Create:               util.IsStringInSlice(string(models.HookEventCreate), form.Events, true),
				Delete:               util.IsStringInSlice(string(models.HookEventDelete), form.Events, true),
				Fork:                 util.IsStringInSlice(string(models.HookEventFork), form.Events, true),
				Issues:               issuesHook(form.Events, "issues_only"),
				IssueAssign:          issuesHook(form.Events, string(models.HookEventIssueAssign)),
				IssueLabel:           issuesHook(form.Events, string(models.HookEventIssueLabel)),
				IssueMilestone:       issuesHook(form.Events, string(models.HookEventIssueMilestone)),
				IssueComment:         issuesHook(form.Events, string(models.HookEventIssueComment)),
				Push:                 util.IsStringInSlice(string(models.HookEventPush), form.Events, true),
				PullRequest:          pullHook(form.Events, "pull_request_only"),
				PullRequestAssign:    pullHook(form.Events, string(models.HookEventPullRequestAssign)),
				PullRequestLabel:     pullHook(form.Events, string(models.HookEventPullRequestLabel)),
				PullRequestMilestone: pullHook(form.Events, string(models.HookEventPullRequestMilestone)),
				PullRequestComment:   pullHook(form.Events, string(models.HookEventPullRequestComment)),
				PullRequestReview:    pullHook(form.Events, "pull_request_review"),
				PullRequestSync:      pullHook(form.Events, string(models.HookEventPullRequestSync)),
				Repository:           util.IsStringInSlice(string(models.HookEventRepository), form.Events, true),
				Release:              util.IsStringInSlice(string(models.HookEventRelease), form.Events, true),
			},
			BranchFilter: form.BranchFilter,
		},
		IsActive: form.Active,
		Type:     models.HookType(form.Type),
	}
	if w.Type == models.SLACK {
		channel, ok := form.Config["channel"]
		if !ok {
			ctx.Error(http.StatusUnprocessableEntity, "", "Missing config option: channel")
			return nil, false
		}

		if !utils.IsValidSlackChannel(channel) {
			ctx.Error(http.StatusBadRequest, "", "Invalid slack channel name")
			return nil, false
		}

		json := jsoniter.ConfigCompatibleWithStandardLibrary
		meta, err := json.Marshal(&webhook.SlackMeta{
			Channel:  strings.TrimSpace(channel),
			Username: form.Config["username"],
			IconURL:  form.Config["icon_url"],
			Color:    form.Config["color"],
		})
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "slack: JSON marshal failed", err)
			return nil, false
		}
		w.Meta = string(meta)
	}

	if err := w.UpdateEvent(); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateEvent", err)
		return nil, false
	} else if err := models.CreateWebhook(w); err != nil {
		ctx.Error(http.StatusInternalServerError, "CreateWebhook", err)
		return nil, false
	}
	return w, true
}

// EditOrgHook edit webhook `w` according to `form`. Writes to `ctx` accordingly
func EditOrgHook(ctx *context.APIContext, form *api.EditHookOption, hookID int64) {
	org := ctx.Org.Organization
	hook, err := GetOrgHook(ctx, org.ID, hookID)
	if err != nil {
		return
	}
	if !editHook(ctx, form, hook) {
		return
	}
	updated, err := GetOrgHook(ctx, org.ID, hookID)
	if err != nil {
		return
	}
	ctx.JSON(http.StatusOK, convert.ToHook(org.HomeLink(), updated))
}

// EditRepoHook edit webhook `w` according to `form`. Writes to `ctx` accordingly
func EditRepoHook(ctx *context.APIContext, form *api.EditHookOption, hookID int64) {
	repo := ctx.Repo
	hook, err := GetRepoHook(ctx, repo.Repository.ID, hookID)
	if err != nil {
		return
	}
	if !editHook(ctx, form, hook) {
		return
	}
	updated, err := GetRepoHook(ctx, repo.Repository.ID, hookID)
	if err != nil {
		return
	}
	ctx.JSON(http.StatusOK, convert.ToHook(repo.RepoLink, updated))
}

// editHook edit the webhook `w` according to `form`. If an error occurs, write
// to `ctx` accordingly and return the error. Return whether successful
func editHook(ctx *context.APIContext, form *api.EditHookOption, w *models.Webhook) bool {
	if form.Config != nil {
		if url, ok := form.Config["url"]; ok {
			w.URL = url
		}
		if ct, ok := form.Config["content_type"]; ok {
			if !models.IsValidHookContentType(ct) {
				ctx.Error(http.StatusUnprocessableEntity, "", "Invalid content type")
				return false
			}
			w.ContentType = models.ToHookContentType(ct)
		}

		if w.Type == models.SLACK {
			if channel, ok := form.Config["channel"]; ok {
				json := jsoniter.ConfigCompatibleWithStandardLibrary
				meta, err := json.Marshal(&webhook.SlackMeta{
					Channel:  channel,
					Username: form.Config["username"],
					IconURL:  form.Config["icon_url"],
					Color:    form.Config["color"],
				})
				if err != nil {
					ctx.Error(http.StatusInternalServerError, "slack: JSON marshal failed", err)
					return false
				}
				w.Meta = string(meta)
			}
		}
	}

	// Update events
	if len(form.Events) == 0 {
		form.Events = []string{"push"}
	}
	w.PushOnly = false
	w.SendEverything = false
	w.ChooseEvents = true
	w.Create = util.IsStringInSlice(string(models.HookEventCreate), form.Events, true)
	w.Push = util.IsStringInSlice(string(models.HookEventPush), form.Events, true)
	w.PullRequest = util.IsStringInSlice(string(models.HookEventPullRequest), form.Events, true)
	w.Create = util.IsStringInSlice(string(models.HookEventCreate), form.Events, true)
	w.Delete = util.IsStringInSlice(string(models.HookEventDelete), form.Events, true)
	w.Fork = util.IsStringInSlice(string(models.HookEventFork), form.Events, true)
	w.Issues = util.IsStringInSlice(string(models.HookEventIssues), form.Events, true)
	w.IssueComment = util.IsStringInSlice(string(models.HookEventIssueComment), form.Events, true)
	w.Push = util.IsStringInSlice(string(models.HookEventPush), form.Events, true)
	w.PullRequest = util.IsStringInSlice(string(models.HookEventPullRequest), form.Events, true)
	w.Repository = util.IsStringInSlice(string(models.HookEventRepository), form.Events, true)
	w.Release = util.IsStringInSlice(string(models.HookEventRelease), form.Events, true)
	w.BranchFilter = form.BranchFilter

	if err := w.UpdateEvent(); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateEvent", err)
		return false
	}

	if form.Active != nil {
		w.IsActive = *form.Active
	}

	if err := models.UpdateWebhook(w); err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateWebhook", err)
		return false
	}
	return true
}
