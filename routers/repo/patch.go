// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/repofiles"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplPatchFile base.TplName = "repo/editor/patch"
)

// NewDiffPatch render create patch page
func NewDiffPatch(ctx *context.Context) {
	ctx.Data["RequireHighlightJS"] = true

	canCommit := renderCommitRights(ctx)

	ctx.Data["TreePath"] = "patch"

	ctx.Data["commit_summary"] = ""
	ctx.Data["commit_message"] = ""
	if canCommit {
		ctx.Data["commit_choice"] = frmCommitChoiceDirect
	} else {
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
	}
	ctx.Data["new_branch_name"] = GetUniquePatchBranchName(ctx)
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL()

	ctx.HTML(200, tplPatchFile)
}

// NewDiffPatchPost response for sending patch page
func NewDiffPatchPost(ctx *context.Context, form auth.EditRepoFileForm) {
	canCommit := renderCommitRights(ctx)
	branchName := ctx.Repo.BranchName
	if form.CommitChoice == frmCommitChoiceNewBranch {
		branchName = form.NewBranchName
	}
	ctx.Data["RequireHighlightJS"] = true
	ctx.Data["TreePath"] = "patch"
	ctx.Data["BranchLink"] = ctx.Repo.RepoLink + "/src/branch/" + ctx.Repo.BranchName
	ctx.Data["FileContent"] = form.Content
	ctx.Data["commit_summary"] = form.CommitSummary
	ctx.Data["commit_message"] = form.CommitMessage
	ctx.Data["commit_choice"] = form.CommitChoice
	ctx.Data["new_branch_name"] = form.NewBranchName
	ctx.Data["last_commit"] = ctx.Repo.CommitID
	ctx.Data["LineWrapExtensions"] = strings.Join(setting.Repository.Editor.LineWrapExtensions, ",")

	if ctx.HasError() {
		ctx.HTML(200, tplPatchFile)
		return
	}

	// Cannot commit to a an existing branch if user doesn't have rights
	if branchName == ctx.Repo.BranchName && !canCommit {
		ctx.Data["Err_NewBranchName"] = true
		ctx.Data["commit_choice"] = frmCommitChoiceNewBranch
		ctx.RenderWithErr(ctx.Tr("repo.editor.cannot_commit_to_protected_branch", branchName), tplEditFile, &form)
		return
	}

	// CommitSummary is optional in the web form, if empty, give it a default message based on add or update
	// `message` will be both the summary and message combined
	message := strings.TrimSpace(form.CommitSummary)
	if len(message) == 0 {
		message = ctx.Tr("repo.editor.patch")
	}

	form.CommitMessage = strings.TrimSpace(form.CommitMessage)
	if len(form.CommitMessage) > 0 {
		message += "\n\n" + form.CommitMessage
	}

	if _, err := repofiles.ApplyDiffPatch(ctx.Repo.Repository, ctx.User, &repofiles.ApplyDiffPatchOptions{
		LastCommitID: form.LastCommit,
		OldBranch:    ctx.Repo.BranchName,
		NewBranch:    branchName,
		Message:      message,
		Content:      strings.Replace(form.Content, "\r", "", -1),
	}); err != nil {
		if models.IsErrBranchAlreadyExists(err) {
			// For when a user specifies a new branch that already exists
			ctx.Data["Err_NewBranchName"] = true
			if branchErr, ok := err.(models.ErrBranchAlreadyExists); ok {
				ctx.RenderWithErr(ctx.Tr("repo.editor.branch_already_exists", branchErr.BranchName), tplEditFile, &form)
			} else {
				ctx.Error(500, err.Error())
			}
		} else if models.IsErrCommitIDDoesNotMatch(err) {
			ctx.RenderWithErr(ctx.Tr("repo.editor.file_changed_while_editing", ctx.Repo.RepoLink+"/compare/"+form.LastCommit+"..."+ctx.Repo.CommitID), tplPatchFile, &form)
		} else {
			ctx.RenderWithErr(ctx.Tr("repo.editor.fail_to_apply_patch", err), tplPatchFile, &form)
		}
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/compare/" + ctx.Repo.BranchName + "..." + form.NewBranchName)
}
