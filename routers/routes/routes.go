// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routes

import (
	"encoding/gob"
	"net/http"
	"os"
	"path"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/gzip"
	"code.gitea.io/gitea/modules/lfs"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/metrics"
	"code.gitea.io/gitea/modules/options"
	"code.gitea.io/gitea/modules/public"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/validation"
	"code.gitea.io/gitea/routers"
	"code.gitea.io/gitea/routers/admin"
	apiv1 "code.gitea.io/gitea/routers/api/v1"
	"code.gitea.io/gitea/routers/dev"
	"code.gitea.io/gitea/routers/org"
	"code.gitea.io/gitea/routers/private"
	"code.gitea.io/gitea/routers/repo"
	"code.gitea.io/gitea/routers/user"
	userSetting "code.gitea.io/gitea/routers/user/setting"

	"github.com/go-macaron/binding"
	"github.com/go-macaron/cache"
	"github.com/go-macaron/captcha"
	"github.com/go-macaron/csrf"
	"github.com/go-macaron/i18n"
	"github.com/go-macaron/session"
	"github.com/go-macaron/toolbox"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tstranex/u2f"
	macaron "gopkg.in/macaron.v1"
)

// NewMacaron initializes Macaron instance.
func NewMacaron() *macaron.Macaron {
	gob.Register(&u2f.Challenge{})
	m := macaron.New()
	if !setting.DisableRouterLog {
		m.Use(macaron.Logger())
	}
	m.Use(macaron.Recovery())
	if setting.EnableGzip {
		m.Use(gzip.Middleware())
	}
	if setting.Protocol == setting.FCGI {
		m.SetURLPrefix(setting.AppSubURL)
	}
	m.Use(public.Custom(
		&public.Options{
			SkipLogging:  setting.DisableRouterLog,
			ExpiresAfter: time.Hour * 6,
		},
	))
	m.Use(public.Static(
		&public.Options{
			Directory:    path.Join(setting.StaticRootPath, "public"),
			SkipLogging:  setting.DisableRouterLog,
			ExpiresAfter: time.Hour * 6,
		},
	))
	m.Use(public.StaticHandler(
		setting.AvatarUploadPath,
		&public.Options{
			Prefix:       "avatars",
			SkipLogging:  setting.DisableRouterLog,
			ExpiresAfter: time.Hour * 6,
		},
	))

	m.Use(templates.HTMLRenderer())
	models.InitMailRender(templates.Mailer())

	localeNames, err := options.Dir("locale")

	if err != nil {
		log.Fatal(4, "Failed to list locale files: %v", err)
	}

	localFiles := make(map[string][]byte)

	for _, name := range localeNames {
		localFiles[name], err = options.Locale(name)

		if err != nil {
			log.Fatal(4, "Failed to load %s locale file. %v", name, err)
		}
	}

	m.Use(i18n.I18n(i18n.Options{
		SubURL:      setting.AppSubURL,
		Files:       localFiles,
		Langs:       setting.Langs,
		Names:       setting.Names,
		DefaultLang: "en-US",
		Redirect:    true,
	}))
	m.Use(cache.Cacher(cache.Options{
		Adapter:       setting.CacheService.Adapter,
		AdapterConfig: setting.CacheService.Conn,
		Interval:      setting.CacheService.Interval,
	}))
	m.Use(captcha.Captchaer(captcha.Options{
		SubURL: setting.AppSubURL,
	}))
	m.Use(session.Sessioner(setting.SessionConfig))
	m.Use(csrf.Csrfer(csrf.Options{
		Secret:         setting.SecretKey,
		Cookie:         setting.CSRFCookieName,
		SetCookie:      true,
		Secure:         setting.SessionConfig.Secure,
		CookieHttpOnly: true,
		Header:         "X-Csrf-Token",
		CookiePath:     setting.AppSubURL,
	}))
	m.Use(toolbox.Toolboxer(m, toolbox.Options{
		HealthCheckFuncs: []*toolbox.HealthCheckFuncDesc{
			{
				Desc: "Database connection",
				Func: models.Ping,
			},
		},
		DisableDebug: !setting.EnablePprof,
	}))
	m.Use(context.Contexter())
	m.SetAutoHead(true)
	return m
}

