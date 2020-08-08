// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/task"
	"code.gitea.io/gitea/modules/util"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/unknwon/com"
)

const (
	tplCreate  base.TplName = "repo/create"
	tplMigrate base.TplName = "repo/migrate"
)

// MustBeNotEmpty render when a repo is a empty git dir
func MustBeNotEmpty(ctx *context.Context) {
	if ctx.Repo.Repository.IsEmpty {
		ctx.NotFound("MustBeNotEmpty", nil)
	}
}

// MustBeEditable check that repo can be edited
func MustBeEditable(ctx *context.Context) {
	if !ctx.Repo.Repository.CanEnableEditor() || ctx.Repo.IsViewCommit {
		ctx.NotFound("", nil)
		return
	}
}

// MustBeAbleToUpload check that repo can be uploaded to
func MustBeAbleToUpload(ctx *context.Context) {
	if !setting.Repository.Upload.Enabled {
		ctx.NotFound("", nil)
	}
}

func checkContextUser(ctx *context.Context, uid int64) *models.User {
	orgs, err := models.GetOrgsCanCreateRepoByUserID(ctx.User.ID)
	if err != nil {
		ctx.ServerError("GetOrgsCanCreateRepoByUserID", err)
		return nil
	}

	if !ctx.User.IsAdmin {
		orgsAvailable := []*models.User{}
		for i := 0; i < len(orgs); i++ {
			if orgs[i].CanCreateRepo() {
				orgsAvailable = append(orgsAvailable, orgs[i])
			}
		}
		ctx.Data["Orgs"] = orgsAvailable
	} else {
		ctx.Data["Orgs"] = orgs
	}

	// Not equal means current user is an organization.
	if uid == ctx.User.ID || uid == 0 {
		return ctx.User
	}

	org, err := models.GetUserByID(uid)
	if models.IsErrUserNotExist(err) {
		return ctx.User
	}

	if err != nil {
		ctx.ServerError("GetUserByID", fmt.Errorf("[%d]: %v", uid, err))
		return nil
	}

	// Check ownership of organization.
	if !org.IsOrganization() {
		ctx.Error(403)
		return nil
	}
	if !ctx.User.IsAdmin {
		canCreate, err := org.CanCreateOrgRepo(ctx.User.ID)
		if err != nil {
			ctx.ServerError("CanCreateOrgRepo", err)
			return nil
		} else if !canCreate {
			ctx.Error(403)
			return nil
		}
	} else {
		ctx.Data["Orgs"] = orgs
	}
	return org
}

func getRepoPrivate(ctx *context.Context) bool {
	switch strings.ToLower(setting.Repository.DefaultPrivate) {
	case setting.RepoCreatingLastUserVisibility:
		return ctx.User.LastRepoVisibility
	case setting.RepoCreatingPrivate:
		return true
	case setting.RepoCreatingPublic:
		return false
	default:
		return ctx.User.LastRepoVisibility
	}
}

// Create render creating repository page
func Create(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("new_repo")

	// Give default value for template to render.
	ctx.Data["Gitignores"] = models.Gitignores
	ctx.Data["LabelTemplates"] = models.LabelTemplates
	ctx.Data["Licenses"] = models.Licenses
	ctx.Data["Readmes"] = models.Readmes
	ctx.Data["readme"] = "Default"
	ctx.Data["private"] = getRepoPrivate(ctx)
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate
	ctx.Data["default_branch"] = setting.Repository.DefaultBranch

	ctxUser := checkContextUser(ctx, ctx.QueryInt64("org"))
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	ctx.Data["repo_template_name"] = ctx.Tr("repo.template_select")
	templateID := ctx.QueryInt64("template_id")
	if templateID > 0 {
		templateRepo, err := models.GetRepositoryByID(templateID)
		if err == nil && templateRepo.CheckUnitUser(ctxUser.ID, ctxUser.IsAdmin, models.UnitTypeCode) {
			ctx.Data["repo_template"] = templateID
			ctx.Data["repo_template_name"] = templateRepo.Name
		}
	}

	if !ctx.User.CanCreateRepo() {
		ctx.RenderWithErr(ctx.Tr("repo.form.reach_limit_of_creation", ctx.User.MaxCreationLimit()), tplCreate, nil)
	} else {
		ctx.HTML(200, tplCreate)
	}
}

