// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"regexp"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/validation"
	"code.gitea.io/gitea/routers/utils"
	"code.gitea.io/gitea/services/mailer"
	mirror_service "code.gitea.io/gitea/services/mirror"
	repo_service "code.gitea.io/gitea/services/repository"

	"github.com/unknwon/com"
	"mvdan.cc/xurls/v2"
)

const (
	tplSettingsOptions base.TplName = "repo/settings/options"
	tplCollaboration   base.TplName = "repo/settings/collaboration"
	tplBranches        base.TplName = "repo/settings/branches"
	tplGithooks        base.TplName = "repo/settings/githooks"
	tplGithookEdit     base.TplName = "repo/settings/githook_edit"
	tplDeployKeys      base.TplName = "repo/settings/deploy_keys"
	tplProtectedBranch base.TplName = "repo/settings/protected_branch"
)

var validFormAddress *regexp.Regexp

// Settings show a repository's settings page
func Settings(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsOptions"] = true
	ctx.Data["ForcePrivate"] = setting.Repository.ForcePrivate
	ctx.HTML(200, tplSettingsOptions)
}

// SettingsPost response for changes of a repository
func SettingsPost(ctx *context.Context, form auth.RepoSettingForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsOptions"] = true

	repo := ctx.Repo.Repository

	switch ctx.Query("action") {
	case "update":
		if ctx.HasError() {
			ctx.HTML(200, tplSettingsOptions)
			return
		}

		newRepoName := form.RepoName
		// Check if repository name has been changed.
		if repo.LowerName != strings.ToLower(newRepoName) {
			// Close the GitRepo if open
			if ctx.Repo.GitRepo != nil {
				ctx.Repo.GitRepo.Close()
				ctx.Repo.GitRepo = nil
			}
			if err := repo_service.ChangeRepositoryName(ctx.User, repo, newRepoName); err != nil {
				ctx.Data["Err_RepoName"] = true
				switch {
				case models.IsErrRepoAlreadyExist(err):
					ctx.RenderWithErr(ctx.Tr("form.repo_name_been_taken"), tplSettingsOptions, &form)
				case models.IsErrNameReserved(err):
					ctx.RenderWithErr(ctx.Tr("repo.form.name_reserved", err.(models.ErrNameReserved).Name), tplSettingsOptions, &form)
				case models.IsErrNamePatternNotAllowed(err):
					ctx.RenderWithErr(ctx.Tr("repo.form.name_pattern_not_allowed", err.(models.ErrNamePatternNotAllowed).Pattern), tplSettingsOptions, &form)
				default:
					ctx.ServerError("ChangeRepositoryName", err)
				}
				return
			}

			log.Trace("Repository name changed: %s/%s -> %s", ctx.Repo.Owner.Name, repo.Name, newRepoName)
		}
		// In case it's just a case change.
		repo.Name = newRepoName
		repo.LowerName = strings.ToLower(newRepoName)
		repo.Description = form.Description
		repo.Website = form.Website
		repo.IsTemplate = form.Template

		// Visibility of forked repository is forced sync with base repository.
		if repo.IsFork {
			form.Private = repo.BaseRepo.IsPrivate || repo.BaseRepo.Owner.Visibility == structs.VisibleTypePrivate
		}

		visibilityChanged := repo.IsPrivate != form.Private
		// when ForcePrivate enabled, you could change public repo to private, but only admin users can change private to public
		if visibilityChanged && setting.Repository.ForcePrivate && !form.Private && !ctx.User.IsAdmin {
			ctx.ServerError("Force Private enabled", errors.New("cannot change private repository to public"))
			return
		}

		repo.IsPrivate = form.Private
		if err := models.UpdateRepository(repo, visibilityChanged); err != nil {
			ctx.ServerError("UpdateRepository", err)
			return
		}
		log.Trace("Repository basic settings updated: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(repo.Link() + "/settings")

	case "mirror":
		if !repo.IsMirror {
			ctx.NotFound("", nil)
			return
		}

		// This section doesn't require repo_name/RepoName to be set in the form, don't show it
		// as an error on the UI for this action
		ctx.Data["Err_RepoName"] = nil

		interval, err := time.ParseDuration(form.Interval)
		if err != nil || (interval != 0 && interval < setting.Mirror.MinInterval) {
			ctx.Data["Err_Interval"] = true
			ctx.RenderWithErr(ctx.Tr("repo.mirror_interval_invalid"), tplSettingsOptions, &form)
		} else {
			ctx.Repo.Mirror.EnablePrune = form.EnablePrune
			ctx.Repo.Mirror.Interval = interval
			if interval != 0 {
				ctx.Repo.Mirror.NextUpdateUnix = timeutil.TimeStampNow().AddDuration(interval)
			} else {
				ctx.Repo.Mirror.NextUpdateUnix = 0
			}
			if err := models.UpdateMirror(ctx.Repo.Mirror); err != nil {
				ctx.Data["Err_Interval"] = true
				ctx.RenderWithErr(ctx.Tr("repo.mirror_interval_invalid"), tplSettingsOptions, &form)
				return
			}
		}

		// Validate the form.MirrorAddress
		u, err := url.Parse(form.MirrorAddress)
		if err != nil {
			ctx.Data["Err_MirrorAddress"] = true
			ctx.RenderWithErr(ctx.Tr("repo.mirror_address_url_invalid"), tplSettingsOptions, &form)
			return
		}

		if u.Opaque != "" || !(u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "git" || u.Scheme == "ssh") {
			ctx.Data["Err_MirrorAddress"] = true
			ctx.RenderWithErr(ctx.Tr("repo.mirror_address_protocol_invalid"), tplSettingsOptions, &form)
			return
		}

		if form.MirrorUsername != "" || form.MirrorPassword != "" {
			u.User = url.UserPassword(form.MirrorUsername, form.MirrorPassword)
		}

		// Now use xurls
		address := validFormAddress.FindString(form.MirrorAddress)
		if address != form.MirrorAddress && form.MirrorAddress != "" {
			ctx.Data["Err_MirrorAddress"] = true
			ctx.RenderWithErr(ctx.Tr("repo.mirror_address_url_invalid"), tplSettingsOptions, &form)
			return
		}

		if u.EscapedPath() == "" || u.Host == "" || !u.IsAbs() {
			ctx.Data["Err_MirrorAddress"] = true
			ctx.RenderWithErr(ctx.Tr("repo.mirror_address_url_invalid"), tplSettingsOptions, &form)
			return
		}

		address = u.String()

		if err := mirror_service.SaveAddress(ctx.Repo.Mirror, address); err != nil {
			ctx.ServerError("SaveAddress", err)
			return
		}

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(repo.Link() + "/settings")

	case "mirror-sync":
		if !repo.IsMirror {
			ctx.NotFound("", nil)
			return
		}

		mirror_service.StartToMirror(repo.ID)

		ctx.Flash.Info(ctx.Tr("repo.settings.mirror_sync_in_progress"))
		ctx.Redirect(repo.Link() + "/settings")

	case "advanced":
		var units []models.RepoUnit
		var deleteUnitTypes []models.UnitType

		// This section doesn't require repo_name/RepoName to be set in the form, don't show it
		// as an error on the UI for this action
		ctx.Data["Err_RepoName"] = nil

		if form.EnableWiki && form.EnableExternalWiki && !models.UnitTypeExternalWiki.UnitGlobalDisabled() {
			if !validation.IsValidExternalURL(form.ExternalWikiURL) {
				ctx.Flash.Error(ctx.Tr("repo.settings.external_wiki_url_error"))
				ctx.Redirect(repo.Link() + "/settings")
				return
			}

			units = append(units, models.RepoUnit{
				RepoID: repo.ID,
				Type:   models.UnitTypeExternalWiki,
				Config: &models.ExternalWikiConfig{
					ExternalWikiURL: form.ExternalWikiURL,
				},
			})
			deleteUnitTypes = append(deleteUnitTypes, models.UnitTypeWiki)
		} else if form.EnableWiki && !form.EnableExternalWiki && !models.UnitTypeWiki.UnitGlobalDisabled() {
			units = append(units, models.RepoUnit{
				RepoID: repo.ID,
				Type:   models.UnitTypeWiki,
				Config: new(models.UnitConfig),
			})
			deleteUnitTypes = append(deleteUnitTypes, models.UnitTypeExternalWiki)
		} else {
			if !models.UnitTypeExternalWiki.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, models.UnitTypeExternalWiki)
			}
			if !models.UnitTypeWiki.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, models.UnitTypeWiki)
			}
		}

		if form.EnableIssues && form.EnableExternalTracker && !models.UnitTypeExternalTracker.UnitGlobalDisabled() {
			if !validation.IsValidExternalURL(form.ExternalTrackerURL) {
				ctx.Flash.Error(ctx.Tr("repo.settings.external_tracker_url_error"))
				ctx.Redirect(repo.Link() + "/settings")
				return
			}
			if len(form.TrackerURLFormat) != 0 && !validation.IsValidExternalTrackerURLFormat(form.TrackerURLFormat) {
				ctx.Flash.Error(ctx.Tr("repo.settings.tracker_url_format_error"))
				ctx.Redirect(repo.Link() + "/settings")
				return
			}
			units = append(units, models.RepoUnit{
				RepoID: repo.ID,
				Type:   models.UnitTypeExternalTracker,
				Config: &models.ExternalTrackerConfig{
					ExternalTrackerURL:    form.ExternalTrackerURL,
					ExternalTrackerFormat: form.TrackerURLFormat,
					ExternalTrackerStyle:  form.TrackerIssueStyle,
				},
			})
			deleteUnitTypes = append(deleteUnitTypes, models.UnitTypeIssues)
		} else if form.EnableIssues && !form.EnableExternalTracker && !models.UnitTypeIssues.UnitGlobalDisabled() {
			units = append(units, models.RepoUnit{
				RepoID: repo.ID,
				Type:   models.UnitTypeIssues,
				Config: &models.IssuesConfig{
					EnableTimetracker:                form.EnableTimetracker,
					AllowOnlyContributorsToTrackTime: form.AllowOnlyContributorsToTrackTime,
					EnableDependencies:               form.EnableIssueDependencies,
				},
			})
			deleteUnitTypes = append(deleteUnitTypes, models.UnitTypeExternalTracker)
		} else {
			if !models.UnitTypeExternalTracker.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, models.UnitTypeExternalTracker)
			}
			if !models.UnitTypeIssues.UnitGlobalDisabled() {
				deleteUnitTypes = append(deleteUnitTypes, models.UnitTypeIssues)
			}
		}

		if form.EnablePulls && !models.UnitTypePullRequests.UnitGlobalDisabled() {
			units = append(units, models.RepoUnit{
				RepoID: repo.ID,
				Type:   models.UnitTypePullRequests,
				Config: &models.PullRequestsConfig{
					IgnoreWhitespaceConflicts: form.PullsIgnoreWhitespace,
					AllowMerge:                form.PullsAllowMerge,
					AllowRebase:               form.PullsAllowRebase,
					AllowRebaseMerge:          form.PullsAllowRebaseMerge,
					AllowSquash:               form.PullsAllowSquash,
				},
			})
		} else if !models.UnitTypePullRequests.UnitGlobalDisabled() {
			deleteUnitTypes = append(deleteUnitTypes, models.UnitTypePullRequests)
		}

		if err := models.UpdateRepositoryUnits(repo, units, deleteUnitTypes); err != nil {
			ctx.ServerError("UpdateRepositoryUnits", err)
			return
		}
		log.Trace("Repository advanced settings updated: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")

	case "admin":
		if !ctx.User.IsAdmin {
			ctx.Error(403)
			return
		}

		if repo.IsFsckEnabled != form.EnableHealthCheck {
			repo.IsFsckEnabled = form.EnableHealthCheck
		}

		if repo.CloseIssuesViaCommitInAnyBranch != form.EnableCloseIssuesViaCommitInAnyBranch {
			repo.CloseIssuesViaCommitInAnyBranch = form.EnableCloseIssuesViaCommitInAnyBranch
		}

		if err := models.UpdateRepository(repo, false); err != nil {
			ctx.ServerError("UpdateRepository", err)
			return
		}

		log.Trace("Repository admin settings updated: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.update_settings_success"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")

	case "convert":
		if !ctx.Repo.IsOwner() {
			ctx.Error(404)
			return
		}
		if repo.Name != form.RepoName {
			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_repo_name"), tplSettingsOptions, nil)
			return
		}

		if !repo.IsMirror {
			ctx.Error(404)
			return
		}
		repo.IsMirror = false

		if _, err := repository.CleanUpMigrateInfo(repo); err != nil {
			ctx.ServerError("CleanUpMigrateInfo", err)
			return
		} else if err = models.DeleteMirrorByRepoID(ctx.Repo.Repository.ID); err != nil {
			ctx.ServerError("DeleteMirrorByRepoID", err)
			return
		}
		log.Trace("Repository converted from mirror to regular: %s", repo.FullName())
		ctx.Flash.Success(ctx.Tr("repo.settings.convert_succeed"))
		ctx.Redirect(repo.Link())

	case "convert_fork":
		if !ctx.Repo.IsOwner() {
			ctx.Error(404)
			return
		}
		if err := repo.GetOwner(); err != nil {
			ctx.ServerError("Convert Fork", err)
			return
		}
		if repo.Name != form.RepoName {
			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_repo_name"), tplSettingsOptions, nil)
			return
		}

		if !repo.IsFork {
			ctx.Error(404)
			return
		}

		if !ctx.Repo.Owner.CanCreateRepo() {
			ctx.Flash.Error(ctx.Tr("repo.form.reach_limit_of_creation", ctx.User.MaxCreationLimit()))
			ctx.Redirect(repo.Link() + "/settings")
			return
		}

		repo.IsFork = false
		repo.ForkID = 0
		if err := models.UpdateRepository(repo, false); err != nil {
			log.Error("Unable to update repository %-v whilst converting from fork", repo)
			ctx.ServerError("Convert Fork", err)
			return
		}

		log.Trace("Repository converted from fork to regular: %s", repo.FullName())
		ctx.Flash.Success(ctx.Tr("repo.settings.convert_fork_succeed"))
		ctx.Redirect(repo.Link())

	case "transfer":
		if !ctx.Repo.IsOwner() {
			ctx.Error(404)
			return
		}
		if repo.Name != form.RepoName {
			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_repo_name"), tplSettingsOptions, nil)
			return
		}

		newOwner, err := models.GetUserByName(ctx.Query("new_owner_name"))
		if err != nil {
			if models.IsErrUserNotExist(err) {
				ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_owner_name"), tplSettingsOptions, nil)
				return
			}
			ctx.ServerError("IsUserExist", err)
			return
		}

		if newOwner.Type == models.UserTypeOrganization {
			if !ctx.User.IsAdmin && newOwner.Visibility == structs.VisibleTypePrivate && !ctx.User.IsUserPartOfOrg(newOwner.ID) {
				// The user shouldn't know about this organization
				ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_owner_name"), tplSettingsOptions, nil)
				return
			}
		}

		// Close the GitRepo if open
		if ctx.Repo.GitRepo != nil {
			ctx.Repo.GitRepo.Close()
			ctx.Repo.GitRepo = nil
		}
		if err = repo_service.TransferOwnership(ctx.User, newOwner, repo, nil); err != nil {
			if models.IsErrRepoAlreadyExist(err) {
				ctx.RenderWithErr(ctx.Tr("repo.settings.new_owner_has_same_repo"), tplSettingsOptions, nil)
			} else {
				ctx.ServerError("TransferOwnership", err)
			}
			return
		}

		log.Trace("Repository transferred: %s/%s -> %s", ctx.Repo.Owner.Name, repo.Name, newOwner)
		ctx.Flash.Success(ctx.Tr("repo.settings.transfer_succeed"))
		ctx.Redirect(setting.AppSubURL + "/" + newOwner.Name + "/" + repo.Name)

	case "delete":
		if !ctx.Repo.IsOwner() {
			ctx.Error(404)
			return
		}
		if repo.Name != form.RepoName {
			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_repo_name"), tplSettingsOptions, nil)
			return
		}

		if err := repo_service.DeleteRepository(ctx.User, ctx.Repo.Repository); err != nil {
			ctx.ServerError("DeleteRepository", err)
			return
		}
		log.Trace("Repository deleted: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.deletion_success"))
		ctx.Redirect(ctx.Repo.Owner.DashboardLink())

	case "delete-wiki":
		if !ctx.Repo.IsOwner() {
			ctx.Error(404)
			return
		}
		if repo.Name != form.RepoName {
			ctx.RenderWithErr(ctx.Tr("form.enterred_invalid_repo_name"), tplSettingsOptions, nil)
			return
		}

		err := repo.DeleteWiki()
		if err != nil {
			log.Error("Delete Wiki: %v", err.Error())
		}
		log.Trace("Repository wiki deleted: %s/%s", ctx.Repo.Owner.Name, repo.Name)

		ctx.Flash.Success(ctx.Tr("repo.settings.wiki_deletion_success"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")

	case "archive":
		if !ctx.Repo.IsOwner() {
			ctx.Error(403)
			return
		}

		if repo.IsMirror {
			ctx.Flash.Error(ctx.Tr("repo.settings.archive.error_ismirror"))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings")
			return
		}

		if err := repo.SetArchiveRepoState(true); err != nil {
			log.Error("Tried to archive a repo: %s", err)
			ctx.Flash.Error(ctx.Tr("repo.settings.archive.error"))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings")
			return
		}

		ctx.Flash.Success(ctx.Tr("repo.settings.archive.success"))

		log.Trace("Repository was archived: %s/%s", ctx.Repo.Owner.Name, repo.Name)
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")
	case "unarchive":
		if !ctx.Repo.IsOwner() {
			ctx.Error(403)
			return
		}

		if err := repo.SetArchiveRepoState(false); err != nil {
			log.Error("Tried to unarchive a repo: %s", err)
			ctx.Flash.Error(ctx.Tr("repo.settings.unarchive.error"))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings")
			return
		}

		ctx.Flash.Success(ctx.Tr("repo.settings.unarchive.success"))

		log.Trace("Repository was un-archived: %s/%s", ctx.Repo.Owner.Name, repo.Name)
		ctx.Redirect(ctx.Repo.RepoLink + "/settings")

	default:
		ctx.NotFound("", nil)
	}
}

