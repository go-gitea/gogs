// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package v1 Gitea API.
//
// This documentation describes the Gitea API.
//
//     Schemes: http, https
//     BasePath: /api/v1
//     Version: 1.1.1
//     License: MIT http://opensource.org/licenses/MIT
//
//     Consumes:
//     - application/json
//     - text/plain
//
//     Produces:
//     - application/json
//     - text/html
//
//     Security:
//     - BasicAuth :
//     - Token :
//     - AccessToken :
//     - AuthorizationHeaderToken :
//     - SudoParam :
//     - SudoHeader :
//     - TOTPHeader :
//
//     SecurityDefinitions:
//     BasicAuth:
//          type: basic
//     Token:
//          type: apiKey
//          name: token
//          in: query
//     AccessToken:
//          type: apiKey
//          name: access_token
//          in: query
//     AuthorizationHeaderToken:
//          type: apiKey
//          name: Authorization
//          in: header
//          description: API tokens must be prepended with "token" followed by a space.
//     SudoParam:
//          type: apiKey
//          name: sudo
//          in: query
//          description: Sudo API request as the user provided as the key. Admin privileges are required.
//     SudoHeader:
//          type: apiKey
//          name: Sudo
//          in: header
//          description: Sudo API request as the user provided as the key. Admin privileges are required.
//     TOTPHeader:
//          type: apiKey
//          name: X-GITEA-OTP
//          in: header
//          description: Must be used in combination with BasicAuth if two-factor authentication is enabled.
//
// swagger:meta
package v1

import (
	"net/http"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/routers/api/v1/admin"
	"code.gitea.io/gitea/routers/api/v1/misc"
	"code.gitea.io/gitea/routers/api/v1/notify"
	"code.gitea.io/gitea/routers/api/v1/org"
	"code.gitea.io/gitea/routers/api/v1/repo"
	"code.gitea.io/gitea/routers/api/v1/settings"
	_ "code.gitea.io/gitea/routers/api/v1/swagger" // for swagger generation
	"code.gitea.io/gitea/routers/api/v1/user"

	"gitea.com/macaron/binding"
	"gitea.com/macaron/macaron"
)

func sudo() macaron.Handler {
	return func(ctx *context.APIContext) {
		sudo := ctx.Query("sudo")
		if len(sudo) == 0 {
			sudo = ctx.Req.Header.Get("Sudo")
		}

		if len(sudo) > 0 {
			if ctx.IsSigned && ctx.User.IsAdmin {
				user, err := models.GetUserByName(sudo)
				if err != nil {
					if models.IsErrUserNotExist(err) {
						ctx.NotFound()
					} else {
						ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
					}
					return
				}
				log.Trace("Sudo from (%s) to: %s", ctx.User.Name, user.Name)
				ctx.User = user
			} else {
				ctx.JSON(http.StatusForbidden, map[string]string{
					"message": "Only administrators allowed to sudo.",
				})
				return
			}
		}
	}
}

func repoAssignment() macaron.Handler {
	return func(ctx *context.APIContext) {
		userName := ctx.Params(":username")
		repoName := ctx.Params(":reponame")

		var (
			owner *models.User
			err   error
		)

		// Check if the user is the same as the repository owner.
		if ctx.IsSigned && ctx.User.LowerName == strings.ToLower(userName) {
			owner = ctx.User
		} else {
			owner, err = models.GetUserByName(userName)
			if err != nil {
				if models.IsErrUserNotExist(err) {
					ctx.NotFound()
				} else {
					ctx.Error(http.StatusInternalServerError, "GetUserByName", err)
				}
				return
			}
		}
		ctx.Repo.Owner = owner

		// Get repository.
		repo, err := models.GetRepositoryByName(owner.ID, repoName)
		if err != nil {
			if models.IsErrRepoNotExist(err) {
				redirectRepoID, err := models.LookupRepoRedirect(owner.ID, repoName)
				if err == nil {
					context.RedirectToRepo(ctx.Context, redirectRepoID)
				} else if models.IsErrRepoRedirectNotExist(err) {
					ctx.NotFound()
				} else {
					ctx.Error(http.StatusInternalServerError, "LookupRepoRedirect", err)
				}
			} else {
				ctx.Error(http.StatusInternalServerError, "GetRepositoryByName", err)
			}
			return
		}

		repo.Owner = owner
		ctx.Repo.Repository = repo

		ctx.Repo.Permission, err = models.GetUserRepoPermission(repo, ctx.User)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "GetUserRepoPermission", err)
			return
		}

		if !ctx.Repo.HasAccess() {
			ctx.NotFound()
			return
		}
	}
}

// Contexter middleware already checks token for user sign in process.
func reqToken() macaron.Handler {
	return func(ctx *context.APIContext) {
		if true == ctx.Data["IsApiToken"] {
			return
		}
		if ctx.Context.IsBasicAuth {
			ctx.CheckForOTP()
			return
		}
		if ctx.IsSigned {
			ctx.RequireCSRF()
			return
		}
		ctx.Context.Error(http.StatusUnauthorized)
	}
}

