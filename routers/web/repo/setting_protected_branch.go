// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
	pull_service "code.gitea.io/gitea/services/pull"
)

// ProtectedBranch render the page to protect the repository
func ProtectedBranch(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsBranches"] = true

	protectedBranches, err := ctx.Repo.Repository.GetProtectedBranches()
	if err != nil {
		ctx.ServerError("GetProtectedBranches", err)
		return
	}
	ctx.Data["ProtectedBranches"] = protectedBranches

	branches := ctx.Data["Branches"].([]string)
	leftBranches := make([]string, 0, len(branches)-len(protectedBranches))
	for _, b := range branches {
		var protected bool
		for _, pb := range protectedBranches {
			if b == pb.BranchName {
				protected = true
				break
			}
		}
		if !protected {
			leftBranches = append(leftBranches, b)
		}
	}

	ctx.Data["LeftBranches"] = leftBranches

	ctx.HTML(http.StatusOK, tplBranches)
}

// ProtectedBranchPost response for protect for a branch of a repository
func ProtectedBranchPost(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsBranches"] = true

	repo := ctx.Repo.Repository

	switch ctx.Query("action") {
	case "default_branch":
		if ctx.HasError() {
			ctx.HTML(http.StatusOK, tplBranches)
			return
		}

		branch := ctx.Query("branch")
		if !ctx.Repo.GitRepo.IsBranchExist(branch) {
			ctx.Status(404)
			return
		} else if repo.DefaultBranch != branch {
			repo.DefaultBranch = branch
			if err := ctx.Repo.GitRepo.SetDefaultBranch(branch); err != nil {
				if !git.IsErrUnsupportedVersion(err) {
					ctx.ServerError("SetDefaultBranch", err)
					return
				}
			}
			if err := repo.UpdateDefaultBranch(); err != nil {
				ctx.ServerError("SetDefaultBranch", err)
				return
			}
		}

		log.Trace("Repository basic settings updated: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.Path)
	default:
		ctx.NotFound("", nil)
	}
}

// SettingsProtectedBranch renders the protected branch setting page
func SettingsProtectedBranch(c *context.Context) {
	branch := c.Params("*")
	if !c.Repo.GitRepo.IsBranchExist(branch) {
		c.NotFound("IsBranchExist", nil)
		return
	}

	c.Data["Title"] = c.Tr("repo.settings.protected_branch") + " - " + branch
	c.Data["PageIsSettingsBranches"] = true

	protectBranch, err := models.GetProtectedBranchBy(c.Repo.Repository.ID, branch)
	if err != nil {
		if !git.IsErrBranchNotExist(err) {
			c.ServerError("GetProtectBranchOfRepoByName", err)
			return
		}
	}

	if protectBranch == nil {
		// No options found, create defaults.
		protectBranch = &models.ProtectedBranch{
			BranchName: branch,
		}
	}

	users, err := c.Repo.Repository.GetReaders()
	if err != nil {
		c.ServerError("Repo.Repository.GetReaders", err)
		return
	}
	c.Data["Users"] = users
	c.Data["whitelist_users"] = strings.Join(base.Int64sToStrings(protectBranch.WhitelistUserIDs), ",")
	c.Data["merge_whitelist_users"] = strings.Join(base.Int64sToStrings(protectBranch.MergeWhitelistUserIDs), ",")
	c.Data["approvals_whitelist_users"] = strings.Join(base.Int64sToStrings(protectBranch.ApprovalsWhitelistUserIDs), ",")
	contexts, _ := models.FindRepoRecentCommitStatusContexts(c.Repo.Repository.ID, 7*24*time.Hour) // Find last week status check contexts
	for _, ctx := range protectBranch.StatusCheckContexts {
		var found bool
		for i := range contexts {
			if contexts[i] == ctx {
				found = true
				break
			}
		}
		if !found {
			contexts = append(contexts, ctx)
		}
	}

	c.Data["branch_status_check_contexts"] = contexts
	c.Data["is_context_required"] = func(context string) bool {
		for _, c := range protectBranch.StatusCheckContexts {
			if c == context {
				return true
			}
		}
		return false
	}

	if c.Repo.Owner.IsOrganization() {
		teams, err := c.Repo.Owner.TeamsWithAccessToRepo(c.Repo.Repository.ID, models.AccessModeRead)
		if err != nil {
			c.ServerError("Repo.Owner.TeamsWithAccessToRepo", err)
			return
		}
		c.Data["Teams"] = teams
		c.Data["whitelist_teams"] = strings.Join(base.Int64sToStrings(protectBranch.WhitelistTeamIDs), ",")
		c.Data["merge_whitelist_teams"] = strings.Join(base.Int64sToStrings(protectBranch.MergeWhitelistTeamIDs), ",")
		c.Data["approvals_whitelist_teams"] = strings.Join(base.Int64sToStrings(protectBranch.ApprovalsWhitelistTeamIDs), ",")
	}

	c.Data["Branch"] = protectBranch
	c.HTML(http.StatusOK, tplProtectedBranch)
}

// SettingsProtectedBranchPost updates the protected branch settings
func SettingsProtectedBranchPost(ctx *context.Context) {
	f := web.GetForm(ctx).(*forms.ProtectBranchForm)
	branch := ctx.Params("*")
	if !ctx.Repo.GitRepo.IsBranchExist(branch) {
		ctx.NotFound("IsBranchExist", nil)
		return
	}

	protectBranch, err := models.GetProtectedBranchBy(ctx.Repo.Repository.ID, branch)
	if err != nil {
		if !git.IsErrBranchNotExist(err) {
			ctx.ServerError("GetProtectBranchOfRepoByName", err)
			return
		}
	}

	if f.Protected {
		if protectBranch == nil {
			// No options found, create defaults.
			protectBranch = &models.ProtectedBranch{
				RepoID:     ctx.Repo.Repository.ID,
				BranchName: branch,
			}
		}
		if f.RequiredApprovals < 0 {
			ctx.Flash.Error(ctx.Tr("repo.settings.protected_branch_required_approvals_min"))
			ctx.Redirect(fmt.Sprintf("%s/settings/branches/%s", ctx.Repo.RepoLink, branch))
		}

		var whitelistUsers, whitelistTeams, mergeWhitelistUsers, mergeWhitelistTeams, approvalsWhitelistUsers, approvalsWhitelistTeams []int64
		switch f.EnablePush {
		case "all":
			protectBranch.CanPush = true
			protectBranch.EnableWhitelist = false
			protectBranch.WhitelistDeployKeys = false
		case "whitelist":
			protectBranch.CanPush = true
			protectBranch.EnableWhitelist = true
			protectBranch.WhitelistDeployKeys = f.WhitelistDeployKeys
			if strings.TrimSpace(f.WhitelistUsers) != "" {
				whitelistUsers, _ = base.StringsToInt64s(strings.Split(f.WhitelistUsers, ","))
			}
			if strings.TrimSpace(f.WhitelistTeams) != "" {
				whitelistTeams, _ = base.StringsToInt64s(strings.Split(f.WhitelistTeams, ","))
			}
		default:
			protectBranch.CanPush = false
			protectBranch.EnableWhitelist = false
			protectBranch.WhitelistDeployKeys = false
		}

		protectBranch.EnableMergeWhitelist = f.EnableMergeWhitelist
		if f.EnableMergeWhitelist {
			if strings.TrimSpace(f.MergeWhitelistUsers) != "" {
				mergeWhitelistUsers, _ = base.StringsToInt64s(strings.Split(f.MergeWhitelistUsers, ","))
			}
			if strings.TrimSpace(f.MergeWhitelistTeams) != "" {
				mergeWhitelistTeams, _ = base.StringsToInt64s(strings.Split(f.MergeWhitelistTeams, ","))
			}
		}

		protectBranch.EnableStatusCheck = f.EnableStatusCheck
		if f.EnableStatusCheck {
			protectBranch.StatusCheckContexts = f.StatusCheckContexts
		} else {
			protectBranch.StatusCheckContexts = nil
		}

		protectBranch.RequiredApprovals = f.RequiredApprovals
		protectBranch.EnableApprovalsWhitelist = f.EnableApprovalsWhitelist
		if f.EnableApprovalsWhitelist {
			if strings.TrimSpace(f.ApprovalsWhitelistUsers) != "" {
				approvalsWhitelistUsers, _ = base.StringsToInt64s(strings.Split(f.ApprovalsWhitelistUsers, ","))
			}
			if strings.TrimSpace(f.ApprovalsWhitelistTeams) != "" {
				approvalsWhitelistTeams, _ = base.StringsToInt64s(strings.Split(f.ApprovalsWhitelistTeams, ","))
			}
		}
		protectBranch.BlockOnRejectedReviews = f.BlockOnRejectedReviews
		protectBranch.BlockOnOfficialReviewRequests = f.BlockOnOfficialReviewRequests
		protectBranch.DismissStaleApprovals = f.DismissStaleApprovals
		protectBranch.RequireSignedCommits = f.RequireSignedCommits
		protectBranch.ProtectedFilePatterns = f.ProtectedFilePatterns
		protectBranch.UnprotectedFilePatterns = f.UnprotectedFilePatterns
		protectBranch.BlockOnOutdatedBranch = f.BlockOnOutdatedBranch

		err = models.UpdateProtectBranch(ctx.Repo.Repository, protectBranch, models.WhitelistOptions{
			UserIDs:          whitelistUsers,
			TeamIDs:          whitelistTeams,
			MergeUserIDs:     mergeWhitelistUsers,
			MergeTeamIDs:     mergeWhitelistTeams,
			ApprovalsUserIDs: approvalsWhitelistUsers,
			ApprovalsTeamIDs: approvalsWhitelistTeams,
		})
		if err != nil {
			ctx.ServerError("UpdateProtectBranch", err)
			return
		}
		if err = pull_service.CheckPrsForBaseBranch(ctx.Repo.Repository, protectBranch.BranchName); err != nil {
			ctx.ServerError("CheckPrsForBaseBranch", err)
			return
		}
		ctx.Flash.Success(ctx.Tr("repo.settings.update_protect_branch_success", branch))
		ctx.Redirect(fmt.Sprintf("%s/settings/branches/%s", ctx.Repo.RepoLink, branch))
	} else {
		if protectBranch != nil {
			if err := ctx.Repo.Repository.DeleteProtectedBranch(protectBranch.ID); err != nil {
				ctx.ServerError("DeleteProtectedBranch", err)
				return
			}
		}
		ctx.Flash.Success(ctx.Tr("repo.settings.remove_protected_branch_success", branch))
		ctx.Redirect(fmt.Sprintf("%s/settings/branches", ctx.Repo.RepoLink))
	}
}
