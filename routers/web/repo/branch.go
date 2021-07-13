// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repofiles"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/utils"
	"code.gitea.io/gitea/services/forms"
	release_service "code.gitea.io/gitea/services/release"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	tplBranch base.TplName = "repo/branch/list"
)

// Branch contains the branch information
type Branch struct {
	Name              string
	Commit            *git.Commit
	IsProtected       bool
	IsDeleted         bool
	IsIncluded        bool
	DeletedBranch     *models.DeletedBranch
	CommitsAhead      int
	CommitsBehind     int
	LatestPullRequest *models.PullRequest
	MergeMovedOn      bool
}

// Branches render repository branch page
func Branches(ctx *context.Context) {
	ctx.Data["Title"] = "Branches"
	ctx.Data["IsRepoToolbarBranches"] = true
	ctx.Data["DefaultBranch"] = ctx.Repo.Repository.DefaultBranch
	ctx.Data["AllowsPulls"] = ctx.Repo.Repository.AllowsPulls()
	ctx.Data["IsWriter"] = ctx.Repo.CanWrite(models.UnitTypeCode)
	ctx.Data["IsMirror"] = ctx.Repo.Repository.IsMirror
	ctx.Data["CanPull"] = ctx.Repo.CanWrite(models.UnitTypeCode) || (ctx.IsSigned && ctx.User.HasForkedRepo(ctx.Repo.Repository.ID))
	ctx.Data["PageIsViewCode"] = true
	ctx.Data["PageIsBranches"] = true

	page := ctx.QueryInt("page")
	if page <= 1 {
		page = 1
	}

	limit := ctx.QueryInt("limit")
	if limit <= 0 || limit > setting.Git.BranchesRangeSize {
		limit = setting.Git.BranchesRangeSize
	}

	skip := (page - 1) * limit
	log.Debug("Branches: skip: %d limit: %d", skip, limit)
	branches, branchesCount := loadBranches(ctx, skip, limit)
	if ctx.Written() {
		return
	}
	ctx.Data["Branches"] = branches
	pager := context.NewPagination(int(branchesCount), setting.Git.BranchesRangeSize, page, 5)
	pager.SetDefaultParams(ctx)
	ctx.Data["Page"] = pager

	ctx.HTML(http.StatusOK, tplBranch)
}

// DeleteBranchPost responses for delete merged branch
func DeleteBranchPost(ctx *context.Context) {
	defer redirect(ctx)
	branchName := ctx.Query("name")

	if err := repo_service.DeleteBranch(ctx.User, ctx.Repo.Repository, ctx.Repo.GitRepo, branchName); err != nil {
		switch {
		case git.IsErrBranchNotExist(err):
			log.Debug("DeleteBranch: Can't delete non existing branch '%s'", branchName)
			ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", branchName))
		case errors.Is(err, repo_service.ErrBranchIsDefault):
			log.Debug("DeleteBranch: Can't delete default branch '%s'", branchName)
			ctx.Flash.Error(ctx.Tr("repo.branch.default_deletion_failed", branchName))
		case errors.Is(err, repo_service.ErrBranchIsProtected):
			log.Debug("DeleteBranch: Can't delete protected branch '%s'", branchName)
			ctx.Flash.Error(ctx.Tr("repo.branch.protected_deletion_failed", branchName))
		default:
			log.Error("DeleteBranch: %v", err)
			ctx.Flash.Error(ctx.Tr("repo.branch.deletion_failed", branchName))
		}

		return
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.deletion_success", branchName))
}

// RestoreBranchPost responses for delete merged branch
func RestoreBranchPost(ctx *context.Context) {
	defer redirect(ctx)

	branchID := ctx.QueryInt64("branch_id")
	branchName := ctx.Query("name")

	deletedBranch, err := ctx.Repo.Repository.GetDeletedBranchByID(branchID)
	if err != nil {
		log.Error("GetDeletedBranchByID: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.restore_failed", branchName))
		return
	}

	if err := git.Push(ctx.Repo.Repository.RepoPath(), git.PushOptions{
		Remote: ctx.Repo.Repository.RepoPath(),
		Branch: fmt.Sprintf("%s:%s%s", deletedBranch.Commit, git.BranchPrefix, deletedBranch.Name),
		Env:    models.PushingEnvironment(ctx.User, ctx.Repo.Repository),
	}); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Debug("RestoreBranch: Can't restore branch '%s', since one with same name already exist", deletedBranch.Name)
			ctx.Flash.Error(ctx.Tr("repo.branch.already_exists", deletedBranch.Name))
			return
		}
		log.Error("RestoreBranch: CreateBranch: %v", err)
		ctx.Flash.Error(ctx.Tr("repo.branch.restore_failed", deletedBranch.Name))
		return
	}

	// Don't return error below this
	if err := repo_service.PushUpdate(
		&repo_module.PushUpdateOptions{
			RefFullName:  git.BranchPrefix + deletedBranch.Name,
			OldCommitID:  git.EmptySHA,
			NewCommitID:  deletedBranch.Commit,
			PusherID:     ctx.User.ID,
			PusherName:   ctx.User.Name,
			RepoUserName: ctx.Repo.Owner.Name,
			RepoName:     ctx.Repo.Repository.Name,
		}); err != nil {
		log.Error("RestoreBranch: Update: %v", err)
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.restore_success", deletedBranch.Name))
}