func reqBasicAuth() macaron.Handler {
	return func(ctx *context.APIContext) {
		if !ctx.Context.IsBasicAuth {
			ctx.Context.Error(http.StatusUnauthorized)
			return
		}
		ctx.CheckForOTP()
	}
}

// reqSiteAdmin user should be the site admin
func reqSiteAdmin() macaron.Handler {
	return func(ctx *context.Context) {
		if !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden)
			return
		}
	}
}

// reqOwner user should be the owner of the repo or site admin.
func reqOwner() macaron.Handler {
	return func(ctx *context.Context) {
		if !ctx.IsUserRepoOwner() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden)
			return
		}
	}
}

// reqAdmin user should be an owner or a collaborator with admin write of a repository, or site admin
func reqAdmin() macaron.Handler {
	return func(ctx *context.Context) {
		if !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden)
			return
		}
	}
}

// reqRepoWriter user should have a permission to write to a repo, or be a site admin
func reqRepoWriter(unitTypes ...models.UnitType) macaron.Handler {
	return func(ctx *context.Context) {
		if !ctx.IsUserRepoWriter(unitTypes) && !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden)
			return
		}
	}
}

// reqRepoReader user should have specific read permission or be a repo admin or a site admin
func reqRepoReader(unitType models.UnitType) macaron.Handler {
	return func(ctx *context.Context) {
		if !ctx.IsUserRepoReaderSpecific(unitType) && !ctx.IsUserRepoAdmin() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden)
			return
		}
	}
}

// reqAnyRepoReader user should have any permission to read repository or permissions of site admin
func reqAnyRepoReader() macaron.Handler {
	return func(ctx *context.Context) {
		if !ctx.IsUserRepoReaderAny() && !ctx.IsUserSiteAdmin() {
			ctx.Error(http.StatusForbidden)
			return
		}
	}
}

// reqOrgOwnership user should be an organization owner, or a site admin
func reqOrgOwnership() macaron.Handler {
	return func(ctx *context.APIContext) {
		if ctx.Context.IsUserSiteAdmin() {
			return
		}

		var orgID int64
		if ctx.Org.Organization != nil {
			orgID = ctx.Org.Organization.ID
		} else if ctx.Org.Team != nil {
			orgID = ctx.Org.Team.OrgID
		} else {
			ctx.Error(http.StatusInternalServerError, "", "reqOrgOwnership: unprepared context")
			return
		}

		isOwner, err := models.IsOrganizationOwner(orgID, ctx.User.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrganizationOwner", err)
			return
		} else if !isOwner {
			if ctx.Org.Organization != nil {
				ctx.Error(http.StatusForbidden, "", "Must be an organization owner")
			} else {
				ctx.NotFound()
			}
			return
		}
	}
}

// reqTeamMembership user should be an team member, or a site admin
func reqTeamMembership() macaron.Handler {
	return func(ctx *context.APIContext) {
		if ctx.Context.IsUserSiteAdmin() {
			return
		}
		if ctx.Org.Team == nil {
			ctx.Error(http.StatusInternalServerError, "", "reqTeamMembership: unprepared context")
			return
		}

		var orgID = ctx.Org.Team.OrgID
		isOwner, err := models.IsOrganizationOwner(orgID, ctx.User.ID)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrganizationOwner", err)
			return
		} else if isOwner {
			return
		}

		if isTeamMember, err := models.IsTeamMember(orgID, ctx.Org.Team.ID, ctx.User.ID); err != nil {
			ctx.Error(http.StatusInternalServerError, "IsTeamMember", err)
			return
		} else if !isTeamMember {
			isOrgMember, err := models.IsOrganizationMember(orgID, ctx.User.ID)
			if err != nil {
				ctx.Error(http.StatusInternalServerError, "IsOrganizationMember", err)
			} else if isOrgMember {
				ctx.Error(http.StatusForbidden, "", "Must be a team member")
			} else {
				ctx.NotFound()
			}
			return
		}
	}
}

// reqOrgMembership user should be an organization member, or a site admin
func reqOrgMembership() macaron.Handler {
	return func(ctx *context.APIContext) {
		if ctx.Context.IsUserSiteAdmin() {
			return
		}

		var orgID int64
		if ctx.Org.Organization != nil {
			orgID = ctx.Org.Organization.ID
		} else if ctx.Org.Team != nil {
			orgID = ctx.Org.Team.OrgID
		} else {
			ctx.Error(http.StatusInternalServerError, "", "reqOrgMembership: unprepared context")
			return
		}

		if isMember, err := models.IsOrganizationMember(orgID, ctx.User.ID); err != nil {
			ctx.Error(http.StatusInternalServerError, "IsOrganizationMember", err)
			return
		} else if !isMember {
			if ctx.Org.Organization != nil {
				ctx.Error(http.StatusForbidden, "", "Must be an organization member")
			} else {
				ctx.NotFound()
			}
			return
		}
	}
}

