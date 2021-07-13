// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/web"
	archiver_service "code.gitea.io/gitea/services/archiver"
	"code.gitea.io/gitea/services/forms"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplCreate       base.TplName = "repo/create"
	tplAlertDetails base.TplName = "base/alert_details"
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
		ctx.Error(http.StatusForbidden)
		return nil
	}
	if !ctx.User.IsAdmin {
		canCreate, err := org.CanCreateOrgRepo(ctx.User.ID)
		if err != nil {
			ctx.ServerError("CanCreateOrgRepo", err)
			return nil
		} else if !canCreate {
			ctx.Error(http.StatusForbidden)
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
		if err == nil && templateRepo.CheckUnitUser(ctxUser, models.UnitTypeCode) {
			ctx.Data["repo_template"] = templateID
			ctx.Data["repo_template_name"] = templateRepo.Name
		}
	}

	ctx.Data["CanCreateRepo"] = ctx.User.CanCreateRepo()
	ctx.Data["MaxCreationLimit"] = ctx.User.MaxCreationLimit()

	ctx.HTML(http.StatusOK, tplCreate)
}

func handleCreateError(ctx *context.Context, owner *models.User, err error, name string, tpl base.TplName, form interface{}) {
	switch {
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
		ctx.ServerError(name, err)
	}
}

// CreatePost response for creating repository
func CreatePost(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.CreateRepoForm)
	ctx.Data["Title"] = ctx.Tr("new_repo")

	ctx.Data["Gitignores"] = models.Gitignores
	ctx.Data["LabelTemplates"] = models.LabelTemplates
	ctx.Data["Licenses"] = models.Licenses
	ctx.Data["Readmes"] = models.Readmes

	ctx.Data["CanCreateRepo"] = ctx.User.CanCreateRepo()
	ctx.Data["MaxCreationLimit"] = ctx.User.MaxCreationLimit()

	ctxUser := checkContextUser(ctx, form.UID)
	if ctx.Written() {
		return
	}
	ctx.Data["ContextUser"] = ctxUser

	if ctx.HasError() {
		ctx.HTML(http.StatusOK, tplCreate)
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
			ctx.Redirect(ctxUser.HomeLink() + "/" + repo.Name)
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
			IsTemplate:    form.Template,
			TrustModel:    models.ToTrustModel(form.TrustModel),
		})
		if err == nil {
			log.Trace("Repository created [%d]: %s/%s", repo.ID, ctxUser.Name, repo.Name)
			ctx.Redirect(ctxUser.HomeLink() + "/" + repo.Name)
			return
		}
	}

	handleCreateError(ctx, ctxUser, err, "CreatePost", tplCreate, &form)
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
	case "accept_transfer":
		err = acceptOrRejectRepoTransfer(ctx, true)
	case "reject_transfer":
		err = acceptOrRejectRepoTransfer(ctx, false)
	case "desc": // FIXME: this is not used
		if !ctx.Repo.IsOwner() {
			ctx.Error(http.StatusNotFound)
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

func acceptOrRejectRepoTransfer(ctx *context.Context, accept bool) error {
	repoTransfer, err := models.GetPendingRepositoryTransfer(ctx.Repo.Repository)
	if err != nil {
		return err
	}

	if err := repoTransfer.LoadAttributes(); err != nil {
		return err
	}

	if !repoTransfer.CanUserAcceptTransfer(ctx.User) {
		return errors.New("user does not have enough permissions")
	}

	if accept {
		if err := repo_service.TransferOwnership(repoTransfer.Doer, repoTransfer.Recipient, ctx.Repo.Repository, repoTransfer.Teams); err != nil {
			return err
		}
		ctx.Flash.Success(ctx.Tr("repo.settings.transfer.success"))
	} else {
		if err := models.CancelRepositoryTransfer(ctx.Repo.Repository); err != nil {
			return err
		}
		ctx.Flash.Success(ctx.Tr("repo.settings.transfer.rejected"))
	}

	ctx.Redirect(ctx.Repo.Repository.HTMLURL())
	return nil
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
			ctx.Error(http.StatusNotFound)
			return
		}
		ctx.ServerError("RedirectDownload", err)
		return
	}
	if len(releases) == 1 {
		release := releases[0]
		att, err := models.GetAttachmentByReleaseIDFileName(release.ID, fileName)
		if err != nil {
			ctx.Error(http.StatusNotFound)
			return
		}
		if att != nil {
			ctx.Redirect(att.DownloadURL())
			return
		}
	}
	ctx.Error(http.StatusNotFound)
}