func redirect(ctx *context.Context) {
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/branches",
	})
}

// loadBranches loads branches from the repository limited by page & pageSize.
// NOTE: May write to context on error.
func loadBranches(ctx *context.Context, skip, limit int) ([]*Branch, int) {
	defaultBranch, err := repo_module.GetBranch(ctx.Repo.Repository, ctx.Repo.Repository.DefaultBranch)
	if err != nil {
		log.Error("loadBranches: get default branch: %v", err)
		ctx.ServerError("GetDefaultBranch", err)
		return nil, 0
	}

	rawBranches, totalNumOfBranches, err := repo_module.GetBranches(ctx.Repo.Repository, skip, limit)
	if err != nil {
		log.Error("GetBranches: %v", err)
		ctx.ServerError("GetBranches", err)
		return nil, 0
	}

	protectedBranches, err := ctx.Repo.Repository.GetProtectedBranches()
	if err != nil {
		ctx.ServerError("GetProtectedBranches", err)
		return nil, 0
	}

	repoIDToRepo := map[int64]*models.Repository{}
	repoIDToRepo[ctx.Repo.Repository.ID] = ctx.Repo.Repository

	repoIDToGitRepo := map[int64]*git.Repository{}
	repoIDToGitRepo[ctx.Repo.Repository.ID] = ctx.Repo.GitRepo

	var branches []*Branch
	for i := range rawBranches {
		if rawBranches[i].Name == defaultBranch.Name {
			// Skip default branch
			continue
		}

		var branch = loadOneBranch(ctx, rawBranches[i], protectedBranches, repoIDToRepo, repoIDToGitRepo)
		if branch == nil {
			return nil, 0
		}

		branches = append(branches, branch)
	}

	// Always add the default branch
	log.Debug("loadOneBranch: load default: '%s'", defaultBranch.Name)
	branches = append(branches, loadOneBranch(ctx, defaultBranch, protectedBranches, repoIDToRepo, repoIDToGitRepo))

	if ctx.Repo.CanWrite(models.UnitTypeCode) {
		deletedBranches, err := getDeletedBranches(ctx)
		if err != nil {
			ctx.ServerError("getDeletedBranches", err)
			return nil, 0
		}
		branches = append(branches, deletedBranches...)
	}

	return branches, totalNumOfBranches - 1
}

func loadOneBranch(ctx *context.Context, rawBranch *git.Branch, protectedBranches []*models.ProtectedBranch,
	repoIDToRepo map[int64]*models.Repository,
	repoIDToGitRepo map[int64]*git.Repository) *Branch {
	log.Trace("loadOneBranch: '%s'", rawBranch.Name)

	commit, err := rawBranch.GetCommit()
	if err != nil {
		ctx.ServerError("GetCommit", err)
		return nil
	}

	branchName := rawBranch.Name
	var isProtected bool
	for _, b := range protectedBranches {
		if b.BranchName == branchName {
			isProtected = true
			break
		}
	}

	divergence, divergenceError := repofiles.CountDivergingCommits(ctx.Repo.Repository, git.BranchPrefix+branchName)
	if divergenceError != nil {
		ctx.ServerError("CountDivergingCommits", divergenceError)
		return nil
	}

	pr, err := models.GetLatestPullRequestByHeadInfo(ctx.Repo.Repository.ID, branchName)
	if err != nil {
		ctx.ServerError("GetLatestPullRequestByHeadInfo", err)
		return nil
	}
	headCommit := commit.ID.String()

	mergeMovedOn := false
	if pr != nil {
		pr.HeadRepo = ctx.Repo.Repository
		if err := pr.LoadIssue(); err != nil {
			ctx.ServerError("pr.LoadIssue", err)
			return nil
		}
		if repo, ok := repoIDToRepo[pr.BaseRepoID]; ok {
			pr.BaseRepo = repo
		} else if err := pr.LoadBaseRepo(); err != nil {
			ctx.ServerError("pr.LoadBaseRepo", err)
			return nil
		} else {
			repoIDToRepo[pr.BaseRepoID] = pr.BaseRepo
		}
		pr.Issue.Repo = pr.BaseRepo

		if pr.HasMerged {
			baseGitRepo, ok := repoIDToGitRepo[pr.BaseRepoID]
			if !ok {
				baseGitRepo, err = git.OpenRepository(pr.BaseRepo.RepoPath())
				if err != nil {
					ctx.ServerError("OpenRepository", err)
					return nil
				}
				defer baseGitRepo.Close()
				repoIDToGitRepo[pr.BaseRepoID] = baseGitRepo
			}
			pullCommit, err := baseGitRepo.GetRefCommitID(pr.GetGitRefName())
			if err != nil && !git.IsErrNotExist(err) {
				ctx.ServerError("GetBranchCommitID", err)
				return nil
			}
			if err == nil && headCommit != pullCommit {
				// the head has moved on from the merge - we shouldn't delete
				mergeMovedOn = true
			}
		}
	}

	isIncluded := divergence.Ahead == 0 && ctx.Repo.Repository.DefaultBranch != branchName
	return &Branch{
		Name:              branchName,
		Commit:            commit,
		IsProtected:       isProtected,
		IsIncluded:        isIncluded,
		CommitsAhead:      divergence.Ahead,
		CommitsBehind:     divergence.Behind,
		LatestPullRequest: pr,
		MergeMovedOn:      mergeMovedOn,
	}
}