func reqGitHook() macaron.Handler {
	return func(ctx *context.APIContext) {
		if !ctx.User.CanEditGitHook() {
			ctx.Error(http.StatusForbidden, "", "must be allowed to edit Git hooks")
			return
		}
	}
}

func orgAssignment(args ...bool) macaron.Handler {
	var (
		assignOrg  bool
		assignTeam bool
	)
	if len(args) > 0 {
		assignOrg = args[0]
	}
	if len(args) > 1 {
		assignTeam = args[1]
	}
	return func(ctx *context.APIContext) {
		ctx.Org = new(context.APIOrganization)

		var err error
		if assignOrg {
			ctx.Org.Organization, err = models.GetOrgByName(ctx.Params(":org"))
			if err != nil {
				if models.IsErrOrgNotExist(err) {
					ctx.NotFound()
				} else {
					ctx.Error(http.StatusInternalServerError, "GetOrgByName", err)
				}
				return
			}
		}

		if assignTeam {
			ctx.Org.Team, err = models.GetTeamByID(ctx.ParamsInt64(":teamid"))
			if err != nil {
				if models.IsErrUserNotExist(err) {
					ctx.NotFound()
				} else {
					ctx.Error(http.StatusInternalServerError, "GetTeamById", err)
				}
				return
			}
		}
	}
}

