// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	auth "code.gitea.io/gitea/modules/forms"
	"code.gitea.io/gitea/modules/web"
)

// LockIssue locks an issue. This would limit commenting abilities to
// users with write access to the repo.
func LockIssue(ctx *context.Context) {
	form := web.GetForm(ctx).(*auth.IssueLockForm)
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if issue.IsLocked {
		ctx.Flash.Error(ctx.Tr("repo.issues.lock_duplicate"))
		ctx.Redirect(issue.HTMLURL())
		return
	}

	if !form.HasValidReason() {
		ctx.Flash.Error(ctx.Tr("repo.issues.lock.unknown_reason"))
		ctx.Redirect(issue.HTMLURL())
		return
	}

	if err := models.LockIssue(&models.IssueLockOptions{
		Doer:   ctx.User,
		Issue:  issue,
		Reason: form.Reason,
	}); err != nil {
		ctx.ServerError("LockIssue", err)
		return
	}

	ctx.Redirect(issue.HTMLURL(), http.StatusSeeOther)
}

// UnlockIssue unlocks a previously locked issue.
func UnlockIssue(ctx *context.Context) {
	issue := GetActionIssue(ctx)
	if ctx.Written() {
		return
	}

	if !issue.IsLocked {
		ctx.Flash.Error(ctx.Tr("repo.issues.unlock_error"))
		ctx.Redirect(issue.HTMLURL())
		return
	}

	if err := models.UnlockIssue(&models.IssueLockOptions{
		Doer:  ctx.User,
		Issue: issue,
	}); err != nil {
		ctx.ServerError("UnlockIssue", err)
		return
	}

	ctx.Redirect(issue.HTMLURL(), http.StatusSeeOther)
}
