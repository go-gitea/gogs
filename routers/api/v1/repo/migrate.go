// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/notification"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
)

// Migrate migrate remote git repository to gitea
func Migrate(ctx *context.APIContext, form auth.MigrateRepoForm) {
	// swagger:operation POST /repos/migrate repository repoMigrate
	// ---
	// summary: Migrate a remote git repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/MigrateRepoForm"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Repository"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	ctxUser := ctx.User
	// Not equal means context user is an organization,
	// or is another user/organization if current user is admin.
	if form.UID != ctxUser.ID {
		org, err := models.GetUserByID(form.UID)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "", err)
			} else {
				ctx.Error(http.StatusInternalServerError, "GetUserByID", err)
			}
			return
		}
		ctxUser = org
	}

	if ctx.HasError() {
		ctx.Error(http.StatusUnprocessableEntity, "", ctx.GetErrMsg())
		return
	}

	if !ctx.User.IsAdmin {
		if !ctxUser.IsOrganization() && ctx.User.ID != ctxUser.ID {
			ctx.Error(http.StatusForbidden, "", "Given user is not an organization.")
			return
		}

		if ctxUser.IsOrganization() {
			// Check ownership of organization.
			isOwner, err := ctxUser.IsOwnedBy(ctx.User.ID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "IsOwnedBy", err)
				return
			} else if !isOwner {
				ctx.Error(http.StatusForbidden, "", "Given user is not owner of organization.")
				return
			}
		}
	}

	remoteAddr, err := form.ParseRemoteAddr(ctx.User)
	if err != nil {
		if models.IsErrInvalidCloneAddr(err) {
			addrErr := err.(models.ErrInvalidCloneAddr)
			switch {
			case addrErr.IsURLError:
				ctx.Error(http.StatusUnprocessableEntity, "", err)
			case addrErr.IsPermissionDenied:
				ctx.Error(http.StatusUnprocessableEntity, "", "You are not allowed to import local repositories.")
			case addrErr.CantUseSSHServerCredentials:
				ctx.Error(http.StatusUnprocessableEntity, "", "You are not allowed to use server credentials to import repositories via SSH.")
			case addrErr.IsInvalidPath:
				ctx.Error(http.StatusUnprocessableEntity, "", "Invalid local path, it does not exist or not a directory.")
			default:
				ctx.Error(http.StatusInternalServerError, "ParseRemoteAddr", "Unknown error type (ErrInvalidCloneAddr): "+err.Error())
			}
		} else {
			ctx.Error(http.StatusInternalServerError, "ParseRemoteAddr", err)
		}
		return
	}

	var gitServiceType = api.PlainGitService
	u, err := url.Parse(remoteAddr)
	if err == nil && strings.EqualFold(u.Host, "github.com") {
		gitServiceType = api.GithubService
	}

	if form.Mirror && setting.Repository.DisableMirrors {
		ctx.Error(http.StatusForbidden, "MirrorsGlobalDisabled", fmt.Errorf("the site administrator has disabled mirrors"))
		return
	}

	var opts = migrations.MigrateOptions{
		CloneAddr:      remoteAddr,
		RepoName:       form.RepoName,
		Description:    form.Description,
		Private:        form.Private || setting.Repository.ForcePrivate,
		Mirror:         form.Mirror,
		AuthUsername:   form.AuthUsername,
		AuthPassword:   form.AuthPassword,
		Wiki:           form.Wiki,
		Issues:         form.Issues,
		Milestones:     form.Milestones,
		Labels:         form.Labels,
		Comments:       true,
		PullRequests:   form.PullRequests,
		Releases:       form.Releases,
		GitServiceType: gitServiceType,
	}
	if opts.Mirror {
		opts.Issues = false
		opts.Milestones = false
		opts.Labels = false
		opts.Comments = false
		opts.PullRequests = false
		opts.Releases = false
	}

	repo, err := repo_module.CreateRepository(ctx.User, ctxUser, models.CreateRepoOptions{
		Name:           opts.RepoName,
		Description:    opts.Description,
		OriginalURL:    form.CloneAddr,
		GitServiceType: gitServiceType,
		IsPrivate:      opts.Private,
		IsMirror:       opts.Mirror,
		Status:         models.RepositoryBeingMigrated,
	})
	if err != nil {
		handleMigrateError(ctx, ctxUser, remoteAddr, err)
		return
	}

	opts.MigrateToRepoID = repo.ID

	defer func() {
		if e := recover(); e != nil {
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "Handler crashed with error: %v", log.Stack(2))

			err = errors.New(buf.String())
		}

		if err == nil {
			repo.Status = models.RepositoryReady
			if err := models.UpdateRepositoryCols(repo, "status"); err == nil {
				notification.NotifyMigrateRepository(ctx.User, ctxUser, repo)
				return
			}
		}

		if repo != nil {
			if errDelete := models.DeleteRepository(ctx.User, ctxUser.ID, repo.ID); errDelete != nil {
				log.Error("DeleteRepository: %v", errDelete)
			}
		}
	}()

	if _, err = migrations.MigrateRepository(graceful.GetManager().HammerContext(), ctx.User, ctxUser.Name, opts); err != nil {
		handleMigrateError(ctx, ctxUser, remoteAddr, err)
		return
	}

	log.Trace("Repository migrated: %s/%s", ctxUser.Name, form.RepoName)
	ctx.JSON(http.StatusCreated, repo.APIFormat(models.AccessModeAdmin))
}

func handleMigrateError(ctx *context.APIContext, repoOwner *models.User, remoteAddr string, err error) {
	switch {
	case models.IsErrRepoAlreadyExist(err):
		ctx.Error(http.StatusConflict, "", "The repository with the same name already exists.")
	case migrations.IsRateLimitError(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "Remote visit addressed rate limitation.")
	case migrations.IsTwoFactorAuthError(err):
		ctx.Error(http.StatusUnprocessableEntity, "", "Remote visit required two factors authentication.")
	case models.IsErrReachLimitOfRepo(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("You have already reached your limit of %d repositories.", repoOwner.MaxCreationLimit()))
	case models.IsErrNameReserved(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("The username '%s' is reserved.", err.(models.ErrNameReserved).Name))
	case models.IsErrNameCharsNotAllowed(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("The username '%s' contains invalid characters.", err.(models.ErrNameCharsNotAllowed).Name))
	case models.IsErrNamePatternNotAllowed(err):
		ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("The pattern '%s' is not allowed in a username.", err.(models.ErrNamePatternNotAllowed).Pattern))
	default:
		err = util.URLSanitizedError(err, remoteAddr)
		if strings.Contains(err.Error(), "Authentication failed") ||
			strings.Contains(err.Error(), "Bad credentials") ||
			strings.Contains(err.Error(), "could not read Username") {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Authentication failed: %v.", err))
		} else if strings.Contains(err.Error(), "fatal:") {
			ctx.Error(http.StatusUnprocessableEntity, "", fmt.Sprintf("Migration failed: %v.", err))
		} else {
			ctx.Error(http.StatusInternalServerError, "MigrateRepository", err)
		}
	}
}
