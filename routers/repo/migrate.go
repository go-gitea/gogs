// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	auth "code.gitea.io/gitea/modules/forms"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/task"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
)

const (
	tplMigrate base.TplName = "repo/migrate/migrate"
)

// Migrate render migration of repository page
func Migrate(ctx *context.Context) {
	if setting.Repository.DisableMigrations {
		ctx.Error(http.StatusForbidden, "Migrate: the site administrator has disabled migrations")
		return
	}

	serviceType := structs.GitServiceType(ctx.QueryInt("service_type"))

	setMigrationContextData(ctx, serviceType)

	if serviceType == 0 {
		ctx.Data["Org"] = ctx.Query("org")
		ctx.Data["Mirror"] = ctx.Query("mirror")

		ctx.HTML(200, tplMigrate)
		return
	}

	ctx.Data["private"] = getRepoPrivate(ctx)
	ctx.Data["mirror"] = ctx.Query("mirror") == "1"
	ctx.Data["wiki"] = ctx.Query("wiki") == "1"
	ctx.Data["milestones"] = ctx.Query("milestones") == "1"
	ctx.Data["labels"] = ctx.Query("labels") == "1"
	ctx.Data["issues"] = ctx.Query("issues") == "1"
	ctx.Data["pull_requests"] = ctx.Query("pull_requests") == "1"
	ctx.Data["releases"] = ctx.Query("releases") == "1"

	ctxUser := checkContextUser(ctx, ctx.QueryInt64("org"))
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	ctx.HTML(200, base.TplName("repo/migrate/"+serviceType.Name()))
}

func handleMigrateError(ctx *context.Context, owner *models.User, err error, name string, tpl base.TplName, form *auth.MigrateRepoForm) {
	if setting.Repository.DisableMigrations {
		ctx.Error(http.StatusForbidden, "MigrateError: the site administrator has disabled migrations")
		return
	}

	switch {
	case migrations.IsRateLimitError(err):
		ctx.RenderWithErr(ctx.Tr("form.visit_rate_limit"), tpl, form)
	case migrations.IsTwoFactorAuthError(err):
		ctx.RenderWithErr(ctx.Tr("form.2fa_auth_required"), tpl, form)
	case models.IsErrReachLimitOfRepo(err):
		ctx.RenderWithErr(ctx.Tr("repo.form.reach_limit_of_creation", owner.MaxCreationLimit()), tpl, form)
	case models.IsErrRepoAlreadyExist(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("form.repo_name_been_taken"), tpl, form)
	case models.IsErrRepoFilesAlreadyExist(err):
		ctx.Data["Err_RepoName"] = true
		switch {
		case ctx.IsUserSiteAdmin() || (setting.Repository.AllowAdoptionOfUnadoptedRepositories && setting.Repository.AllowDeleteOfUnadoptedRepositories):
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.adopt_or_delete"), tpl, form)
		case setting.Repository.AllowAdoptionOfUnadoptedRepositories:
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.adopt"), tpl, form)
		case setting.Repository.AllowDeleteOfUnadoptedRepositories:
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist.delete"), tpl, form)
		default:
			ctx.RenderWithErr(ctx.Tr("form.repository_files_already_exist"), tpl, form)
		}
	case models.IsErrNameReserved(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(models.ErrNameReserved).Name), tpl, form)
	case models.IsErrNamePatternNotAllowed(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tpl, form)
	default:
		remoteAddr, _ := auth.ParseRemoteAddr(form.CloneAddr, form.AuthUsername, form.AuthPassword)
		err = util.URLSanitizedError(err, remoteAddr)
		if strings.Contains(err.Error(), "Authentication failed") ||
			strings.Contains(err.Error(), "Bad credentials") ||
			strings.Contains(err.Error(), "could not read Username") {
			ctx.Data["Err_Auth"] = true
			ctx.RenderWithErr(ctx.Tr("form.auth_failed", err.Error()), tpl, form)
		} else if strings.Contains(err.Error(), "fatal:") {
			ctx.Data["Err_CloneAddr"] = true
			ctx.RenderWithErr(ctx.Tr("repo.migrate.failed", err.Error()), tpl, form)
		} else {
			ctx.ServerError(name, err)
		}
	}
}

