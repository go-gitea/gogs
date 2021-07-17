// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"fmt"
	"net/http"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	repo_module "code.gitea.io/gitea/modules/repository"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/api/v1/utils"
	pull_service "code.gitea.io/gitea/services/pull"
	repo_service "code.gitea.io/gitea/services/repository"
)

// GetBranch get a branch of a repository
func GetBranch(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/branches/{branch} repository repoGetBranch
	// ---
	// summary: Retrieve a specific branch from a repository, including its effective branch protection
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: branch
	//   in: path
	//   description: branch to get
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/Branch"
	//   "404":
	//     "$ref": "#/responses/notFound"

	branchName := ctx.Params("*")

	branch, err := repo_module.GetBranch(ctx.Repo.Repository, branchName)
	if err != nil {
		if git.IsErrBranchNotExist(err) {
			ctx.NotFound(err)
		} else {
			ctx.Error(http.StatusInternalServerError, "GetBranch", err)
		}
		return
	}

	c, err := branch.GetCommit()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommit", err)
		return
	}

	branchProtection, err := ctx.Repo.Repository.GetBranchProtection(branchName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetBranchProtection", err)
		return
	}

	br, err := convert.ToBranch(ctx.Repo.Repository, branch, c, branchProtection, ctx.User, ctx.Repo.IsAdmin())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convert.ToBranch", err)
		return
	}

	ctx.JSON(http.StatusOK, br)
}

// DeleteBranch get a branch of a repository
func DeleteBranch(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/branches/{branch} repository repoDeleteBranch
	// ---
	// summary: Delete a specific branch from a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: branch
	//   in: path
	//   description: branch to delete
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "403":
	//     "$ref": "#/responses/error"
	//   "404":
	//     "$ref": "#/responses/notFound"

	branchName := ctx.Params("*")

	if err := repo_service.DeleteBranch(ctx.User, ctx.Repo.Repository, ctx.Repo.GitRepo, branchName); err != nil {
		switch {
		case git.IsErrBranchNotExist(err):
			ctx.NotFound(err)
		case errors.Is(err, repo_service.ErrBranchIsDefault):
			ctx.Error(http.StatusForbidden, "DefaultBranch", fmt.Errorf("can not delete default branch"))
		case errors.Is(err, repo_service.ErrBranchIsProtected):
			ctx.Error(http.StatusForbidden, "IsProtectedBranch", fmt.Errorf("branch protected"))
		default:
			ctx.Error(http.StatusInternalServerError, "DeleteBranch", err)
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// CreateBranch creates a branch for a user's repository
func CreateBranch(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/branches repository repoCreateBranch
	// ---
	// summary: Create a branch
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateBranchRepoOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/Branch"
	//   "404":
	//     description: The old branch does not exist.
	//   "409":
	//     description: The branch with the same name already exists.

	opt := web.GetForm(ctx).(*api.CreateBranchRepoOption)
	if ctx.Repo.Repository.IsEmpty {
		ctx.Error(http.StatusNotFound, "", "Git Repository is empty.")
		return
	}

	if len(opt.OldBranchName) == 0 {
		opt.OldBranchName = ctx.Repo.Repository.DefaultBranch
	}

	err := repo_module.CreateNewBranch(ctx.User, ctx.Repo.Repository, opt.OldBranchName, opt.BranchName)

	if err != nil {
		if models.IsErrBranchDoesNotExist(err) {
			ctx.Error(http.StatusNotFound, "", "The old branch does not exist")
		}
		if models.IsErrTagAlreadyExists(err) {
			ctx.Error(http.StatusConflict, "", "The branch with the same tag already exists.")

		} else if models.IsErrBranchAlreadyExists(err) || git.IsErrPushOutOfDate(err) {
			ctx.Error(http.StatusConflict, "", "The branch already exists.")

		} else if models.IsErrBranchNameConflict(err) {
			ctx.Error(http.StatusConflict, "", "The branch with the same name already exists.")

		} else {
			ctx.Error(http.StatusInternalServerError, "CreateRepoBranch", err)

		}
		return
	}

	branch, err := repo_module.GetBranch(ctx.Repo.Repository, opt.BranchName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetBranch", err)
		return
	}

	commit, err := branch.GetCommit()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetCommit", err)
		return
	}

	branchProtection, err := ctx.Repo.Repository.GetBranchProtection(branch.Name)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetBranchProtection", err)
		return
	}

	br, err := convert.ToBranch(ctx.Repo.Repository, branch, commit, branchProtection, ctx.User, ctx.Repo.IsAdmin())
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "convert.ToBranch", err)
		return
	}

	ctx.JSON(http.StatusCreated, br)
}

