// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/label"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	issue_service "code.gitea.io/gitea/services/issue"
)

const (
	tplLabels base.TplName = "repo/issue/labels"
)

// Labels render issue's labels page
func Labels(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.labels")
	ctx.Data["PageIsIssueList"] = true
	ctx.Data["PageIsLabels"] = true
	ctx.Data["RequireTribute"] = true
	ctx.Data["LabelTemplates"] = models.LabelTemplates
	ctx.Data["LabelPriorities"] = label.GetPriorities()
	ctx.HTML(200, tplLabels)
}

// InitializeLabels init labels for a repository
func InitializeLabels(ctx *context.Context, form auth.InitializeLabelsForm) {
	if ctx.HasError() {
		ctx.Redirect(ctx.Repo.RepoLink + "/labels")
		return
	}

	if err := models.InitializeLabels(models.DefaultDBContext(), ctx.Repo.Repository.ID, form.TemplateName, false); err != nil {
		if models.IsErrIssueLabelTemplateLoad(err) {
			originalErr := err.(models.ErrIssueLabelTemplateLoad).OriginalError
			ctx.Flash.Error(ctx.Tr("repo.issues.label_templates.fail_to_load_file", form.TemplateName, originalErr))
			ctx.Redirect(ctx.Repo.RepoLink + "/labels")
			return
		}
		ctx.ServerError("InitializeLabels", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/labels")
}

// RetrieveLabels find all the labels of a repository and organization
func RetrieveLabels(ctx *context.Context) {
	labels, err := models.GetLabelsByRepoID(ctx.Repo.Repository.ID, ctx.Query("sort"), models.ListOptions{})
	if err != nil {
		ctx.ServerError("RetrieveLabels.GetLabels", err)
		return
	}

	for _, l := range labels {
		l.CalOpenIssues()
	}

	ctx.Data["Labels"] = labels

	if ctx.Repo.Owner.IsOrganization() {
		orgLabels, err := models.GetLabelsByOrgID(ctx.Repo.Owner.ID, ctx.Query("sort"), models.ListOptions{})
		if err != nil {
			ctx.ServerError("GetLabelsByOrgID", err)
			return
		}
		for _, l := range orgLabels {
			l.CalOpenOrgIssues(ctx.Repo.Repository.ID, l.ID)
		}
		ctx.Data["OrgLabels"] = orgLabels

		org, err := models.GetOrgByName(ctx.Repo.Owner.LowerName)
		if err != nil {
			ctx.ServerError("GetOrgByName", err)
			return
		}
		if ctx.User != nil {
			ctx.Org.IsOwner, err = org.IsOwnedBy(ctx.User.ID)
			if err != nil {
				ctx.ServerError("org.IsOwnedBy", err)
				return
			}
			ctx.Org.OrgLink = setting.AppSubURL + "/org/" + org.LowerName
			ctx.Data["IsOrganizationOwner"] = ctx.Org.IsOwner
			ctx.Data["OrganizationLink"] = ctx.Org.OrgLink
		}
	}
	ctx.Data["NumLabels"] = len(labels)
	ctx.Data["SortType"] = ctx.Query("sort")
}

// NewLabel create new label for repository
func NewLabel(ctx *context.Context, form auth.CreateLabelForm) {
	ctx.Data["Title"] = ctx.Tr("repo.labels")
	ctx.Data["PageIsLabels"] = true

	if ctx.HasError() {
		ctx.Flash.Error(ctx.Data["ErrorMsg"].(string))
		ctx.Redirect(ctx.Repo.RepoLink + "/labels")
		return
	}

	l := &models.Label{
		RepoID:      ctx.Repo.Repository.ID,
		Name:        form.Title,
		Description: form.Description,
		Color:       form.Color,
	}
	if err := models.NewLabel(l); err != nil {
		ctx.ServerError("NewLabel", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/labels")
}

// UpdateLabel update a label's name and color
func UpdateLabel(ctx *context.Context, form auth.CreateLabelForm) {
	l, err := models.GetLabelInRepoByID(ctx.Repo.Repository.ID, form.ID)
	if err != nil {
		switch {
		case models.IsErrRepoLabelNotExist(err):
			ctx.Error(404)
		default:
			ctx.ServerError("UpdateLabel", err)
		}
		return
	}

	l.Name = form.Title
	l.Description = form.Description
	l.Priority = form.Priority
	l.Color = form.Color
	if err := models.UpdateLabel(l); err != nil {
		ctx.ServerError("UpdateLabel", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/labels")
}

// DeleteLabel delete a label
func DeleteLabel(ctx *context.Context) {
	if err := models.DeleteLabel(ctx.Repo.Repository.ID, ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeleteLabel: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.issues.label_deletion_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/labels",
	})
}

// UpdateIssueLabel change issue's labels
func UpdateIssueLabel(ctx *context.Context) {
	issues := getActionIssues(ctx)
	if ctx.Written() {
		return
	}

	switch action := ctx.Query("action"); action {
	case "clear":
		for _, issue := range issues {
			if err := issue_service.ClearLabels(issue, ctx.User); err != nil {
				ctx.ServerError("ClearLabels", err)
				return
			}
		}
	case "attach", "detach", "toggle":
		label, err := models.GetLabelByID(ctx.QueryInt64("id"))
		if err != nil {
			if models.IsErrRepoLabelNotExist(err) {
				ctx.Error(404, "GetLabelByID")
			} else {
				ctx.ServerError("GetLabelByID", err)
			}
			return
		}

		if action == "toggle" {
			// detach if any issues already have label, otherwise attach
			action = "attach"
			for _, issue := range issues {
				if issue.HasLabel(label.ID) {
					action = "detach"
					break
				}
			}
		}

		if action == "attach" {
			for _, issue := range issues {
				if err = issue_service.AddLabel(issue, ctx.User, label); err != nil {
					ctx.ServerError("AddLabel", err)
					return
				}
			}
		} else {
			for _, issue := range issues {
				if err = issue_service.RemoveLabel(issue, ctx.User, label); err != nil {
					ctx.ServerError("RemoveLabel", err)
					return
				}
			}
		}
	default:
		log.Warn("Unrecognized action: %s", action)
		ctx.Error(500)
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"ok": true,
	})
}