func handleCreateError(ctx *context.Context, owner *models.User, err error, name string, tpl base.TplName, form interface{}) {
	switch {
	case models.IsErrReachLimitOfRepo(err):
		ctx.RenderWithErr(ctx.Tr("repo.form.reach_limit_of_creation", owner.MaxCreationLimit()), tpl, form)
	case models.IsErrRepoAlreadyExist(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("form.repo_name_been_taken"), tpl, form)
	case models.IsErrNameReserved(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(models.ErrNameReserved).Name), tpl, form)
	case models.IsErrNamePatternNotAllowed(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tpl, form)
	default:
		ctx.ServerError(name, err)
	}
}

// CreatePost response for creating repository
func CreatePost(ctx *context.Context, form auth.CreateRepoForm) {
	ctx.Data["Title"] = ctx.Tr("new_repo")

	ctx.Data["Gitignores"] = models.Gitignores
	ctx.Data["LabelTemplates"] = models.LabelTemplates
	ctx.Data["Licenses"] = models.Licenses
	ctx.Data["Readmes"] = models.Readmes

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	if ctx.HasError() {
		ctx.HTML(200, tplCreate)
		return
	}

	var repo *models.Repository
	var err error
	if form.RepoTemplate > 0 {
		opts := models.GenerateRepoOptions{
			Name:        form.RepoName,
			Description: form.Description,
			Private:     form.Private,
			GitContent:  form.GitContent,
			Topics:      form.Topics,
			GitHooks:    form.GitHooks,
			Webhooks:    form.Webhooks,
			Avatar:      form.Avatar,
			IssueLabels: form.Labels,
		}

		if !opts.IsValid() {
			ctx.RenderWithErr(ctx.Tr("repo.template.one_item"), tplCreate, form)
			return
		}

		templateRepo := getRepository(ctx, form.RepoTemplate)
		if ctx.Written() {
			return
		}

		if !templateRepo.IsTemplate {
			ctx.RenderWithErr(ctx.Tr("repo.template.invalid"), tplCreate, form)
			return
		}

		repo, err = repo_service.GenerateRepository(ctx.User, ctxUser, templateRepo, opts)
		if err == nil {
			log.Trace("Repository generated [%d]: %s/%s", repo.ID, ctxUser.Name, repo.Name)
			ctx.Redirect(setting.AppSubURL + "/" + ctxUser.Name + "/" + repo.Name)
			return
		}
	} else {
		repo, err = repo_service.CreateRepository(ctx.User, ctxUser, models.CreateRepoOptions{
			Name:          form.RepoName,
			Description:   form.Description,
			Gitignores:    form.Gitignores,
			IssueLabels:   form.IssueLabels,
			License:       form.License,
			Readme:        form.Readme,
			IsPrivate:     form.Private || setting.Repository.ForcePrivate,
			DefaultBranch: form.DefaultBranch,
			AutoInit:      form.AutoInit,
		})
		if err == nil {
			log.Trace("Repository created [%d]: %s/%s", repo.ID, ctxUser.Name, repo.Name)
			ctx.Redirect(setting.AppSubURL + "/" + ctxUser.Name + "/" + repo.Name)
			return
		}
	}

	handleCreateError(ctx, ctxUser, err, "CreatePost", tplCreate, &form)
}

// Migrate render migration of repository page
func Migrate(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("new_migrate")
	ctx.Data["private"] = getRepoPrivate(ctx)
	ctx.Data["IsForcedPrivate"] = setting.Repository.ForcePrivate
	ctx.Data["DisableMirrors"] = setting.Repository.DisableMirrors
	ctx.Data["mirror"] = ctx.Query("mirror") == "1"
	ctx.Data["wiki"] = ctx.Query("wiki") == "1"
	ctx.Data["milestones"] = ctx.Query("milestones") == "1"
	ctx.Data["labels"] = ctx.Query("labels") == "1"
	ctx.Data["issues"] = ctx.Query("issues") == "1"
	ctx.Data["pull_requests"] = ctx.Query("pull_requests") == "1"
	ctx.Data["releases"] = ctx.Query("releases") == "1"
	ctx.Data["LFSActive"] = setting.LFS.StartServer

	ctxUser := checkContextUser(ctx, ctx.QueryInt64("org"))
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	ctx.HTML(200, tplMigrate)
}