// ListBranches list all the branches of a repository
func ListBranches(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/branches repository repoListBranches
	// ---
	// summary: List a repository's branches
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: page
	//   in: query
	//   description: page number of results to return (1-based)
	//   type: integer
	// - name: limit
	//   in: query
	//   description: page size of results
	//   type: integer
	// responses:
	//   "200":
	//     "$ref": "#/responses/BranchList"

	listOptions := utils.GetListOptions(ctx)
	skip, _ := listOptions.GetStartEnd()
	branches, totalNumOfBranches, err := repo_module.GetBranches(ctx.Repo.Repository, skip, listOptions.PageSize)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetBranches", err)
		return
	}

	apiBranches := make([]*api.Branch, len(branches))
	for i := range branches {
		c, err := branches[i].GetCommit()
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetCommit", err)
			return
		}
		branchProtection, err := ctx.Repo.Repository.GetBranchProtection(branches[i].Name)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetBranchProtection", err)
			return
		}
		apiBranches[i], err = convert.ToBranch(ctx.Repo.Repository, branches[i], c, branchProtection, ctx.User, ctx.Repo.IsAdmin())
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "convert.ToBranch", err)
			return
		}
	}

	ctx.SetLinkHeader(int(totalNumOfBranches), listOptions.PageSize)
	ctx.Header().Set("X-Total-Count", fmt.Sprintf("%d", totalNumOfBranches))
	ctx.Header().Set("Access-Control-Expose-Headers", "X-Total-Count, Link")
	ctx.JSON(http.StatusOK, &apiBranches)
}

// GetBranchProtection gets a branch protection
func GetBranchProtection(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/branch_protections/{name} repository repoGetBranchProtection
	// ---
	// summary: Get a specific branch protection for the repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: name
	//   in: path
	//   description: name of protected branch
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/BranchProtection"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo.Repository
	bpName := ctx.Params(":name")
	bp, err := models.GetProtectedBranchBy(repo.ID, bpName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranchByID", err)
		return
	}
	if bp == nil || bp.RepoID != repo.ID {
		ctx.NotFound()
		return
	}

	ctx.JSON(http.StatusOK, convert.ToBranchProtection(bp))
}

// ListBranchProtections list branch protections for a repo
func ListBranchProtections(ctx *context.APIContext) {
	// swagger:operation GET /repos/{owner}/{repo}/branch_protections repository repoListBranchProtection
	// ---
	// summary: List branch protections for a repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// responses:
	//   "200":
	//     "$ref": "#/responses/BranchProtectionList"

	repo := ctx.Repo.Repository
	bps, err := repo.GetProtectedBranches()
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranches", err)
		return
	}
	apiBps := make([]*api.BranchProtection, len(bps))
	for i := range bps {
		apiBps[i] = convert.ToBranchProtection(bps[i])
	}

	ctx.JSON(http.StatusOK, apiBps)
}