func mustEnableIssues(ctx *context.APIContext) {
	if !ctx.Repo.CanRead(models.UnitTypeIssues) {
		if log.IsTrace() {
			if ctx.IsSigned {
				log.Trace("Permission Denied: User %-v cannot read %-v in Repo %-v\n"+
					"User in Repo has Permissions: %-+v",
					ctx.User,
					models.UnitTypeIssues,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Anonymous user cannot read %-v in Repo %-v\n"+
					"Anonymous user in Repo has Permissions: %-+v",
					models.UnitTypeIssues,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			}
		}
		ctx.NotFound()
		return
	}
}

func mustAllowPulls(ctx *context.APIContext) {
	if !(ctx.Repo.Repository.CanEnablePulls() && ctx.Repo.CanRead(models.UnitTypePullRequests)) {
		if ctx.Repo.Repository.CanEnablePulls() && log.IsTrace() {
			if ctx.IsSigned {
				log.Trace("Permission Denied: User %-v cannot read %-v in Repo %-v\n"+
					"User in Repo has Permissions: %-+v",
					ctx.User,
					models.UnitTypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Anonymous user cannot read %-v in Repo %-v\n"+
					"Anonymous user in Repo has Permissions: %-+v",
					models.UnitTypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			}
		}
		ctx.NotFound()
		return
	}
}

func mustEnableIssuesOrPulls(ctx *context.APIContext) {
	if !ctx.Repo.CanRead(models.UnitTypeIssues) &&
		!(ctx.Repo.Repository.CanEnablePulls() && ctx.Repo.CanRead(models.UnitTypePullRequests)) {
		if ctx.Repo.Repository.CanEnablePulls() && log.IsTrace() {
			if ctx.IsSigned {
				log.Trace("Permission Denied: User %-v cannot read %-v and %-v in Repo %-v\n"+
					"User in Repo has Permissions: %-+v",
					ctx.User,
					models.UnitTypeIssues,
					models.UnitTypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			} else {
				log.Trace("Permission Denied: Anonymous user cannot read %-v and %-v in Repo %-v\n"+
					"Anonymous user in Repo has Permissions: %-+v",
					models.UnitTypeIssues,
					models.UnitTypePullRequests,
					ctx.Repo.Repository,
					ctx.Repo.Permission)
			}
		}
		ctx.NotFound()
		return
	}
}

func mustEnableUserHeatmap(ctx *context.APIContext) {
	if !setting.Service.EnableUserHeatmap {
		ctx.NotFound()
		return
	}
}

func mustNotBeArchived(ctx *context.APIContext) {
	if ctx.Repo.Repository.IsArchived {
		ctx.NotFound()
		return
	}
}

// RegisterRoutes registers all v1 APIs routes to web application.
// FIXME: custom form error response
func RegisterRoutes(m *macaron.Macaron) {
	bind := binding.Bind

	if setting.API.EnableSwagger {
		m.Get("/swagger", misc.Swagger) // Render V1 by default
	}

	m.Group("/v1", func() {
		// Miscellaneous
		if setting.API.EnableSwagger {
			m.Get("/swagger", misc.Swagger)
		}
		m.Get("/version", misc.Version)
		m.Get("/signing-key.gpg", misc.SigningKey)
		m.Post("/markdown", bind(api.MarkdownOption{}), misc.Markdown)
		m.Post("/markdown/raw", misc.MarkdownRaw)
		m.Group("/settings", func() {
			m.Get("/ui", settings.GetGeneralUISettings)
			m.Get("/api", settings.GetGeneralAPISettings)
			m.Get("/attachment", settings.GetGeneralAttachmentSettings)
			m.Get("/repository", settings.GetGeneralRepoSettings)
		})

		// Notifications
		m.Group("/notifications", func() {
			m.Combo("").
				Get(notify.ListNotifications).
				Put(notify.ReadNotifications)
			m.Get("/new", notify.NewAvailable)
			m.Combo("/threads/:id").
				Get(notify.GetThread).
				Patch(notify.ReadThread)
		}, reqToken())

		// Users
		m.Group("/users", func() {
			m.Get("/search", user.Search)

			m.Group("/:username", func() {
				m.Get("", user.GetInfo)
				m.Get("/heatmap", mustEnableUserHeatmap, user.GetUserHeatmapData)

				m.Get("/repos", user.ListUserRepos)
				m.Group("/tokens", func() {
					m.Combo("").Get(user.ListAccessTokens).
						Post(bind(api.CreateAccessTokenOption{}), user.CreateAccessToken)
					m.Combo("/:id").Delete(user.DeleteAccessToken)
				}, reqBasicAuth())
			})
		})

		m.Group("/users", func() {
			m.Group("/:username", func() {
				m.Get("/keys", user.ListPublicKeys)
				m.Get("/gpg_keys", user.ListGPGKeys)

				m.Get("/followers", user.ListFollowers)
				m.Group("/following", func() {
					m.Get("", user.ListFollowing)
					m.Get("/:target", user.CheckFollowing)
				})

				m.Get("/starred", user.GetStarredRepos)

				m.Get("/subscriptions", user.GetWatchedRepos)
			})
		}, reqToken())

		m.Group("/user", func() {
			m.Get("", user.GetAuthenticatedUser)
			m.Combo("/emails").Get(user.ListEmails).
				Post(bind(api.CreateEmailOption{}), user.AddEmail).
				Delete(bind(api.DeleteEmailOption{}), user.DeleteEmail)

			m.Get("/followers", user.ListMyFollowers)
			m.Group("/following", func() {
				m.Get("", user.ListMyFollowing)
				m.Combo("/:username").Get(user.CheckMyFollowing).Put(user.Follow).Delete(user.Unfollow)
			})

			m.Group("/keys", func() {
				m.Combo("").Get(user.ListMyPublicKeys).
					Post(bind(api.CreateKeyOption{}), user.CreatePublicKey)
				m.Combo("/:id").Get(user.GetPublicKey).
					Delete(user.DeletePublicKey)
			})
			m.Group("/applications", func() {
				m.Combo("/oauth2").
					Get(user.ListOauth2Applications).
					Post(bind(api.CreateOAuth2ApplicationOptions{}), user.CreateOauth2Application)
				m.Combo("/oauth2/:id").
					Delete(user.DeleteOauth2Application).
					Patch(bind(api.CreateOAuth2ApplicationOptions{}), user.UpdateOauth2Application).
					Get(user.GetOauth2Application)
			}, reqToken())

			m.Group("/gpg_keys", func() {
				m.Combo("").Get(user.ListMyGPGKeys).
					Post(bind(api.CreateGPGKeyOption{}), user.CreateGPGKey)
				m.Combo("/:id").Get(user.GetGPGKey).
					Delete(user.DeleteGPGKey)
			})

			m.Combo("/repos").Get(user.ListMyRepos).
				Post(bind(api.CreateRepoOption{}), repo.Create)

			m.Group("/starred", func() {
				m.Get("", user.GetMyStarredRepos)
				m.Group("/:username/:reponame", func() {
					m.Get("", user.IsStarring)
					m.Put("", user.Star)
					m.Delete("", user.Unstar)
				}, repoAssignment())
			})
			m.Get("/times", repo.ListMyTrackedTimes)

			m.Get("/stopwatches", repo.GetStopwatches)

			m.Get("/subscriptions", user.GetMyWatchedRepos)

			m.Get("/teams", org.ListUserTeams)
		}, reqToken())

		// Repositories
		m.Post("/org/:org/repos", reqToken(), bind(api.CreateRepoOption{}), repo.CreateOrgRepoDeprecated)

		m.Combo("/repositories/:id", reqToken()).Get(repo.GetByID)

		m.Group("/repos", func() {
			m.Get("/search", repo.Search)

			m.Get("/issues/search", repo.SearchIssues)

			m.Post("/migrate", reqToken(), bind(api.MigrateRepoOptions{}), repo.Migrate)

			m.Group("/:username/:reponame", func() {
				m.Combo("").Get(reqAnyRepoReader(), repo.Get).
					Delete(reqToken(), reqOwner(), repo.Delete).
					Patch(reqToken(), reqAdmin(), bind(api.EditRepoOption{}), context.RepoRef(), repo.Edit)
				m.Post("/transfer", reqOwner(), bind(api.TransferRepoOption{}), repo.Transfer)
				m.Combo("/notifications").
					Get(reqToken(), notify.ListRepoNotifications).
					Put(reqToken(), notify.ReadRepoNotifications)
				m.Group("/hooks", func() {
					m.Combo("").Get(repo.ListHooks).
						Post(bind(api.CreateHookOption{}), repo.CreateHook)
					m.Group("/:id", func() {
						m.Combo("").Get(repo.GetHook).
							Patch(bind(api.EditHookOption{}), repo.EditHook).
							Delete(repo.DeleteHook)
						m.Post("/tests", context.RepoRef(), repo.TestHook)
					})
					m.Group("/git", func() {
						m.Combo("").Get(repo.ListGitHooks)
						m.Group("/:id", func() {
							m.Combo("").Get(repo.GetGitHook).
								Patch(bind(api.EditGitHookOption{}), repo.EditGitHook).
								Delete(repo.DeleteGitHook)
						})
					}, reqGitHook(), context.ReferencesGitRepo(true))
				}, reqToken(), reqAdmin())
				m.Group("/collaborators", func() {
					m.Get("", reqAnyRepoReader(), repo.ListCollaborators)
					m.Combo("/:collaborator").Get(reqAnyRepoReader(), repo.IsCollaborator).
						Put(reqAdmin(), bind(api.AddCollaboratorOption{}), repo.AddCollaborator).
						Delete(reqAdmin(), repo.DeleteCollaborator)
				}, reqToken())
				m.Get("/raw/*", context.RepoRefByType(context.RepoRefAny), reqRepoReader(models.UnitTypeCode), repo.GetRawFile)
				m.Get("/archive/*", reqRepoReader(models.UnitTypeCode), repo.GetArchive)
				m.Combo("/forks").Get(repo.ListForks).
					Post(reqToken(), reqRepoReader(models.UnitTypeCode), bind(api.CreateForkOption{}), repo.CreateFork)
				m.Group("/branches", func() {
					m.Get("", repo.ListBranches)
					m.Get("/*", context.RepoRefByType(context.RepoRefBranch), repo.GetBranch)
					m.Delete("/*", reqRepoWriter(models.UnitTypeCode), context.RepoRefByType(context.RepoRefBranch), repo.DeleteBranch)
					m.Post("", reqRepoWriter(models.UnitTypeCode), bind(api.CreateBranchRepoOption{}), repo.CreateBranch)
				}, reqRepoReader(models.UnitTypeCode))
				m.Group("/branch_protections", func() {
					m.Get("", repo.ListBranchProtections)
					m.Post("", bind(api.CreateBranchProtectionOption{}), repo.CreateBranchProtection)
					m.Group("/:name", func() {
						m.Get("", repo.GetBranchProtection)
						m.Patch("", bind(api.EditBranchProtectionOption{}), repo.EditBranchProtection)
						m.Delete("", repo.DeleteBranchProtection)
					})
				}, reqToken(), reqAdmin())
				m.Group("/tags", func() {
					m.Get("", repo.ListTags)
				}, reqRepoReader(models.UnitTypeCode), context.ReferencesGitRepo(true))
				m.Group("/keys", func() {
					m.Combo("").Get(repo.ListDeployKeys).
						Post(bind(api.CreateKeyOption{}), repo.CreateDeployKey)
					m.Combo("/:id").Get(repo.GetDeployKey).
						Delete(repo.DeleteDeploykey)
				}, reqToken(), reqAdmin())
				m.Group("/times", func() {
					m.Combo("").Get(repo.ListTrackedTimesByRepository)
					m.Combo("/:timetrackingusername").Get(repo.ListTrackedTimesByUser)
				}, mustEnableIssues, reqToken())
				m.Group("/issues", func() {
					m.Combo("").Get(repo.ListIssues).
						Post(reqToken(), mustNotBeArchived, bind(api.CreateIssueOption{}), repo.CreateIssue)
					m.Group("/comments", func() {
						m.Get("", repo.ListRepoIssueComments)
						m.Group("/:id", func() {
							m.Combo("").
								Get(repo.GetIssueComment).
								Patch(mustNotBeArchived, reqToken(), bind(api.EditIssueCommentOption{}), repo.EditIssueComment).
								Delete(reqToken(), repo.DeleteIssueComment)
							m.Combo("/reactions").
								Get(repo.GetIssueCommentReactions).
								Post(bind(api.EditReactionOption{}), reqToken(), repo.PostIssueCommentReaction).
								Delete(bind(api.EditReactionOption{}), reqToken(), repo.DeleteIssueCommentReaction)
						})
					})
					m.Group("/:index", func() {
						m.Combo("").Get(repo.GetIssue).
							Patch(reqToken(), bind(api.EditIssueOption{}), repo.EditIssue)
						m.Group("/comments", func() {
							m.Combo("").Get(repo.ListIssueComments).
								Post(reqToken(), mustNotBeArchived, bind(api.CreateIssueCommentOption{}), repo.CreateIssueComment)
							m.Combo("/:id", reqToken()).Patch(bind(api.EditIssueCommentOption{}), repo.EditIssueCommentDeprecated).
								Delete(repo.DeleteIssueCommentDeprecated)
						})
						m.Group("/labels", func() {
							m.Combo("").Get(repo.ListIssueLabels).
								Post(reqToken(), bind(api.IssueLabelsOption{}), repo.AddIssueLabels).
								Put(reqToken(), bind(api.IssueLabelsOption{}), repo.ReplaceIssueLabels).
								Delete(reqToken(), repo.ClearIssueLabels)
							m.Delete("/:id", reqToken(), repo.DeleteIssueLabel)
						})
						m.Group("/times", func() {
							m.Combo("").
								Get(repo.ListTrackedTimes).
								Post(bind(api.AddTimeOption{}), repo.AddTime).
								Delete(repo.ResetIssueTime)
							m.Delete("/:id", repo.DeleteTime)
						}, reqToken())
						m.Combo("/deadline").Post(reqToken(), bind(api.EditDeadlineOption{}), repo.UpdateIssueDeadline)
						m.Group("/stopwatch", func() {
							m.Post("/start", reqToken(), repo.StartIssueStopwatch)
							m.Post("/stop", reqToken(), repo.StopIssueStopwatch)
							m.Delete("/delete", reqToken(), repo.DeleteIssueStopwatch)
						})
						m.Group("/subscriptions", func() {
							m.Get("", repo.GetIssueSubscribers)
							m.Get("/check", reqToken(), repo.CheckIssueSubscription)
							m.Put("/:user", reqToken(), repo.AddIssueSubscription)
							m.Delete("/:user", reqToken(), repo.DelIssueSubscription)
						})
						m.Combo("/reactions").
							Get(repo.GetIssueReactions).
							Post(bind(api.EditReactionOption{}), reqToken(), repo.PostIssueReaction).
							Delete(bind(api.EditReactionOption{}), reqToken(), repo.DeleteIssueReaction)
					})
				}, mustEnableIssuesOrPulls)
				m.Group("/labels", func() {
					m.Combo("").Get(repo.ListLabels).
						Post(reqToken(), reqRepoWriter(models.UnitTypeIssues, models.UnitTypePullRequests), bind(api.CreateLabelOption{}), repo.CreateLabel)
					m.Combo("/:id").Get(repo.GetLabel).
						Patch(reqToken(), reqRepoWriter(models.UnitTypeIssues, models.UnitTypePullRequests), bind(api.EditLabelOption{}), repo.EditLabel).
						Delete(reqToken(), reqRepoWriter(models.UnitTypeIssues, models.UnitTypePullRequests), repo.DeleteLabel)
				})
				m.Post("/markdown", bind(api.MarkdownOption{}), misc.Markdown)
				m.Post("/markdown/raw", misc.MarkdownRaw)
				m.Group("/milestones", func() {
					m.Combo("").Get(repo.ListMilestones).
						Post(reqToken(), reqRepoWriter(models.UnitTypeIssues, models.UnitTypePullRequests), bind(api.CreateMilestoneOption{}), repo.CreateMilestone)
					m.Combo("/:id").Get(repo.GetMilestone).
						Patch(reqToken(), reqRepoWriter(models.UnitTypeIssues, models.UnitTypePullRequests), bind(api.EditMilestoneOption{}), repo.EditMilestone).
						Delete(reqToken(), reqRepoWriter(models.UnitTypeIssues, models.UnitTypePullRequests), repo.DeleteMilestone)
				})
				m.Get("/stargazers", repo.ListStargazers)
				m.Get("/subscribers", repo.ListSubscribers)
				m.Group("/subscription", func() {
					m.Get("", user.IsWatching)
					m.Put("", reqToken(), user.Watch)
					m.Delete("", reqToken(), user.Unwatch)
				})
				m.Group("/releases", func() {
					m.Combo("").Get(repo.ListReleases).
						Post(reqToken(), reqRepoWriter(models.UnitTypeReleases), context.ReferencesGitRepo(false), bind(api.CreateReleaseOption{}), repo.CreateRelease)
					m.Group("/:id", func() {
						m.Combo("").Get(repo.GetRelease).
							Patch(reqToken(), reqRepoWriter(models.UnitTypeReleases), context.ReferencesGitRepo(false), bind(api.EditReleaseOption{}), repo.EditRelease).
							Delete(reqToken(), reqRepoWriter(models.UnitTypeReleases), repo.DeleteRelease)
						m.Group("/assets", func() {
							m.Combo("").Get(repo.ListReleaseAttachments).
								Post(reqToken(), reqRepoWriter(models.UnitTypeReleases), repo.CreateReleaseAttachment)
							m.Combo("/:asset").Get(repo.GetReleaseAttachment).
								Patch(reqToken(), reqRepoWriter(models.UnitTypeReleases), bind(api.EditAttachmentOptions{}), repo.EditReleaseAttachment).
								Delete(reqToken(), reqRepoWriter(models.UnitTypeReleases), repo.DeleteReleaseAttachment)
						})
					})
					m.Group("/tags", func() {
						m.Get("/:tag", repo.GetReleaseTag)
					})
				}, reqRepoReader(models.UnitTypeReleases))
				m.Post("/mirror-sync", reqToken(), reqRepoWriter(models.UnitTypeCode), repo.MirrorSync)
				m.Get("/editorconfig/:filename", context.RepoRef(), reqRepoReader(models.UnitTypeCode), repo.GetEditorconfig)
				m.Group("/pulls", func() {
					m.Combo("").Get(bind(api.ListPullRequestsOptions{}), repo.ListPullRequests).
						Post(reqToken(), mustNotBeArchived, bind(api.CreatePullRequestOption{}), repo.CreatePullRequest)
					m.Group("/:index", func() {
						m.Combo("").Get(repo.GetPullRequest).
							Patch(reqToken(), reqRepoWriter(models.UnitTypePullRequests), bind(api.EditPullRequestOption{}), repo.EditPullRequest)
						m.Get(".diff", repo.DownloadPullDiff)
						m.Get(".patch", repo.DownloadPullPatch)
						m.Post("/update", reqToken(), repo.UpdatePullRequest)
						m.Combo("/merge").Get(repo.IsPullRequestMerged).
							Post(reqToken(), mustNotBeArchived, bind(auth.MergePullRequestForm{}), repo.MergePullRequest)
						m.Group("/reviews", func() {
							m.Combo("").
								Get(repo.ListPullReviews).
								Post(reqToken(), bind(api.CreatePullReviewOptions{}), repo.CreatePullReview)
							m.Group("/:id", func() {
								m.Combo("").
									Get(repo.GetPullReview).
									Delete(reqToken(), repo.DeletePullReview).
									Post(reqToken(), bind(api.SubmitPullReviewOptions{}), repo.SubmitPullReview)
								m.Combo("/comments").
									Get(repo.GetPullReviewComments)
							})
						})

					})
				}, mustAllowPulls, reqRepoReader(models.UnitTypeCode), context.ReferencesGitRepo(false))
				m.Group("/statuses", func() {
					m.Combo("/:sha").Get(repo.GetCommitStatuses).
						Post(reqToken(), bind(api.CreateStatusOption{}), repo.NewCommitStatus)
				}, reqRepoReader(models.UnitTypeCode))
				m.Group("/commits", func() {
					m.Get("", repo.GetAllCommits)
					m.Group("/:ref", func() {
						m.Get("/status", repo.GetCombinedCommitStatusByRef)
						m.Get("/statuses", repo.GetCommitStatusesByRef)
					})
				}, reqRepoReader(models.UnitTypeCode))
				m.Group("/git", func() {
					m.Group("/commits", func() {
						m.Get("/:sha", repo.GetSingleCommit)
					})
					m.Get("/refs", repo.GetGitAllRefs)
					m.Get("/refs/*", repo.GetGitRefs)
					m.Get("/trees/:sha", context.RepoRef(), repo.GetTree)
					m.Get("/blobs/:sha", context.RepoRef(), repo.GetBlob)
					m.Get("/tags/:sha", context.RepoRef(), repo.GetTag)
				}, reqRepoReader(models.UnitTypeCode))
				m.Post("/diffpatch", reqRepoWriter(models.UnitTypeCode), reqToken(), bind(api.ApplyDiffPatchFileOptions{}), repo.ApplyDiffPatch)
				m.Group("/contents", func() {
					m.Get("", repo.GetContentsList)
					m.Get("/*", repo.GetContents)
					m.Group("/*", func() {
						m.Post("", bind(api.CreateFileOptions{}), repo.CreateFile)
						m.Put("", bind(api.UpdateFileOptions{}), repo.UpdateFile)
						m.Delete("", bind(api.DeleteFileOptions{}), repo.DeleteFile)
					}, reqRepoWriter(models.UnitTypeCode), reqToken())
				}, reqRepoReader(models.UnitTypeCode))
				m.Get("/signing-key.gpg", misc.SigningKey)
				m.Group("/topics", func() {
					m.Combo("").Get(repo.ListTopics).
						Put(reqToken(), reqAdmin(), bind(api.RepoTopicOptions{}), repo.UpdateTopics)
					m.Group("/:topic", func() {
						m.Combo("").Put(reqToken(), repo.AddTopic).
							Delete(reqToken(), repo.DeleteTopic)
					}, reqAdmin())
				}, reqAnyRepoReader())
				m.Get("/issue_templates", context.ReferencesGitRepo(false), repo.GetIssueTemplates)
				m.Get("/languages", reqRepoReader(models.UnitTypeCode), repo.GetLanguages)
			}, repoAssignment())
		})

		// Organizations
		m.Get("/user/orgs", reqToken(), org.ListMyOrgs)
		m.Get("/users/:username/orgs", org.ListUserOrgs)
		m.Post("/orgs", reqToken(), bind(api.CreateOrgOption{}), org.Create)
		m.Get("/orgs", org.GetAll)
		m.Group("/orgs/:org", func() {
			m.Combo("").Get(org.Get).
				Patch(reqToken(), reqOrgOwnership(), bind(api.EditOrgOption{}), org.Edit).
				Delete(reqToken(), reqOrgOwnership(), org.Delete)
			m.Combo("/repos").Get(user.ListOrgRepos).
				Post(reqToken(), bind(api.CreateRepoOption{}), repo.CreateOrgRepo)
			m.Group("/members", func() {
				m.Get("", org.ListMembers)
				m.Combo("/:username").Get(org.IsMember).
					Delete(reqToken(), reqOrgOwnership(), org.DeleteMember)
			})
			m.Group("/public_members", func() {
				m.Get("", org.ListPublicMembers)
				m.Combo("/:username").Get(org.IsPublicMember).
					Put(reqToken(), reqOrgMembership(), org.PublicizeMember).
					Delete(reqToken(), reqOrgMembership(), org.ConcealMember)
			})
			m.Group("/teams", func() {
				m.Combo("", reqToken()).Get(org.ListTeams).
					Post(reqOrgOwnership(), bind(api.CreateTeamOption{}), org.CreateTeam)
				m.Get("/search", org.SearchTeam)
			}, reqOrgMembership())
			m.Group("/labels", func() {
				m.Get("", org.ListLabels)
				m.Post("", reqToken(), reqOrgOwnership(), bind(api.CreateLabelOption{}), org.CreateLabel)
				m.Combo("/:id").Get(org.GetLabel).
					Patch(reqToken(), reqOrgOwnership(), bind(api.EditLabelOption{}), org.EditLabel).
					Delete(reqToken(), reqOrgOwnership(), org.DeleteLabel)
			})
			m.Group("/hooks", func() {
				m.Combo("").Get(org.ListHooks).
					Post(bind(api.CreateHookOption{}), org.CreateHook)
				m.Combo("/:id").Get(org.GetHook).
					Patch(bind(api.EditHookOption{}), org.EditHook).
					Delete(org.DeleteHook)
			}, reqToken(), reqOrgOwnership())
		}, orgAssignment(true))
		m.Group("/teams/:teamid", func() {
			m.Combo("").Get(org.GetTeam).
				Patch(reqOrgOwnership(), bind(api.EditTeamOption{}), org.EditTeam).
				Delete(reqOrgOwnership(), org.DeleteTeam)
			m.Group("/members", func() {
				m.Get("", org.GetTeamMembers)
				m.Combo("/:username").
					Get(org.GetTeamMember).
					Put(reqOrgOwnership(), org.AddTeamMember).
					Delete(reqOrgOwnership(), org.RemoveTeamMember)
			})
			m.Group("/repos", func() {
				m.Get("", org.GetTeamRepos)
				m.Combo("/:org/:reponame").
					Put(org.AddTeamRepository).
					Delete(org.RemoveTeamRepository)
			})
		}, orgAssignment(false, true), reqToken(), reqTeamMembership())

		m.Any("/*", func(ctx *context.APIContext) {
			ctx.NotFound()
		})

		m.Group("/admin", func() {
			m.Group("/cron", func() {
				m.Get("", admin.ListCronTasks)
				m.Post("/:task", admin.PostCronTask)
			})
			m.Get("/orgs", admin.GetAllOrgs)
			m.Group("/users", func() {
				m.Get("", admin.GetAllUsers)
				m.Post("", bind(api.CreateUserOption{}), admin.CreateUser)
				m.Group("/:username", func() {
					m.Combo("").Patch(bind(api.EditUserOption{}), admin.EditUser).
						Delete(admin.DeleteUser)
					m.Group("/keys", func() {
						m.Post("", bind(api.CreateKeyOption{}), admin.CreatePublicKey)
						m.Delete("/:id", admin.DeleteUserPublicKey)
					})
					m.Get("/orgs", org.ListUserOrgs)
					m.Post("/orgs", bind(api.CreateOrgOption{}), admin.CreateOrg)
					m.Post("/repos", bind(api.CreateRepoOption{}), admin.CreateRepo)
				})
			})
			m.Group("/unadopted", func() {
				m.Get("", admin.ListUnadoptedRepositories)
				m.Post("/:username/:reponame", admin.AdoptRepository)
				m.Delete("/:username/:reponame", admin.DeleteUnadoptedRepository)
			})
		}, reqToken(), reqSiteAdmin())

		m.Group("/topics", func() {
			m.Get("/search", repo.TopicSearch)
		})
	}, securityHeaders(), context.APIContexter(), sudo())
}

func securityHeaders() macaron.Handler {
	return func(ctx *macaron.Context) {
		ctx.Resp.Before(func(w macaron.ResponseWriter) {
			// CORB: https://www.chromium.org/Home/chromium-security/corb-for-developers
			// http://stackoverflow.com/a/3146618/244009
			w.Header().Set("x-content-type-options", "nosniff")
		})
	}
}