// Download an archive of a repository
func Download(ctx *context.Context) {
	uri := ctx.Params("*")
	aReq, err := archiver_service.NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, uri)
	if err != nil {
		ctx.ServerError("archiver_service.NewRequest", err)
		return
	}
	if aReq == nil {
		ctx.Error(http.StatusNotFound)
		return
	}

	archiver, err := models.GetRepoArchiver(models.DefaultDBContext(), aReq.RepoID, aReq.Type, aReq.CommitID)
	if err != nil {
		ctx.ServerError("models.GetRepoArchiver", err)
		return
	}
	if archiver != nil && archiver.Status == models.RepoArchiverReady {
		download(ctx, aReq.GetArchiveName(), archiver)
		return
	}

	if err := archiver_service.StartArchive(aReq); err != nil {
		ctx.ServerError("archiver_service.StartArchive", err)
		return
	}

	var times int
	var t = time.NewTicker(time.Second * 1)
	defer t.Stop()

	for {
		select {
		case <-graceful.GetManager().HammerContext().Done():
			log.Warn("exit archive download because system stop")
			return
		case <-t.C:
			if times > 20 {
				ctx.ServerError("wait download timeout", nil)
				return
			}
			times++
			archiver, err = models.GetRepoArchiver(models.DefaultDBContext(), aReq.RepoID, aReq.Type, aReq.CommitID)
			if err != nil {
				ctx.ServerError("archiver_service.StartArchive", err)
				return
			}
			if archiver != nil && archiver.Status == models.RepoArchiverReady {
				download(ctx, aReq.GetArchiveName(), archiver)
				return
			}
		}
	}
}

func download(ctx *context.Context, archiveName string, archiver *models.RepoArchiver) {
	downloadName := ctx.Repo.Repository.Name + "-" + archiveName

	rPath, err := archiver.RelativePath()
	if err != nil {
		ctx.ServerError("archiver.RelativePath", err)
		return
	}

	if setting.RepoArchive.ServeDirect {
		//If we have a signed url (S3, object storage), redirect to this directly.
		u, err := storage.RepoArchives.URL(rPath, downloadName)
		if u != nil && err == nil {
			ctx.Redirect(u.String())
			return
		}
	}

	//If we have matched and access to release or issue
	fr, err := storage.RepoArchives.Open(rPath)
	if err != nil {
		ctx.ServerError("Open", err)
		return
	}
	defer fr.Close()
	ctx.ServeStream(fr, downloadName)
}

// InitiateDownload will enqueue an archival request, as needed.  It may submit
// a request that's already in-progress, but the archiver service will just
// kind of drop it on the floor if this is the case.
func InitiateDownload(ctx *context.Context) {
	uri := ctx.Params("*")
	aReq, err := archiver_service.NewRequest(ctx.Repo.Repository.ID, ctx.Repo.GitRepo, uri)
	if err != nil {
		ctx.ServerError("archiver_service.NewRequest", err)
		return
	}
	if aReq == nil {
		ctx.Error(http.StatusNotFound)
		return
	}

	archiver, err := models.GetRepoArchiver(models.DefaultDBContext(), aReq.RepoID, aReq.Type, aReq.CommitID)
	if err != nil {
		ctx.ServerError("archiver_service.StartArchive", err)
		return
	}
	if archiver == nil || archiver.Status != models.RepoArchiverReady {
		if err := archiver_service.StartArchive(aReq); err != nil {
			ctx.ServerError("archiver_service.StartArchive", err)
			return
		}
	}

	var completed bool
	if archiver != nil && archiver.Status == models.RepoArchiverReady {
		completed = true
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"complete": completed,
	})
}