// CreateBranchProtection creates a branch protection for a repo
func CreateBranchProtection(ctx *context.APIContext) {
	// swagger:operation POST /repos/{owner}/{repo}/branch_protections repository repoCreateBranchProtection
	// ---
	// summary: Create a branch protections for a repository
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/CreateBranchProtectionOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/BranchProtection"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"

	form := web.GetForm(ctx).(*api.CreateBranchProtectionOption)
	repo := ctx.Repo.Repository

	// Currently protection must match an actual branch
	if !git.IsBranchExist(ctx.Repo.Repository.RepoPath(), form.BranchName) {
		ctx.NotFound()
		return
	}

	protectBranch, err := models.GetProtectedBranchBy(repo.ID, form.BranchName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectBranchOfRepoByName", err)
		return
	} else if protectBranch != nil {
		ctx.Error(http.StatusForbidden, "Create branch protection", "Branch protection already exist")
		return
	}

	var requiredApprovals int64
	if form.RequiredApprovals > 0 {
		requiredApprovals = form.RequiredApprovals
	}

	whitelistUsers, err := models.GetUserIDsByNames(form.PushWhitelistUsernames, false)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
		return
	}
	mergeWhitelistUsers, err := models.GetUserIDsByNames(form.MergeWhitelistUsernames, false)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
		return
	}
	approvalsWhitelistUsers, err := models.GetUserIDsByNames(form.ApprovalsWhitelistUsernames, false)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
			return
		}
		ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
		return
	}
	var whitelistTeams, mergeWhitelistTeams, approvalsWhitelistTeams []int64
	if repo.Owner.IsOrganization() {
		whitelistTeams, err = models.GetTeamIDsByNames(repo.OwnerID, form.PushWhitelistTeams, false)
		if err != nil {
			if models.IsErrTeamNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
			return
		}
		mergeWhitelistTeams, err = models.GetTeamIDsByNames(repo.OwnerID, form.MergeWhitelistTeams, false)
		if err != nil {
			if models.IsErrTeamNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
			return
		}
		approvalsWhitelistTeams, err = models.GetTeamIDsByNames(repo.OwnerID, form.ApprovalsWhitelistTeams, false)
		if err != nil {
			if models.IsErrTeamNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
			return
		}
	}

	protectBranch = &models.ProtectedBranch{
		RepoID:                        ctx.Repo.Repository.ID,
		BranchName:                    form.BranchName,
		CanPush:                       form.EnablePush,
		EnableWhitelist:               form.EnablePush && form.EnablePushWhitelist,
		EnableMergeWhitelist:          form.EnableMergeWhitelist,
		WhitelistDeployKeys:           form.EnablePush && form.EnablePushWhitelist && form.PushWhitelistDeployKeys,
		EnableStatusCheck:             form.EnableStatusCheck,
		StatusCheckContexts:           form.StatusCheckContexts,
		EnableApprovalsWhitelist:      form.EnableApprovalsWhitelist,
		RequiredApprovals:             requiredApprovals,
		BlockOnRejectedReviews:        form.BlockOnRejectedReviews,
		BlockOnOfficialReviewRequests: form.BlockOnOfficialReviewRequests,
		DismissStaleApprovals:         form.DismissStaleApprovals,
		RequireSignedCommits:          form.RequireSignedCommits,
		ProtectedFilePatterns:         form.ProtectedFilePatterns,
		UnprotectedFilePatterns:       form.UnprotectedFilePatterns,
		BlockOnOutdatedBranch:         form.BlockOnOutdatedBranch,
	}

	err = models.UpdateProtectBranch(ctx.Repo.Repository, protectBranch, models.WhitelistOptions{
		UserIDs:          whitelistUsers,
		TeamIDs:          whitelistTeams,
		MergeUserIDs:     mergeWhitelistUsers,
		MergeTeamIDs:     mergeWhitelistTeams,
		ApprovalsUserIDs: approvalsWhitelistUsers,
		ApprovalsTeamIDs: approvalsWhitelistTeams,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateProtectBranch", err)
		return
	}

	if err = pull_service.CheckPrsForBaseBranch(ctx.Repo.Repository, protectBranch.BranchName); err != nil {
		ctx.Error(http.StatusInternalServerError, "CheckPrsForBaseBranch", err)
		return
	}

	// Reload from db to get all whitelists
	bp, err := models.GetProtectedBranchBy(ctx.Repo.Repository.ID, form.BranchName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranchByID", err)
		return
	}
	if bp == nil || bp.RepoID != ctx.Repo.Repository.ID {
		ctx.Error(http.StatusInternalServerError, "New branch protection not found", err)
		return
	}

	ctx.JSON(http.StatusCreated, convert.ToBranchProtection(bp))

}