// Collaboration render a repository's collaboration page
func Collaboration(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings")
	ctx.Data["PageIsSettingsCollaboration"] = true

	users, err := ctx.Repo.Repository.GetCollaborators(models.ListOptions{})
	if err != nil {
		ctx.ServerError("GetCollaborators", err)
		return
	}
	ctx.Data["Collaborators"] = users

	teams, err := ctx.Repo.Repository.GetRepoTeams()
	if err != nil {
		ctx.ServerError("GetRepoTeams", err)
		return
	}
	ctx.Data["Teams"] = teams
	ctx.Data["Repo"] = ctx.Repo.Repository
	ctx.Data["OrgID"] = ctx.Repo.Repository.OwnerID
	ctx.Data["OrgName"] = ctx.Repo.Repository.OwnerName
	ctx.Data["Org"] = ctx.Repo.Repository.Owner
	ctx.Data["Units"] = models.Units

	ctx.HTML(200, tplCollaboration)
}

// CollaborationPost response for actions for a collaboration of a repository
func CollaborationPost(ctx *context.Context) {
	name := utils.RemoveUsernameParameterSuffix(strings.ToLower(ctx.Query("collaborator")))
	if len(name) == 0 || ctx.Repo.Owner.LowerName == name {
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.Path)
		return
	}

	u, err := models.GetUserByName(name)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.user_not_exist"))
			ctx.Redirect(setting.AppSubURL + ctx.Req.URL.Path)
		} else {
			ctx.ServerError("GetUserByName", err)
		}
		return
	}

	if !u.IsActive {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_collaborator_inactive_user"))
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.Path)
		return
	}

	// Organization is not allowed to be added as a collaborator.
	if u.IsOrganization() {
		ctx.Flash.Error(ctx.Tr("repo.settings.org_not_allowed_to_be_collaborator"))
		ctx.Redirect(setting.AppSubURL + ctx.Req.URL.Path)
		return
	}

	if got, err := ctx.Repo.Repository.IsCollaborator(u.ID); err == nil && got {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_collaborator_duplicate"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	if err = ctx.Repo.Repository.AddCollaborator(u); err != nil {
		ctx.ServerError("AddCollaborator", err)
		return
	}

	if setting.Service.EnableNotifyMail {
		mailer.SendCollaboratorMail(u, ctx.User, ctx.Repo.Repository)
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_collaborator_success"))
	ctx.Redirect(setting.AppSubURL + ctx.Req.URL.Path)
}