func handleMigrateError(ctx *context.Context, owner *models.User, err error, name string, tpl base.TplName, form *auth.MigrateRepoForm) {
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
	case models.IsErrNameReserved(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(models.ErrNameReserved).Name), tpl, form)
	case models.IsErrNamePatternNotAllowed(err):
		ctx.Data["Err_RepoName"] = true
		ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tpl, form)
	default:
		remoteAddr, _ := form.ParseRemoteAddr(owner)
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
func MigratePost(ctx *context.Context, form auth.MigrateRepoForm) {
	ctx.Data["Title"] = ctx.Tr("new_migrate")

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	if ctx.HasError() {
		ctx.HTML(200, tplMigrate)
		return
	}

	remoteAddr, err := form.ParseRemoteAddr(ctx.User)
	if err != nil {
		if models.IsErrInvalidCloneAddr(err) {
			ctx.Data["Err_CloneAddr"] = true
			addrErr := err.(models.ErrInvalidCloneAddr)
			switch {
			case addrErr.IsURLError:
				ctx.RenderWithErr(ctx.Tr("form.url_error"), tplMigrate, &form)
			case addrErr.IsPermissionDenied:
				ctx.RenderWithErr(ctx.Tr("repo.migrate.permission_denied"), tplMigrate, &form)
			case addrErr.CantUseSSHServerCredentials:
				ctx.RenderWithErr(ctx.Tr("repo.migrate.cant_use_ssh_server_credentials"), tplMigrate, &form)
			case addrErr.IsInvalidPath:
				ctx.RenderWithErr(ctx.Tr("repo.migrate.invalid_local_path"), tplMigrate, &form)
			default:
				ctx.ServerError("Unknown error", err)
			}
		} else {
			ctx.ServerError("ParseRemoteAddr", err)
		}
		return
	}

	var gitServiceType = structs.PlainGitService
	u, err := url.Parse(form.CloneAddr)
	if err == nil && strings.EqualFold(u.Host, "github.com") {
		gitServiceType = structs.GithubService
	}

	var opts = migrations.MigrateOptions{
		OriginalURL:    form.CloneAddr,
		GitServiceType: gitServiceType,
		CloneAddr:      remoteAddr,
		RepoName:       form.RepoName,
		Description:    form.Description,
		Private:        form.Private || setting.Repository.ForcePrivate,
		Mirror:         form.Mirror && !setting.Repository.DisableMirrors,
		AuthUsername:   form.AuthUsername,
		AuthPassword:   form.AuthPassword,
		Wiki:           form.Wiki,
		Issues:         form.Issues,
		Milestones:     form.Milestones,
		Labels:         form.Labels,
		Comments:       true,
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

	err = models.CheckCreateRepository(ctx.User, ctxUser, opts.RepoName)
	if err != nil {
		handleMigrateError(ctx, ctxUser, err, "MigratePost", tplMigrate, &form)
		return
	}

	err = task.MigrateRepository(ctx.User, ctxUser, opts)
	if err == nil {
		ctx.Redirect(setting.AppSubURL + "/" + ctxUser.Name + "/" + opts.RepoName)
		return
	}

	handleMigrateError(ctx, ctxUser, err, "MigratePost", tplMigrate, &form)
}

// Action response for actions to a repository
func Action(ctx *context.Context) {
	var err error
	switch ctx.Params(":action") {
	case "watch":
		err = models.WatchRepo(ctx.User.ID, ctx.Repo.Repository.ID, true)
	case "unwatch":
		err = models.WatchRepo(ctx.User.ID, ctx.Repo.Repository.ID, false)
	case "star":
		err = models.StarRepo(ctx.User.ID, ctx.Repo.Repository.ID, true)
	case "unstar":
		err = models.StarRepo(ctx.User.ID, ctx.Repo.Repository.ID, false)
	case "desc": // FIXME: this is not used
		if !ctx.Repo.IsOwner() {
			ctx.Error(404)
			return
		}

		ctx.Repo.Repository.Description = ctx.Query("desc")
		ctx.Repo.Repository.Website = ctx.Query("site")
		err = models.UpdateRepository(ctx.Repo.Repository, false)
	}

	if err != nil {
		ctx.ServerError(fmt.Sprintf("Action (%s)", ctx.Params(":action")), err)
		return
	}

	ctx.RedirectToFirst(ctx.Query("redirect_to"), ctx.Repo.RepoLink)
}

// RedirectDownload return a file based on the following infos:
func RedirectDownload(ctx *context.Context) {
	var (
		vTag     = ctx.Params("vTag")
		fileName = ctx.Params("fileName")
	)
	tagNames := []string{vTag}
	curRepo := ctx.Repo.Repository
	releases, err := models.GetReleasesByRepoIDAndNames(models.DefaultDBContext(), curRepo.ID, tagNames)
	if err != nil {
		if models.IsErrAttachmentNotExist(err) {
			ctx.Error(404)
			return
		}
		ctx.ServerError("RedirectDownload", err)
		return
	}
	if len(releases) == 1 {
		release := releases[0]
		att, err := models.GetAttachmentByReleaseIDFileName(release.ID, fileName)
		if err != nil {
			ctx.Error(404)
			return
		}
		if att != nil {
			ctx.Redirect(att.DownloadURL())
			return
		}
	}
	ctx.Error(404)
}

// Download download an archive of a repository
func Download(ctx *context.Context) {
	var (
		uri         = ctx.Params("*")
		refName     string
		ext         string
		archivePath string
		archiveType git.ArchiveType
	)

	switch {
	case strings.HasSuffix(uri, ".zip"):
		ext = ".zip"
		archivePath = path.Join(ctx.Repo.GitRepo.Path, "archives/zip")
		archiveType = git.ZIP
	case strings.HasSuffix(uri, ".tar.gz"):
		ext = ".tar.gz"
		archivePath = path.Join(ctx.Repo.GitRepo.Path, "archives/targz")
		archiveType = git.TARGZ
	default:
		log.Trace("Unknown format: %s", uri)
		ctx.Error(404)
		return
	}
	refName = strings.TrimSuffix(uri, ext)

	if !com.IsDir(archivePath) {
		if err := os.MkdirAll(archivePath, os.ModePerm); err != nil {
			ctx.ServerError("Download -> os.MkdirAll(archivePath)", err)
			return
		}
	}

	// Get corresponding commit.
	var (
		commit *git.Commit
		err    error
	)
	gitRepo := ctx.Repo.GitRepo
	if gitRepo.IsBranchExist(refName) {
		commit, err = gitRepo.GetBranchCommit(refName)
		if err != nil {
			ctx.ServerError("GetBranchCommit", err)
			return
		}
	} else if gitRepo.IsTagExist(refName) {
		commit, err = gitRepo.GetTagCommit(refName)
		if err != nil {
			ctx.ServerError("GetTagCommit", err)
			return
		}
	} else if len(refName) >= 4 && len(refName) <= 40 {
		commit, err = gitRepo.GetCommit(refName)
		if err != nil {
			ctx.NotFound("GetCommit", nil)
			return
		}
	} else {
		ctx.NotFound("Download", nil)
		return
	}

	archivePath = path.Join(archivePath, base.ShortSha(commit.ID.String())+ext)
	if !com.IsFile(archivePath) {
		if err := commit.CreateArchive(archivePath, git.CreateArchiveOpts{
			Format: archiveType,
			Prefix: setting.Repository.PrefixArchiveFiles,
		}); err != nil {
			ctx.ServerError("Download -> CreateArchive "+archivePath, err)
			return
		}
	}

	ctx.ServeFile(archivePath, ctx.Repo.Repository.Name+"-"+refName+ext)
}

// Status returns repository's status
func Status(ctx *context.Context) {
	task, err := models.GetMigratingTask(ctx.Repo.Repository.ID)
	if err != nil {
		ctx.JSON(500, map[string]interface{}{
			"err": err,
		})
		return
	}

	ctx.JSON(200, map[string]interface{}{
		"status": ctx.Repo.Repository.Status,
		"err":    task.Errors,
	})
}