// EditBranchProtection edits a branch protection for a repo
func EditBranchProtection(ctx *context.APIContext) {
	// swagger:operation PATCH /repos/{owner}/{repo}/branch_protections/{name} repository repoEditBranchProtection
	// ---
	// summary: Edit a branch protections for a repository. Only fields that are set will be changed
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: name
	//   in: path
	//   description: name of protected branch
	//   type: string
	//   required: true
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/EditBranchProtectionOption"
	// responses:
	//   "200":
	//     "$ref": "#/responses/BranchProtection"
	//   "404":
	//     "$ref": "#/responses/notFound"
	//   "422":
	//     "$ref": "#/responses/validationError"
	form := web.GetForm(ctx).(*api.EditBranchProtectionOption)
	repo := ctx.Repo.Repository
	bpName := ctx.Params(":name")
	protectBranch, err := models.GetProtectedBranchBy(repo.ID, bpName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranchByID", err)
		return
	}
	if protectBranch == nil || protectBranch.RepoID != repo.ID {
		ctx.NotFound()
		return
	}

	if form.EnablePush != nil {
		if !*form.EnablePush {
			protectBranch.CanPush = false
			protectBranch.EnableWhitelist = false
			protectBranch.WhitelistDeployKeys = false
		} else {
			protectBranch.CanPush = true
			if form.EnablePushWhitelist != nil {
				if !*form.EnablePushWhitelist {
					protectBranch.EnableWhitelist = false
					protectBranch.WhitelistDeployKeys = false
				} else {
					protectBranch.EnableWhitelist = true
					if form.PushWhitelistDeployKeys != nil {
						protectBranch.WhitelistDeployKeys = *form.PushWhitelistDeployKeys
					}
				}
			}
		}
	}

	if form.EnableMergeWhitelist != nil {
		protectBranch.EnableMergeWhitelist = *form.EnableMergeWhitelist
	}

	if form.EnableStatusCheck != nil {
		protectBranch.EnableStatusCheck = *form.EnableStatusCheck
	}
	if protectBranch.EnableStatusCheck {
		protectBranch.StatusCheckContexts = form.StatusCheckContexts
	}

	if form.RequiredApprovals != nil && *form.RequiredApprovals >= 0 {
		protectBranch.RequiredApprovals = *form.RequiredApprovals
	}

	if form.EnableApprovalsWhitelist != nil {
		protectBranch.EnableApprovalsWhitelist = *form.EnableApprovalsWhitelist
	}

	if form.BlockOnRejectedReviews != nil {
		protectBranch.BlockOnRejectedReviews = *form.BlockOnRejectedReviews
	}

	if form.BlockOnOfficialReviewRequests != nil {
		protectBranch.BlockOnOfficialReviewRequests = *form.BlockOnOfficialReviewRequests
	}

	if form.DismissStaleApprovals != nil {
		protectBranch.DismissStaleApprovals = *form.DismissStaleApprovals
	}

	if form.RequireSignedCommits != nil {
		protectBranch.RequireSignedCommits = *form.RequireSignedCommits
	}

	if form.ProtectedFilePatterns != nil {
		protectBranch.ProtectedFilePatterns = *form.ProtectedFilePatterns
	}

	if form.UnprotectedFilePatterns != nil {
		protectBranch.UnprotectedFilePatterns = *form.UnprotectedFilePatterns
	}

	if form.BlockOnOutdatedBranch != nil {
		protectBranch.BlockOnOutdatedBranch = *form.BlockOnOutdatedBranch
	}

	var whitelistUsers []int64
	if form.PushWhitelistUsernames != nil {
		whitelistUsers, err = models.GetUserIDsByNames(form.PushWhitelistUsernames, false)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
			return
		}
	} else {
		whitelistUsers = protectBranch.WhitelistUserIDs
	}
	var mergeWhitelistUsers []int64
	if form.MergeWhitelistUsernames != nil {
		mergeWhitelistUsers, err = models.GetUserIDsByNames(form.MergeWhitelistUsernames, false)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
			return
		}
	} else {
		mergeWhitelistUsers = protectBranch.MergeWhitelistUserIDs
	}
	var approvalsWhitelistUsers []int64
	if form.ApprovalsWhitelistUsernames != nil {
		approvalsWhitelistUsers, err = models.GetUserIDsByNames(form.ApprovalsWhitelistUsernames, false)
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.Error(http.StatusUnprocessableEntity, "User does not exist", err)
				return
			}
			ctx.Error(http.StatusInternalServerError, "GetUserIDsByNames", err)
			return
		}
	} else {
		approvalsWhitelistUsers = protectBranch.ApprovalsWhitelistUserIDs
	}

	var whitelistTeams, mergeWhitelistTeams, approvalsWhitelistTeams []int64
	if repo.Owner.IsOrganization() {
		if form.PushWhitelistTeams != nil {
			whitelistTeams, err = models.GetTeamIDsByNames(repo.OwnerID, form.PushWhitelistTeams, false)
			if err != nil {
				if models.IsErrTeamNotExist(err) {
					ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
					return
				}
				ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
				return
			}
		} else {
			whitelistTeams = protectBranch.WhitelistTeamIDs
		}
		if form.MergeWhitelistTeams != nil {
			mergeWhitelistTeams, err = models.GetTeamIDsByNames(repo.OwnerID, form.MergeWhitelistTeams, false)
			if err != nil {
				if models.IsErrTeamNotExist(err) {
					ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
					return
				}
				ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
				return
			}
		} else {
			mergeWhitelistTeams = protectBranch.MergeWhitelistTeamIDs
		}
		if form.ApprovalsWhitelistTeams != nil {
			approvalsWhitelistTeams, err = models.GetTeamIDsByNames(repo.OwnerID, form.ApprovalsWhitelistTeams, false)
			if err != nil {
				if models.IsErrTeamNotExist(err) {
					ctx.Error(http.StatusUnprocessableEntity, "Team does not exist", err)
					return
				}
				ctx.Error(http.StatusInternalServerError, "GetTeamIDsByNames", err)
				return
			}
		} else {
			approvalsWhitelistTeams = protectBranch.ApprovalsWhitelistTeamIDs
		}
	}

	err = models.UpdateProtectBranch(ctx.Repo.Repository, protectBranch, models.WhitelistOptions{
		UserIDs:          whitelistUsers,
		TeamIDs:          whitelistTeams,
		MergeUserIDs:     mergeWhitelistUsers,
		MergeTeamIDs:     mergeWhitelistTeams,
		ApprovalsUserIDs: approvalsWhitelistUsers,
		ApprovalsTeamIDs: approvalsWhitelistTeams,
	})
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "UpdateProtectBranch", err)
		return
	}

	if err = pull_service.CheckPrsForBaseBranch(ctx.Repo.Repository, protectBranch.BranchName); err != nil {
		ctx.Error(http.StatusInternalServerError, "CheckPrsForBaseBranch", err)
		return
	}

	// Reload from db to ensure get all whitelists
	bp, err := models.GetProtectedBranchBy(repo.ID, bpName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranchBy", err)
		return
	}
	if bp == nil || bp.RepoID != ctx.Repo.Repository.ID {
		ctx.Error(http.StatusInternalServerError, "New branch protection not found", err)
		return
	}

	ctx.JSON(http.StatusOK, convert.ToBranchProtection(bp))
}