// ChangeCollaborationAccessMode response for changing access of a collaboration
func ChangeCollaborationAccessMode(ctx *context.Context) {
	if err := ctx.Repo.Repository.ChangeCollaborationAccessMode(
		ctx.QueryInt64("uid"),
		models.AccessMode(ctx.QueryInt("mode"))); err != nil {
		log.Error("ChangeCollaborationAccessMode: %v", err)
	}
}

// DeleteCollaboration delete a collaboration for a repository
func DeleteCollaboration(ctx *context.Context) {
	if err := ctx.Repo.Repository.DeleteCollaboration(ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeleteCollaboration: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.remove_collaborator_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/settings/collaboration",
	})
}

// AddTeamPost response for adding a team to a repository
func AddTeamPost(ctx *context.Context) {
	if !ctx.Repo.Owner.RepoAdminChangeTeamAccess && !ctx.Repo.IsOwner() {
		ctx.Flash.Error(ctx.Tr("repo.settings.change_team_access_not_allowed"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	name := utils.RemoveUsernameParameterSuffix(strings.ToLower(ctx.Query("team")))
	if len(name) == 0 {
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	team, err := ctx.Repo.Owner.GetTeam(name)
	if err != nil {
		if models.IsErrTeamNotExist(err) {
			ctx.Flash.Error(ctx.Tr("form.team_not_exist"))
			ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		} else {
			ctx.ServerError("GetTeam", err)
		}
		return
	}

	if team.OrgID != ctx.Repo.Repository.OwnerID {
		ctx.Flash.Error(ctx.Tr("repo.settings.team_not_in_organization"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	if models.HasTeamRepo(ctx.Repo.Repository.OwnerID, team.ID, ctx.Repo.Repository.ID) {
		ctx.Flash.Error(ctx.Tr("repo.settings.add_team_duplicate"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	if err = team.AddRepository(ctx.Repo.Repository); err != nil {
		ctx.ServerError("team.AddRepository", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.add_team_success"))
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
}

// DeleteTeam response for deleting a team from a repository
func DeleteTeam(ctx *context.Context) {
	if !ctx.Repo.Owner.RepoAdminChangeTeamAccess && !ctx.Repo.IsOwner() {
		ctx.Flash.Error(ctx.Tr("repo.settings.change_team_access_not_allowed"))
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/collaboration")
		return
	}

	team, err := models.GetTeamByID(ctx.QueryInt64("id"))
	if err != nil {
		ctx.ServerError("GetTeamByID", err)
		return
	}

	if err = team.RemoveRepository(ctx.Repo.Repository.ID); err != nil {
		ctx.ServerError("team.RemoveRepositorys", err)
		return
	}

	ctx.Flash.Success(ctx.Tr("repo.settings.remove_team_success"))
	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/settings/collaboration",
	})
}

// parseOwnerAndRepo get repos by owner
func parseOwnerAndRepo(ctx *context.Context) (*models.User, *models.Repository) {
	owner, err := models.GetUserByName(ctx.Params(":username"))
	if err != nil {
		if models.IsErrUserNotExist(err) {
			ctx.NotFound("GetUserByName", err)
		} else {
			ctx.ServerError("GetUserByName", err)
		}
		return nil, nil
	}

	repo, err := models.GetRepositoryByName(owner.ID, ctx.Params(":reponame"))
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			ctx.NotFound("GetRepositoryByName", err)
		} else {
			ctx.ServerError("GetRepositoryByName", err)
		}
		return nil, nil
	}

	return owner, repo
}

// GitHooks hooks of a repository
func GitHooks(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.githooks")
	ctx.Data["PageIsSettingsGitHooks"] = true

	hooks, err := ctx.Repo.GitRepo.Hooks()
	if err != nil {
		ctx.ServerError("Hooks", err)
		return
	}
	ctx.Data["Hooks"] = hooks

	ctx.HTML(200, tplGithooks)
}

// GitHooksEdit render for editing a hook of repository page
func GitHooksEdit(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.githooks")
	ctx.Data["PageIsSettingsGitHooks"] = true
	ctx.Data["RequireSimpleMDE"] = true

	name := ctx.Params(":name")
	hook, err := ctx.Repo.GitRepo.GetHook(name)
	if err != nil {
		if err == git.ErrNotValidHook {
			ctx.NotFound("GetHook", err)
		} else {
			ctx.ServerError("GetHook", err)
		}
		return
	}
	ctx.Data["Hook"] = hook
	ctx.HTML(200, tplGithookEdit)
}

// GitHooksEditPost response for editing a git hook of a repository
func GitHooksEditPost(ctx *context.Context) {
	name := ctx.Params(":name")
	hook, err := ctx.Repo.GitRepo.GetHook(name)
	if err != nil {
		if err == git.ErrNotValidHook {
			ctx.NotFound("GetHook", err)
		} else {
			ctx.ServerError("GetHook", err)
		}
		return
	}
	hook.Content = ctx.Query("content")
	if err = hook.Update(); err != nil {
		ctx.ServerError("hook.Update", err)
		return
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/hooks/git")
}

// DeployKeys render the deploy keys list of a repository page
func DeployKeys(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.deploy_keys")
	ctx.Data["PageIsSettingsKeys"] = true
	ctx.Data["DisableSSH"] = setting.SSH.Disabled

	keys, err := models.ListDeployKeys(ctx.Repo.Repository.ID, models.ListOptions{})
	if err != nil {
		ctx.ServerError("ListDeployKeys", err)
		return
	}
	ctx.Data["Deploykeys"] = keys

	ctx.HTML(200, tplDeployKeys)
}

// DeployKeysPost response for adding a deploy key of a repository
func DeployKeysPost(ctx *context.Context, form auth.AddKeyForm) {
	ctx.Data["Title"] = ctx.Tr("repo.settings.deploy_keys")
	ctx.Data["PageIsSettingsKeys"] = true

	keys, err := models.ListDeployKeys(ctx.Repo.Repository.ID, models.ListOptions{})
	if err != nil {
		ctx.ServerError("ListDeployKeys", err)
		return
	}
	ctx.Data["Deploykeys"] = keys

	if ctx.HasError() {
		ctx.HTML(200, tplDeployKeys)
		return
	}

	content, err := models.CheckPublicKeyString(form.Content)
	if err != nil {
		if models.IsErrSSHDisabled(err) {
			ctx.Flash.Info(ctx.Tr("settings.ssh_disabled"))
		} else if models.IsErrKeyUnableVerify(err) {
			ctx.Flash.Info(ctx.Tr("form.unable_verify_ssh_key"))
		} else {
			ctx.Data["HasError"] = true
			ctx.Data["Err_Content"] = true
			ctx.Flash.Error(ctx.Tr("form.invalid_ssh_key", err.Error()))
		}
		ctx.Redirect(ctx.Repo.RepoLink + "/settings/keys")
		return
	}

	key, err := models.AddDeployKey(ctx.Repo.Repository.ID, form.Title, content, !form.IsWritable)
	if err != nil {
		ctx.Data["HasError"] = true
		switch {
		case models.IsErrDeployKeyAlreadyExist(err):
			ctx.Data["Err_Content"] = true
			ctx.RenderWithErr(ctx.Tr("repo.settings.key_been_used"), tplDeployKeys, &form)
		case models.IsErrKeyAlreadyExist(err):
			ctx.Data["Err_Content"] = true
			ctx.RenderWithErr(ctx.Tr("settings.ssh_key_been_used"), tplDeployKeys, &form)
		case models.IsErrKeyNameAlreadyUsed(err):
			ctx.Data["Err_Title"] = true
			ctx.RenderWithErr(ctx.Tr("repo.settings.key_name_used"), tplDeployKeys, &form)
		default:
			ctx.ServerError("AddDeployKey", err)
		}
		return
	}

	log.Trace("Deploy key added: %d", ctx.Repo.Repository.ID)
	ctx.Flash.Success(ctx.Tr("repo.settings.add_key_success", key.Name))
	ctx.Redirect(ctx.Repo.RepoLink + "/settings/keys")
}

// DeleteDeployKey response for deleting a deploy key
func DeleteDeployKey(ctx *context.Context) {
	if err := models.DeleteDeployKey(ctx.User, ctx.QueryInt64("id")); err != nil {
		ctx.Flash.Error("DeleteDeployKey: " + err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.deploy_key_deletion_success"))
	}

	ctx.JSON(200, map[string]interface{}{
		"redirect": ctx.Repo.RepoLink + "/settings/keys",
	})
}

func init() {
	var err error
	validFormAddress, err = xurls.StrictMatchingScheme(`(https?)|(git)|(ssh)://`)
	if err != nil {
		panic(err)
	}
}

// UpdateAvatarSetting update repo's avatar
func UpdateAvatarSetting(ctx *context.Context, form auth.AvatarForm) error {
	ctxRepo := ctx.Repo.Repository

	if form.Avatar == nil {
		// No avatar is uploaded and we not removing it here.
		// No random avatar generated here.
		// Just exit, no action.
		if !com.IsFile(ctxRepo.CustomAvatarPath()) {
			log.Trace("No avatar was uploaded for repo: %d. Default icon will appear instead.", ctxRepo.ID)
		}
		return nil
	}

	r, err := form.Avatar.Open()
	if err != nil {
		return fmt.Errorf("Avatar.Open: %v", err)
	}
	defer r.Close()

	if form.Avatar.Size > setting.AvatarMaxFileSize {
		return errors.New(ctx.Tr("settings.uploaded_avatar_is_too_big"))
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("ioutil.ReadAll: %v", err)
	}
	if !base.IsImageFile(data) {
		return errors.New(ctx.Tr("settings.uploaded_avatar_not_a_image"))
	}
	if err = ctxRepo.UploadAvatar(data); err != nil {
		return fmt.Errorf("UploadAvatar: %v", err)
	}
	return nil
}

// SettingsAvatar save new POSTed repository avatar
func SettingsAvatar(ctx *context.Context, form auth.AvatarForm) {
	form.Source = auth.AvatarLocal
	if err := UpdateAvatarSetting(ctx, form); err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("repo.settings.update_avatar_success"))
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings")
}

// SettingsDeleteAvatar delete repository avatar
func SettingsDeleteAvatar(ctx *context.Context) {
	if err := ctx.Repo.Repository.DeleteAvatar(); err != nil {
		ctx.Flash.Error(fmt.Sprintf("DeleteAvatar: %v", err))
	}
	ctx.Redirect(ctx.Repo.RepoLink + "/settings")
}