// RegisterRoutes routes routes to Macaron
func RegisterRoutes(m *macaron.Macaron) {
	reqSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: true})
	ignSignIn := context.Toggle(&context.ToggleOptions{SignInRequired: setting.Service.RequireSignInView})
	ignSignInAndCsrf := context.Toggle(&context.ToggleOptions{DisableCSRF: true})
	reqSignOut := context.Toggle(&context.ToggleOptions{SignOutRequired: true})

	bindIgnErr := binding.BindIgnErr
	validation.AddBindingRules()

	openIDSignInEnabled := func(ctx *context.Context) {
		if !setting.Service.EnableOpenIDSignIn {
			ctx.Error(403)
			return
		}
	}

	openIDSignUpEnabled := func(ctx *context.Context) {
		if !setting.Service.EnableOpenIDSignUp {
			ctx.Error(403)
			return
		}
	}

	m.Use(user.GetNotificationCount)

	// FIXME: not all routes need go through same middlewares.
	// Especially some AJAX requests, we can reduce middleware number to improve performance.
	// Routers.
	// for health check
	m.Head("/", func() string {
		return ""
	})
	m.Get("/", routers.Home)
	m.Group("/explore", func() {
		m.Get("", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL + "/explore/repos")
		})
		m.Get("/repos", routers.ExploreRepos)
		m.Get("/users", routers.ExploreUsers)
		m.Get("/organizations", routers.ExploreOrganizations)
		m.Get("/code", routers.ExploreCode)
	}, ignSignIn)
	m.Combo("/install", routers.InstallInit).Get(routers.Install).
		Post(bindIgnErr(auth.InstallForm{}), routers.InstallPost)
	m.Get("/^:type(issues|pulls)$", reqSignIn, user.Issues)

	// ***** START: User *****
	m.Group("/user", func() {
		m.Get("/login", user.SignIn)
		m.Post("/login", bindIgnErr(auth.SignInForm{}), user.SignInPost)
		m.Group("", func() {
			m.Combo("/login/openid").
				Get(user.SignInOpenID).
				Post(bindIgnErr(auth.SignInOpenIDForm{}), user.SignInOpenIDPost)
		}, openIDSignInEnabled)
		m.Group("/openid", func() {
			m.Combo("/connect").
				Get(user.ConnectOpenID).
				Post(bindIgnErr(auth.ConnectOpenIDForm{}), user.ConnectOpenIDPost)
			m.Group("/register", func() {
				m.Combo("").
					Get(user.RegisterOpenID, openIDSignUpEnabled).
					Post(bindIgnErr(auth.SignUpOpenIDForm{}), user.RegisterOpenIDPost)
			}, openIDSignUpEnabled)
		}, openIDSignInEnabled)
		m.Get("/sign_up", user.SignUp)
		m.Post("/sign_up", bindIgnErr(auth.RegisterForm{}), user.SignUpPost)
		m.Get("/reset_password", user.ResetPasswd)
		m.Post("/reset_password", user.ResetPasswdPost)
		m.Group("/oauth2", func() {
			m.Get("/:provider", user.SignInOAuth)
			m.Get("/:provider/callback", user.SignInOAuthCallback)
		})
		m.Get("/link_account", user.LinkAccount)
		m.Post("/link_account_signin", bindIgnErr(auth.SignInForm{}), user.LinkAccountPostSignIn)
		m.Post("/link_account_signup", bindIgnErr(auth.RegisterForm{}), user.LinkAccountPostRegister)
		m.Group("/two_factor", func() {
			m.Get("", user.TwoFactor)
			m.Post("", bindIgnErr(auth.TwoFactorAuthForm{}), user.TwoFactorPost)
			m.Get("/scratch", user.TwoFactorScratch)
			m.Post("/scratch", bindIgnErr(auth.TwoFactorScratchAuthForm{}), user.TwoFactorScratchPost)
		})
		m.Group("/u2f", func() {
			m.Get("", user.U2F)
			m.Get("/challenge", user.U2FChallenge)
			m.Post("/sign", bindIgnErr(u2f.SignResponse{}), user.U2FSign)

		})
	}, reqSignOut)

	m.Group("/user/settings", func() {
		m.Get("", userSetting.Profile)
		m.Post("", bindIgnErr(auth.UpdateProfileForm{}), userSetting.ProfilePost)
		m.Get("/change_password", user.MustChangePassword)
		m.Post("/change_password", bindIgnErr(auth.MustChangePasswordForm{}), user.MustChangePasswordPost)
		m.Post("/avatar", binding.MultipartForm(auth.AvatarForm{}), userSetting.AvatarPost)
		m.Post("/avatar/delete", userSetting.DeleteAvatar)
		m.Group("/account", func() {
			m.Combo("").Get(userSetting.Account).Post(bindIgnErr(auth.ChangePasswordForm{}), userSetting.AccountPost)
			m.Post("/email", bindIgnErr(auth.AddEmailForm{}), userSetting.EmailPost)
			m.Post("/email/delete", userSetting.DeleteEmail)
			m.Post("/delete", userSetting.DeleteAccount)
			m.Post("/theme", bindIgnErr(auth.UpdateThemeForm{}), userSetting.UpdateUIThemePost)
		})
		m.Group("/security", func() {
			m.Get("", userSetting.Security)
			m.Group("/two_factor", func() {
				m.Post("/regenerate_scratch", userSetting.RegenerateScratchTwoFactor)
				m.Post("/disable", userSetting.DisableTwoFactor)
				m.Get("/enroll", userSetting.EnrollTwoFactor)
				m.Post("/enroll", bindIgnErr(auth.TwoFactorAuthForm{}), userSetting.EnrollTwoFactorPost)
			})
			m.Group("/u2f", func() {
				m.Post("/request_register", bindIgnErr(auth.U2FRegistrationForm{}), userSetting.U2FRegister)
				m.Post("/register", bindIgnErr(u2f.RegisterResponse{}), userSetting.U2FRegisterPost)
				m.Post("/delete", bindIgnErr(auth.U2FDeleteForm{}), userSetting.U2FDelete)
			})
			m.Group("/openid", func() {
				m.Post("", bindIgnErr(auth.AddOpenIDForm{}), userSetting.OpenIDPost)
				m.Post("/delete", userSetting.DeleteOpenID)
				m.Post("/toggle_visibility", userSetting.ToggleOpenIDVisibility)
			}, openIDSignInEnabled)
			m.Post("/account_link", userSetting.DeleteAccountLink)
		})
		m.Combo("/applications").Get(userSetting.Applications).
			Post(bindIgnErr(auth.NewAccessTokenForm{}), userSetting.ApplicationsPost)
		m.Post("/applications/delete", userSetting.DeleteApplication)
		m.Combo("/keys").Get(userSetting.Keys).
			Post(bindIgnErr(auth.AddKeyForm{}), userSetting.KeysPost)
		m.Post("/keys/delete", userSetting.DeleteKey)
		m.Get("/organization", userSetting.Organization)
		m.Get("/repos", userSetting.Repos)

		// redirects from old settings urls to new ones
		// TODO: can be removed on next major version
		m.Get("/avatar", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL+"/user/settings", http.StatusMovedPermanently)
		})
		m.Get("/email", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL+"/user/settings/account", http.StatusMovedPermanently)
		})
		m.Get("/delete", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL+"/user/settings/account", http.StatusMovedPermanently)
		})
		m.Get("/openid", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL+"/user/settings/security", http.StatusMovedPermanently)
		})
		m.Get("/account_link", func(ctx *context.Context) {
			ctx.Redirect(setting.AppSubURL+"/user/settings/security", http.StatusMovedPermanently)
		})
	}, reqSignIn, func(ctx *context.Context) {
		ctx.Data["PageIsUserSettings"] = true
		ctx.Data["AllThemes"] = setting.UI.Themes
	})

	m.Group("/user", func() {
		// r.Get("/feeds", binding.Bind(auth.FeedsForm{}), user.Feeds)
		m.Any("/activate", user.Activate)
		m.Any("/activate_email", user.ActivateEmail)
		m.Get("/email2user", user.Email2User)
		m.Get("/forgot_password", user.ForgotPasswd)
		m.Post("/forgot_password", user.ForgotPasswdPost)
		m.Get("/logout", user.SignOut)
	})
	// ***** END: User *****

	adminReq := context.Toggle(&context.ToggleOptions{SignInRequired: true, AdminRequired: true})

	// ***** START: Admin *****
	m.Group("/admin", func() {
		m.Get("", adminReq, admin.Dashboard)
		m.Get("/config", admin.Config)
		m.Post("/config/test_mail", admin.SendTestMail)
		m.Get("/monitor", admin.Monitor)

		m.Group("/users", func() {
			m.Get("", admin.Users)
			m.Combo("/new").Get(admin.NewUser).Post(bindIgnErr(auth.AdminCreateUserForm{}), admin.NewUserPost)
			m.Combo("/:userid").Get(admin.EditUser).Post(bindIgnErr(auth.AdminEditUserForm{}), admin.EditUserPost)
			m.Post("/:userid/delete", admin.DeleteUser)
		})

		m.Group("/orgs", func() {
			m.Get("", admin.Organizations)
		})

		m.Group("/repos", func() {
			m.Get("", admin.Repos)
			m.Post("/delete", admin.DeleteRepo)
		})

		m.Group("/auths", func() {
			m.Get("", admin.Authentications)
			m.Combo("/new").Get(admin.NewAuthSource).Post(bindIgnErr(auth.AuthenticationForm{}), admin.NewAuthSourcePost)
			m.Combo("/:authid").Get(admin.EditAuthSource).
				Post(bindIgnErr(auth.AuthenticationForm{}), admin.EditAuthSourcePost)
			m.Post("/:authid/delete", admin.DeleteAuthSource)
		})

		m.Group("/notices", func() {
			m.Get("", admin.Notices)
			m.Post("/delete", admin.DeleteNotices)
			m.Get("/empty", admin.EmptyNotices)
		})
	}, adminReq)
	// ***** END: Admin *****

	m.Group("", func() {
		m.Group("/:username", func() {
			m.Get("", user.Profile)
			m.Get("/followers", user.Followers)
			m.Get("/following", user.Following)
		})

		m.Get("/attachments/:uuid", func(ctx *context.Context) {
			attach, err := models.GetAttachmentByUUID(ctx.Params(":uuid"))
			if err != nil {
				if models.IsErrAttachmentNotExist(err) {
					ctx.Error(404)
				} else {
					ctx.ServerError("GetAttachmentByUUID", err)
				}
				return
			}

			fr, err := os.Open(attach.LocalPath())
			if err != nil {
				ctx.ServerError("Open", err)
				return
			}
			defer fr.Close()

			if err := attach.IncreaseDownloadCount(); err != nil {
				ctx.ServerError("Update", err)
				return
			}

			if err = repo.ServeData(ctx, attach.Name, fr); err != nil {
				ctx.ServerError("ServeData", err)
				return
			}
		})
		m.Post("/attachments", repo.UploadAttachment)
	}, ignSignIn)

	m.Group("/:username", func() {
		m.Get("/action/:action", user.Action)
	}, reqSignIn)

	if macaron.Env == macaron.DEV {
		m.Get("/template/*", dev.TemplatePreview)
	}

	reqRepoAdmin := context.RequireRepoAdmin()
	reqRepoCodeWriter := context.RequireRepoWriter(models.UnitTypeCode)
	reqRepoCodeReader := context.RequireRepoReader(models.UnitTypeCode)
	reqRepoReleaseWriter := context.RequireRepoWriter(models.UnitTypeReleases)
	reqRepoReleaseReader := context.RequireRepoReader(models.UnitTypeReleases)
	reqRepoWikiWriter := context.RequireRepoWriter(models.UnitTypeWiki)
	reqRepoIssueReader := context.RequireRepoReader(models.UnitTypeIssues)
	reqRepoPullsWriter := context.RequireRepoWriter(models.UnitTypePullRequests)
	reqRepoPullsReader := context.RequireRepoReader(models.UnitTypePullRequests)
	reqRepoIssuesOrPullsWriter := context.RequireRepoWriterOr(models.UnitTypeIssues, models.UnitTypePullRequests)
	reqRepoIssuesOrPullsReader := context.RequireRepoReaderOr(models.UnitTypeIssues, models.UnitTypePullRequests)

	// ***** START: Organization *****
	m.Group("/org", func() {
		m.Group("", func() {
			m.Get("/create", org.Create)
			m.Post("/create", bindIgnErr(auth.CreateOrgForm{}), org.CreatePost)
		})

		m.Group("/:org", func() {
			m.Get("/dashboard", user.Dashboard)
			m.Get("/^:type(issues|pulls)$", user.Issues)
			m.Get("/members", org.Members)
			m.Get("/members/action/:action", org.MembersAction)

			m.Get("/teams", org.Teams)
		}, context.OrgAssignment(true))

		m.Group("/:org", func() {
			m.Get("/teams/:team", org.TeamMembers)
			m.Get("/teams/:team/repositories", org.TeamRepositories)
			m.Route("/teams/:team/action/:action", "GET,POST", org.TeamsAction)
			m.Route("/teams/:team/action/repo/:action", "GET,POST", org.TeamsRepoAction)
		}, context.OrgAssignment(true, false, true))

		m.Group("/:org", func() {
			m.Get("/teams/new", org.NewTeam)
			m.Post("/teams/new", bindIgnErr(auth.CreateTeamForm{}), org.NewTeamPost)
			m.Get("/teams/:team/edit", org.EditTeam)
			m.Post("/teams/:team/edit", bindIgnErr(auth.CreateTeamForm{}), org.EditTeamPost)
			m.Post("/teams/:team/delete", org.DeleteTeam)

			m.Group("/settings", func() {
				m.Combo("").Get(org.Settings).
					Post(bindIgnErr(auth.UpdateOrgSettingForm{}), org.SettingsPost)
				m.Post("/avatar", binding.MultipartForm(auth.AvatarForm{}), org.SettingsAvatar)
				m.Post("/avatar/delete", org.SettingsDeleteAvatar)

				m.Group("/hooks", func() {
					m.Get("", org.Webhooks)
					m.Post("/delete", org.DeleteWebhook)
					m.Get("/:type/new", repo.WebhooksNew)
					m.Post("/gitea/new", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksNewPost)
					m.Post("/gogs/new", bindIgnErr(auth.NewGogshookForm{}), repo.GogsHooksNewPost)
					m.Post("/slack/new", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksNewPost)
					m.Post("/discord/new", bindIgnErr(auth.NewDiscordHookForm{}), repo.DiscordHooksNewPost)
					m.Post("/dingtalk/new", bindIgnErr(auth.NewDingtalkHookForm{}), repo.DingtalkHooksNewPost)
					m.Get("/:id", repo.WebHooksEdit)
					m.Post("/gitea/:id", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksEditPost)
					m.Post("/gogs/:id", bindIgnErr(auth.NewGogshookForm{}), repo.GogsHooksEditPost)
					m.Post("/slack/:id", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksEditPost)
					m.Post("/discord/:id", bindIgnErr(auth.NewDiscordHookForm{}), repo.DiscordHooksEditPost)
					m.Post("/dingtalk/:id", bindIgnErr(auth.NewDingtalkHookForm{}), repo.DingtalkHooksEditPost)
				})

				m.Route("/delete", "GET,POST", org.SettingsDelete)
			})
		}, context.OrgAssignment(true, true))
	}, reqSignIn)
	// ***** END: Organization *****

	// ***** START: Repository *****
	m.Group("/repo", func() {
		m.Get("/create", repo.Create)
		m.Post("/create", bindIgnErr(auth.CreateRepoForm{}), repo.CreatePost)
		m.Get("/migrate", repo.Migrate)
		m.Post("/migrate", bindIgnErr(auth.MigrateRepoForm{}), repo.MigratePost)
		m.Group("/fork", func() {
			m.Combo("/:repoid").Get(repo.Fork).
				Post(bindIgnErr(auth.CreateRepoForm{}), repo.ForkPost)
		}, context.RepoIDAssignment(), context.UnitTypes(), reqRepoCodeReader)
	}, reqSignIn)

	// ***** Release Attachment Download without Signin
	m.Get("/:username/:reponame/releases/download/:vTag/:fileName", ignSignIn, context.RepoAssignment(), repo.MustBeNotEmpty, repo.RedirectDownload)

	m.Group("/:username/:reponame", func() {
		m.Group("/settings", func() {
			m.Combo("").Get(repo.Settings).
				Post(bindIgnErr(auth.RepoSettingForm{}), repo.SettingsPost)
			m.Group("/collaboration", func() {
				m.Combo("").Get(repo.Collaboration).Post(repo.CollaborationPost)
				m.Post("/access_mode", repo.ChangeCollaborationAccessMode)
				m.Post("/delete", repo.DeleteCollaboration)
			})
			m.Group("/branches", func() {
				m.Combo("").Get(repo.ProtectedBranch).Post(repo.ProtectedBranchPost)
				m.Combo("/*").Get(repo.SettingsProtectedBranch).
					Post(bindIgnErr(auth.ProtectBranchForm{}), context.RepoMustNotBeArchived(), repo.SettingsProtectedBranchPost)
			}, repo.MustBeNotEmpty)

			m.Group("/hooks", func() {
				m.Get("", repo.Webhooks)
				m.Post("/delete", repo.DeleteWebhook)
				m.Get("/:type/new", repo.WebhooksNew)
				m.Post("/gitea/new", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksNewPost)
				m.Post("/gogs/new", bindIgnErr(auth.NewGogshookForm{}), repo.GogsHooksNewPost)
				m.Post("/slack/new", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksNewPost)
				m.Post("/discord/new", bindIgnErr(auth.NewDiscordHookForm{}), repo.DiscordHooksNewPost)
				m.Post("/dingtalk/new", bindIgnErr(auth.NewDingtalkHookForm{}), repo.DingtalkHooksNewPost)
				m.Get("/:id", repo.WebHooksEdit)
				m.Post("/:id/test", repo.TestWebhook)
				m.Post("/gitea/:id", bindIgnErr(auth.NewWebhookForm{}), repo.WebHooksEditPost)
				m.Post("/gogs/:id", bindIgnErr(auth.NewGogshookForm{}), repo.GogsHooksEditPost)
				m.Post("/slack/:id", bindIgnErr(auth.NewSlackHookForm{}), repo.SlackHooksEditPost)
				m.Post("/discord/:id", bindIgnErr(auth.NewDiscordHookForm{}), repo.DiscordHooksEditPost)
				m.Post("/dingtalk/:id", bindIgnErr(auth.NewDingtalkHookForm{}), repo.DingtalkHooksEditPost)

				m.Group("/git", func() {
					m.Get("", repo.GitHooks)
					m.Combo("/:name").Get(repo.GitHooksEdit).
						Post(repo.GitHooksEditPost)
				}, context.GitHookService())
			})

			m.Group("/keys", func() {
				m.Combo("").Get(repo.DeployKeys).
					Post(bindIgnErr(auth.AddKeyForm{}), repo.DeployKeysPost)
				m.Post("/delete", repo.DeleteDeployKey)
			})

		}, func(ctx *context.Context) {
			ctx.Data["PageIsSettings"] = true
		})
	}, reqSignIn, context.RepoAssignment(), reqRepoAdmin, context.UnitTypes(), context.RepoRef())

	m.Get("/:username/:reponame/action/:action", reqSignIn, context.RepoAssignment(), context.RepoMustNotBeArchived(), repo.Action)

	m.Group("/:username/:reponame", func() {
		m.Group("/issues", func() {
			m.Combo("/new").Get(context.RepoRef(), repo.NewIssue).
				Post(bindIgnErr(auth.CreateIssueForm{}), repo.NewIssuePost)
		}, context.RepoMustNotBeArchived(), reqRepoIssueReader)
		// FIXME: should use different URLs but mostly same logic for comments of issue and pull reuqest.
		// So they can apply their own enable/disable logic on routers.
		m.Group("/issues", func() {
			m.Group("/:index", func() {
				m.Post("/title", repo.UpdateIssueTitle)
				m.Post("/content", repo.UpdateIssueContent)
				m.Post("/watch", repo.IssueWatch)
				m.Group("/dependency", func() {
					m.Post("/add", repo.AddDependency)
					m.Post("/delete", repo.RemoveDependency)
				})
				m.Combo("/comments").Post(bindIgnErr(auth.CreateCommentForm{}), repo.NewComment)
				m.Group("/times", func() {
					m.Post("/add", bindIgnErr(auth.AddTimeManuallyForm{}), repo.AddTimeManually)
					m.Group("/stopwatch", func() {
						m.Post("/toggle", repo.IssueStopwatch)
						m.Post("/cancel", repo.CancelStopwatch)
					})
				})
				m.Post("/reactions/:action", bindIgnErr(auth.ReactionForm{}), repo.ChangeIssueReaction)

				m.Patch("/pin", reqRepoWriter, repo.PinIssue)
				m.Patch("/unpin", reqRepoWriter, repo.UnpinIssue)
			}, context.RepoMustNotBeArchived())

			m.Post("/labels", reqRepoIssuesOrPullsWriter, repo.UpdateIssueLabel)
			m.Post("/milestone", reqRepoIssuesOrPullsWriter, repo.UpdateIssueMilestone)
			m.Post("/assignee", reqRepoIssuesOrPullsWriter, repo.UpdateIssueAssignee)
			m.Post("/status", reqRepoIssuesOrPullsWriter, repo.UpdateIssueStatus)
		}, context.RepoMustNotBeArchived())
		m.Group("/comments/:id", func() {
			m.Post("", repo.UpdateCommentContent)
			m.Post("/delete", repo.DeleteComment)
			m.Post("/reactions/:action", bindIgnErr(auth.ReactionForm{}), repo.ChangeCommentReaction)
		}, context.RepoMustNotBeArchived())
		m.Group("/labels", func() {
			m.Post("/new", bindIgnErr(auth.CreateLabelForm{}), repo.NewLabel)
			m.Post("/edit", bindIgnErr(auth.CreateLabelForm{}), repo.UpdateLabel)
			m.Post("/delete", repo.DeleteLabel)
			m.Post("/initialize", bindIgnErr(auth.InitializeLabelsForm{}), repo.InitializeLabels)
		}, context.RepoMustNotBeArchived(), reqRepoIssuesOrPullsWriter, context.RepoRef())
		m.Group("/milestones", func() {
			m.Combo("/new").Get(repo.NewMilestone).
				Post(bindIgnErr(auth.CreateMilestoneForm{}), repo.NewMilestonePost)
			m.Get("/:id/edit", repo.EditMilestone)
			m.Post("/:id/edit", bindIgnErr(auth.CreateMilestoneForm{}), repo.EditMilestonePost)
			m.Get("/:id/:action", repo.ChangeMilestonStatus)
			m.Post("/delete", repo.DeleteMilestone)
		}, context.RepoMustNotBeArchived(), reqRepoIssuesOrPullsWriter, context.RepoRef())
		m.Group("/milestone", func() {
			m.Get("/:id", repo.MilestoneIssuesAndPulls)
		}, reqRepoIssuesOrPullsWriter, context.RepoRef())
		m.Combo("/compare/*", reqRepoCodeReader, reqRepoPullsReader, repo.MustAllowPulls, repo.SetEditorconfigIfExists).
			Get(repo.SetDiffViewStyle, repo.CompareAndPullRequest).
			Post(bindIgnErr(auth.CreateIssueForm{}), repo.CompareAndPullRequestPost)

		m.Group("", func() {
			m.Group("", func() {
				m.Combo("/_edit/*").Get(repo.EditFile).
					Post(bindIgnErr(auth.EditRepoFileForm{}), repo.EditFilePost)
				m.Combo("/_new/*").Get(repo.NewFile).
					Post(bindIgnErr(auth.EditRepoFileForm{}), repo.NewFilePost)
				m.Post("/_preview/*", bindIgnErr(auth.EditPreviewDiffForm{}), repo.DiffPreviewPost)
				m.Combo("/_delete/*").Get(repo.DeleteFile).
					Post(bindIgnErr(auth.DeleteRepoFileForm{}), repo.DeleteFilePost)
				m.Combo("/_upload/*", repo.MustBeAbleToUpload).
					Get(repo.UploadFile).
					Post(bindIgnErr(auth.UploadRepoFileForm{}), repo.UploadFilePost)
			}, context.RepoRefByType(context.RepoRefBranch), repo.MustBeEditable)
			m.Group("", func() {
				m.Post("/upload-file", repo.UploadFileToServer)
				m.Post("/upload-remove", bindIgnErr(auth.RemoveUploadFileForm{}), repo.RemoveUploadFileFromServer)
			}, context.RepoRef(), repo.MustBeEditable, repo.MustBeAbleToUpload)
		}, context.RepoMustNotBeArchived(), reqRepoCodeWriter, repo.MustBeNotEmpty)

		m.Group("/branches", func() {
			m.Group("/_new/", func() {
				m.Post("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.CreateBranch)
				m.Post("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.CreateBranch)
				m.Post("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.CreateBranch)
			}, bindIgnErr(auth.NewBranchForm{}))
			m.Post("/delete", repo.DeleteBranchPost)
			m.Post("/restore", repo.RestoreBranchPost)
		}, context.RepoMustNotBeArchived(), reqRepoCodeWriter, repo.MustBeNotEmpty)

	}, reqSignIn, context.RepoAssignment(), context.UnitTypes())

	// Releases
	m.Group("/:username/:reponame", func() {
		m.Group("/releases", func() {
			m.Get("/", repo.MustBeNotEmpty, repo.Releases)
		}, repo.MustBeNotEmpty, context.RepoRef())
		m.Group("/releases", func() {
			m.Get("/new", repo.NewRelease)
			m.Post("/new", bindIgnErr(auth.NewReleaseForm{}), repo.NewReleasePost)
			m.Post("/delete", repo.DeleteRelease)
		}, reqSignIn, repo.MustBeNotEmpty, context.RepoMustNotBeArchived(), reqRepoReleaseWriter, context.RepoRef())
		m.Group("/releases", func() {
			m.Get("/edit/*", repo.EditRelease)
			m.Post("/edit/*", bindIgnErr(auth.EditReleaseForm{}), repo.EditReleasePost)
		}, reqSignIn, repo.MustBeNotEmpty, context.RepoMustNotBeArchived(), reqRepoReleaseWriter, func(ctx *context.Context) {
			var err error
			ctx.Repo.Commit, err = ctx.Repo.GitRepo.GetBranchCommit(ctx.Repo.Repository.DefaultBranch)
			if err != nil {
				ctx.ServerError("GetBranchCommit", err)
				return
			}
			ctx.Repo.CommitsCount, err = ctx.Repo.GetCommitsCount()
			if err != nil {
				ctx.ServerError("GetCommitsCount", err)
				return
			}
			ctx.Data["CommitsCount"] = ctx.Repo.CommitsCount
		})
	}, context.RepoAssignment(), context.UnitTypes(), reqRepoReleaseReader)

	m.Group("/:username/:reponame", func() {
		m.Post("/topics", repo.TopicsPost)
	}, context.RepoAssignment(), context.RepoMustNotBeArchived(), reqRepoAdmin)

	m.Group("/:username/:reponame", func() {
		m.Group("", func() {
			m.Get("/^:type(issues|pulls)$", repo.Issues)
			m.Get("/^:type(issues|pulls)$/:index", repo.ViewIssue)
			m.Get("/labels/", reqRepoIssuesOrPullsReader, repo.RetrieveLabels, repo.Labels)
			m.Get("/milestones", reqRepoIssuesOrPullsReader, repo.Milestones)
		}, context.RepoRef())

		m.Group("/wiki", func() {
			m.Get("/?:page", repo.Wiki)
			m.Get("/_pages", repo.WikiPages)

			m.Group("", func() {
				m.Combo("/_new").Get(repo.NewWiki).
					Post(bindIgnErr(auth.NewWikiForm{}), repo.NewWikiPost)
				m.Combo("/:page/_edit").Get(repo.EditWiki).
					Post(bindIgnErr(auth.NewWikiForm{}), repo.EditWikiPost)
				m.Post("/:page/delete", repo.DeleteWikiPagePost)
			}, context.RepoMustNotBeArchived(), reqSignIn, reqRepoWikiWriter)
		}, repo.MustEnableWiki, context.RepoRef())

		m.Group("/wiki", func() {
			m.Get("/raw/*", repo.WikiRaw)
		}, repo.MustEnableWiki)

		m.Group("/activity", func() {
			m.Get("", repo.Activity)
			m.Get("/:period", repo.Activity)
		}, context.RepoRef(), repo.MustBeNotEmpty, context.RequireRepoReaderOr(models.UnitTypePullRequests, models.UnitTypeIssues, models.UnitTypeReleases))

		m.Get("/archive/*", repo.MustBeNotEmpty, reqRepoCodeReader, repo.Download)

		m.Group("/branches", func() {
			m.Get("", repo.Branches)
		}, repo.MustBeNotEmpty, context.RepoRef(), reqRepoCodeReader)

		m.Group("/pulls/:index", func() {
			m.Get(".diff", repo.DownloadPullDiff)
			m.Get(".patch", repo.DownloadPullPatch)
			m.Get("/commits", context.RepoRef(), repo.ViewPullCommits)
			m.Post("/merge", context.RepoMustNotBeArchived(), reqRepoPullsWriter, bindIgnErr(auth.MergePullRequestForm{}), repo.MergePullRequest)
			m.Post("/cleanup", context.RepoMustNotBeArchived(), context.RepoRef(), repo.CleanUpPullRequest)
			m.Group("/files", func() {
				m.Get("", context.RepoRef(), repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.SetWhitespaceBehavior, repo.ViewPullFiles)
				m.Group("/reviews", func() {
					m.Post("/comments", bindIgnErr(auth.CodeCommentForm{}), repo.CreateCodeComment)
					m.Post("/submit", bindIgnErr(auth.SubmitReviewForm{}), repo.SubmitReview)
				}, context.RepoMustNotBeArchived())
			})
		}, repo.MustAllowPulls)

		m.Group("/raw", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.SingleDownload)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.SingleDownload)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.SingleDownload)
			m.Get("/blob/:sha", context.RepoRefByType(context.RepoRefBlob), repo.DownloadByID)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.SingleDownload)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("/commits", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.RefCommits)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.RefCommits)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.RefCommits)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.RefCommits)
		}, repo.MustBeNotEmpty, reqRepoCodeReader)

		m.Group("", func() {
			m.Get("/graph", repo.Graph)
			m.Get("/commit/:sha([a-f0-9]{7,40})$", repo.SetEditorconfigIfExists, repo.SetDiffViewStyle, repo.Diff)
		}, repo.MustBeNotEmpty, context.RepoRef(), reqRepoCodeReader)

		m.Group("/src", func() {
			m.Get("/branch/*", context.RepoRefByType(context.RepoRefBranch), repo.Home)
			m.Get("/tag/*", context.RepoRefByType(context.RepoRefTag), repo.Home)
			m.Get("/commit/*", context.RepoRefByType(context.RepoRefCommit), repo.Home)
			// "/*" route is deprecated, and kept for backward compatibility
			m.Get("/*", context.RepoRefByType(context.RepoRefLegacy), repo.Home)
		}, repo.SetEditorconfigIfExists)

		m.Group("", func() {
			m.Get("/forks", repo.Forks)
		}, context.RepoRef(), reqRepoCodeReader)
		m.Get("/commit/:sha([a-f0-9]{7,40})\\.:ext(patch|diff)",
			repo.MustBeNotEmpty, reqRepoCodeReader, repo.RawDiff)

		m.Get("/compare/:before([a-z0-9]{40})\\.\\.\\.:after([a-z0-9]{40})", repo.SetEditorconfigIfExists,
			repo.SetDiffViewStyle, repo.MustBeNotEmpty, reqRepoCodeReader, repo.CompareDiff)
	}, ignSignIn, context.RepoAssignment(), context.UnitTypes())
	m.Group("/:username/:reponame", func() {
		m.Get("/stars", repo.Stars)
		m.Get("/watchers", repo.Watchers)
		m.Get("/search", reqRepoCodeReader, repo.Search)
	}, ignSignIn, context.RepoAssignment(), context.RepoRef(), context.UnitTypes())

	m.Group("/:username", func() {
		m.Group("/:reponame", func() {
			m.Get("", repo.SetEditorconfigIfExists, repo.Home)
			m.Get("\\.git$", repo.SetEditorconfigIfExists, repo.Home)
		}, ignSignIn, context.RepoAssignment(), context.RepoRef(), context.UnitTypes())

		m.Group("/:reponame", func() {
			m.Group("\\.git/info/lfs", func() {
				m.Post("/objects/batch", lfs.BatchHandler)
				m.Get("/objects/:oid/:filename", lfs.ObjectOidHandler)
				m.Any("/objects/:oid", lfs.ObjectOidHandler)
				m.Post("/objects", lfs.PostHandler)
				m.Post("/verify", lfs.VerifyHandler)
				m.Group("/locks", func() {
					m.Get("/", lfs.GetListLockHandler)
					m.Post("/", lfs.PostLockHandler)
					m.Post("/verify", lfs.VerifyLockHandler)
					m.Post("/:lid/unlock", lfs.UnLockHandler)
				}, context.RepoAssignment())
				m.Any("/*", func(ctx *context.Context) {
					ctx.NotFound("", nil)
				})
			}, ignSignInAndCsrf)
			m.Any("/*", ignSignInAndCsrf, repo.HTTP)
			m.Head("/tasks/trigger", repo.TriggerTask)
		})
	})
	// ***** END: Repository *****

	m.Group("/notifications", func() {
		m.Get("", user.Notifications)
		m.Post("/status", user.NotificationStatusPost)
		m.Post("/purge", user.NotificationPurgePost)
	}, reqSignIn)

	if setting.API.EnableSwagger {
		m.Get("/swagger.v1.json", templates.JSONRenderer(), routers.SwaggerV1Json)
	}

	m.Group("/api", func() {
		apiv1.RegisterRoutes(m)
	}, ignSignIn)

	m.Group("/api/internal", func() {
		// package name internal is ideal but Golang is not allowed, so we use private as package name.
		private.RegisterRoutes(m)
	})

	// robots.txt
	m.Get("/robots.txt", func(ctx *context.Context) {
		if setting.HasRobotsTxt {
			ctx.ServeFileContent(path.Join(setting.CustomPath, "robots.txt"))
		} else {
			ctx.NotFound("", nil)
		}
	})

	// Progressive Web App
	m.Get("/manifest.json", templates.JSONRenderer(), func(ctx *context.Context) {
		ctx.HTML(200, "pwa/manifest_json")
	})

	m.Get("/serviceworker.js", templates.JSRenderer(), func(ctx *context.Context) {
		ctx.HTML(200, "pwa/serviceworker_js")
	})

	// prometheus metrics endpoint
	if setting.Metrics.Enabled {
		c := metrics.NewCollector()
		prometheus.MustRegister(c)

		m.Get("/metrics", routers.Metrics)
	}

	// Not found handler.
	m.NotFound(routers.NotFound)
}