// DeleteBranchProtection deletes a branch protection for a repo
func DeleteBranchProtection(ctx *context.APIContext) {
	// swagger:operation DELETE /repos/{owner}/{repo}/branch_protections/{name} repository repoDeleteBranchProtection
	// ---
	// summary: Delete a specific branch protection for the repository
	// produces:
	// - application/json
	// parameters:
	// - name: owner
	//   in: path
	//   description: owner of the repo
	//   type: string
	//   required: true
	// - name: repo
	//   in: path
	//   description: name of the repo
	//   type: string
	//   required: true
	// - name: name
	//   in: path
	//   description: name of protected branch
	//   type: string
	//   required: true
	// responses:
	//   "204":
	//     "$ref": "#/responses/empty"
	//   "404":
	//     "$ref": "#/responses/notFound"

	repo := ctx.Repo.Repository
	bpName := ctx.Params(":name")
	bp, err := models.GetProtectedBranchBy(repo.ID, bpName)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, "GetProtectedBranchByID", err)
		return
	}
	if bp == nil || bp.RepoID != repo.ID {
		ctx.NotFound()
		return
	}

	if err := ctx.Repo.Repository.DeleteProtectedBranch(bp.ID); err != nil {
		ctx.Error(http.StatusInternalServerError, "DeleteProtectedBranch", err)
		return
	}

	ctx.Status(http.StatusNoContent)
}