// MigratePost response for migrating from external git repository
func MigratePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*auth.MigrateRepoForm)
	if setting.Repository.DisableMigrations {
		ctx.Error(http.StatusForbidden, "MigratePost: the site administrator has disabled migrations")
		return
	}

	serviceType := structs.GitServiceType(form.Service)

	setMigrationContextData(ctx, serviceType)

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	tpl := base.TplName("repo/migrate/" + serviceType.Name())

	if ctx.HasError() {
		ctx.HTML(200, tpl)
		return
	}

	remoteAddr, err := auth.ParseRemoteAddr(form.CloneAddr, form.AuthUsername, form.AuthPassword)
	if err == nil {
		err = migrations.IsMigrateURLAllowed(remoteAddr, ctx.User)
	}
	if err != nil {
		if models.IsErrInvalidCloneAddr(err) {
			ctx.Data["Err_CloneAddr"] = true
			addrErr := err.(*models.ErrInvalidCloneAddr)
			switch {
			case addrErr.IsProtocolInvalid:
				ctx.RenderWithErr(ctx.Tr("repo.mirror_address_protocol_invalid"), tpl, &form)
			case addrErr.IsURLError:
				ctx.RenderWithErr(ctx.Tr("form.url_error"), tpl, &form)
			case addrErr.IsPermissionDenied:
				if addrErr.LocalPath {
					ctx.RenderWithErr(ctx.Tr("repo.migrate.permission_denied"), tpl, &form)
				} else if len(addrErr.PrivateNet) == 0 {
					ctx.RenderWithErr(ctx.Tr("repo.migrate.permission_denied_blocked"), tpl, &form)
				} else {
					ctx.RenderWithErr(ctx.Tr("repo.migrate.permission_denied_private_ip"), tpl, &form)
				}
			case addrErr.IsInvalidPath:
				ctx.RenderWithErr(ctx.Tr("repo.migrate.invalid_local_path"), tpl, &form)
			default:
				log.Error("Error whilst updating url: %v", err)
				ctx.RenderWithErr(ctx.Tr("form.url_error"), tpl, &form)
			}
		} else {
			log.Error("Error whilst updating url: %v", err)
			ctx.RenderWithErr(ctx.Tr("form.url_error"), tpl, &form)
		}
		return
	}

	opts := migrations.MigrateOptions{
		OriginalURL:    form.CloneAddr,
		GitServiceType: serviceType,
		CloneAddr:      remoteAddr,
		RepoName:       form.RepoName,
		Description:    form.Description,
		Private:        form.Private || setting.Repository.ForcePrivate,
		Mirror:         form.Mirror && !setting.Repository.DisableMirrors,
		AuthUsername:   form.AuthUsername,
		AuthPassword:   form.AuthPassword,
		AuthToken:      form.AuthToken,
		Wiki:           form.Wiki,
		Issues:         form.Issues,
		Milestones:     form.Milestones,
		Labels:         form.Labels,
		Comments:       form.Issues || form.PullRequests,
		PullRequests:   form.PullRequests,
		Releases:       form.Releases,
	}
	if opts.Mirror {
		opts.Issues = false
		opts.Milestones = false
		opts.Labels = false
		opts.Comments = false
		opts.PullRequests = false
		opts.Releases = false
	}

	err = models.CheckCreateRepository(ctx.User, ctxUser, opts.RepoName, false)
	if err != nil {
		handleMigrateError(ctx, ctxUser, err, "MigratePost", tpl, form)
		return
	}

	err = task.MigrateRepository(ctx.User, ctxUser, opts)
	if err == nil {
		ctx.Redirect(setting.AppSubURL + "/" + ctxUser.Name + "/" + opts.RepoName)
		return
	}

	handleMigrateError(ctx, ctxUser, err, "MigratePost", tpl, form)
}

func setMigrationContextData(ctx *context.Context, serviceType structs.GitServiceType) {
	ctx.Data["Title"] = ctx.Tr("new_migrate")

	ctx.Data["LFSActive"] = setting.LFS.StartServer
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate
	ctx.Data["DisableMirrors"] = setting.Repository.DisableMirrors

	// Plain git should be first
	ctx.Data["Services"] = append([]structs.GitServiceType{structs.PlainGitService}, structs.SupportedFullGitService...)
	ctx.Data["service"] = serviceType
}