func getDeletedBranches(ctx *context.Context) ([]*Branch, error) {
	branches := []*Branch{}

	deletedBranches, err := ctx.Repo.Repository.GetDeletedBranches()
	if err != nil {
		return branches, err
	}

	for i := range deletedBranches {
		deletedBranches[i].LoadUser()
		branches = append(branches, &Branch{
			Name:          deletedBranches[i].Name,
			IsDeleted:     true,
			DeletedBranch: deletedBranches[i],
		})
	}

	return branches, nil
}

// CreateBranch creates new branch in repository
func CreateBranch(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.NewBranchForm)
	if !ctx.Repo.CanCreateBranch() {
		ctx.NotFound("CreateBranch", nil)
		return
	}

	if ctx.HasError() {
		ctx.Flash.Error(ctx.GetErrMsg())
		ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
		return
	}

	var err error

	if form.CreateTag {
		if ctx.Repo.IsViewTag {
			err = release_service.CreateNewTag(ctx.User, ctx.Repo.Repository, ctx.Repo.CommitID, form.NewBranchName, "")
		} else {
			err = release_service.CreateNewTag(ctx.User, ctx.Repo.Repository, ctx.Repo.BranchName, form.NewBranchName, "")
		}
	} else if ctx.Repo.IsViewBranch {
		err = repo_module.CreateNewBranch(ctx.User, ctx.Repo.Repository, ctx.Repo.BranchName, form.NewBranchName)
	} else if ctx.Repo.IsViewTag {
		err = repo_module.CreateNewBranchFromCommit(ctx.User, ctx.Repo.Repository, ctx.Repo.CommitID, form.NewBranchName)
	} else {
		err = repo_module.CreateNewBranchFromCommit(ctx.User, ctx.Repo.Repository, ctx.Repo.BranchName, form.NewBranchName)
	}
	if err != nil {
		if models.IsErrTagAlreadyExists(err) {
			e := err.(models.ErrTagAlreadyExists)
			ctx.Flash.Error(ctx.Tr("repo.branch.tag_collision", e.TagName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}
		if models.IsErrBranchAlreadyExists(err) || git.IsErrPushOutOfDate(err) {
			ctx.Flash.Error(ctx.Tr("repo.branch.branch_already_exists", form.NewBranchName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}
		if models.IsErrBranchNameConflict(err) {
			e := err.(models.ErrBranchNameConflict)
			ctx.Flash.Error(ctx.Tr("repo.branch.branch_name_conflict", form.NewBranchName, e.BranchName))
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}
		if git.IsErrPushRejected(err) {
			e := err.(*git.ErrPushRejected)
			if len(e.Message) == 0 {
				ctx.Flash.Error(ctx.Tr("repo.editor.push_rejected_no_message"))
			} else {
				flashError, err := ctx.HTMLString(string(tplAlertDetails), map[string]interface{}{
					"Message": ctx.Tr("repo.editor.push_rejected"),
					"Summary": ctx.Tr("repo.editor.push_rejected_summary"),
					"Details": utils.SanitizeFlashErrorString(e.Message),
				})
				if err != nil {
					ctx.ServerError("UpdatePullRequest.HTMLString", err)
					return
				}
				ctx.Flash.Error(flashError)
			}
			ctx.Redirect(ctx.Repo.RepoLink + "/src/" + ctx.Repo.BranchNameSubURL())
			return
		}

		ctx.ServerError("CreateNewBranch", err)
		return
	}

	if form.CreateTag {
		ctx.Flash.Success(ctx.Tr("repo.tag.create_success", form.NewBranchName))
		ctx.Redirect(ctx.Repo.RepoLink + "/src/tag/" + util.PathEscapeSegments(form.NewBranchName))
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.branch.create_success", form.NewBranchName))
	ctx.Redirect(ctx.Repo.RepoLink + "/src/branch/" + util.PathEscapeSegments(form.NewBranchName))
}
