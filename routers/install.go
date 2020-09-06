// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routers

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/user"

	"github.com/unknwon/com"
	"gopkg.in/ini.v1"
)

const (
	// tplInstall template for installation page
	tplInstall base.TplName = "install"
)

// InstallInit prepare for rendering installation page
func InstallInit(ctx *context.Context) {
	if setting.InstallLock {
		ctx.NotFound("Install", errors.New("Installation is prohibited"))
		return
	}

	ctx.Data["Title"] = ctx.Tr("install.install")
	ctx.Data["PageIsInstall"] = true

	ctx.Data["DbOptions"] = setting.SupportedDatabases
}

// Install render installation page
func Install(ctx *context.Context) {
	form := auth.InstallForm{}

	// Database settings
	form.DbHost = setting.Database.Host
	form.DbUser = setting.Database.User
	form.DbPasswd = setting.Database.Passwd
	form.DbName = setting.Database.Name
	form.DbPath = setting.Database.Path
	form.DbSchema = setting.Database.Schema
	form.Charset = setting.Database.Charset

	ctx.Data["CurDbOption"] = "MySQL"
	switch setting.Database.Type {
	case "postgres":
		ctx.Data["CurDbOption"] = "PostgreSQL"
	case "mssql":
		ctx.Data["CurDbOption"] = "MSSQL"
	case "sqlite3":
		if setting.EnableSQLite3 {
			ctx.Data["CurDbOption"] = "SQLite3"
		}
	}

	// Application general settings
	form.AppName = setting.AppName
	form.RepoRootPath = setting.RepoRootPath
	form.LFSRootPath = setting.LFS.ContentPath

	// Note(unknown): it's hard for Windows users change a running user,
	// 	so just use current one if config says default.
	if setting.IsWindows && setting.RunUser == "git" {
		form.RunUser = user.CurrentUsername()
	} else {
		form.RunUser = setting.RunUser
	}

	form.Domain = setting.Domain
	form.SSHPort = setting.SSH.Port
	form.HTTPPort = setting.HTTPPort
	form.AppURL = setting.AppURL
	form.LogRootPath = setting.LogRootPath

	// E-mail service settings
	if setting.MailService != nil {
		form.SMTPHost = setting.MailService.Host
		form.SMTPFrom = setting.MailService.From
		form.SMTPUser = setting.MailService.User
	}
	form.RegisterConfirm = setting.Service.RegisterEmailConfirm
	form.MailNotify = setting.Service.EnableNotifyMail

	// Server and other services settings
	form.OfflineMode = setting.OfflineMode
	form.DisableGravatar = setting.DisableGravatar
	form.EnableFederatedAvatar = setting.EnableFederatedAvatar
	form.EnableOpenIDSignIn = setting.Service.EnableOpenIDSignIn
	form.EnableOpenIDSignUp = setting.Service.EnableOpenIDSignUp
	form.DisableRegistration = setting.Service.DisableRegistration
	form.AllowOnlyExternalRegistration = setting.Service.AllowOnlyExternalRegistration
	form.EnableCaptcha = setting.Service.EnableCaptcha
	form.RequireSignInView = setting.Service.RequireSignInView
	form.DefaultKeepEmailPrivate = setting.Service.DefaultKeepEmailPrivate
	form.DefaultAllowCreateOrganization = setting.Service.DefaultAllowCreateOrganization
	form.DefaultEnableTimetracking = setting.Service.DefaultEnableTimetracking
	form.NoReplyAddress = setting.Service.NoReplyAddress

	auth.AssignForm(form, ctx.Data)
	ctx.HTML(200, tplInstall)
}

// InstallPost response for submit install items
func InstallPost(ctx *context.Context, form auth.InstallForm) {
	var err error
	ctx.Data["CurDbOption"] = form.DbType

	if ctx.HasError() {
		if ctx.HasValue("Err_SMTPUser") {
			ctx.Data["Err_SMTP"] = true
		}
		if ctx.HasValue("Err_AdminName") ||
			ctx.HasValue("Err_AdminPasswd") ||
			ctx.HasValue("Err_AdminEmail") {
			ctx.Data["Err_Admin"] = true
		}

		ctx.HTML(200, tplInstall)
		return
	}

	if _, err = exec.LookPath("git"); err != nil {
		ctx.RenderWithErr(ctx.Tr("install.test_git_failed", err), tplInstall, &form)
		return
	}

	// Pass basic check, now test configuration.
	// Test database setting.

	setting.Database.Type = setting.GetDBTypeByName(form.DbType)
	setting.Database.Host = form.DbHost
	setting.Database.User = form.DbUser
	setting.Database.Passwd = form.DbPasswd
	setting.Database.Name = form.DbName
	setting.Database.Schema = form.DbSchema
	setting.Database.SSLMode = form.SSLMode
	setting.Database.Charset = form.Charset
	setting.Database.Path = form.DbPath
	setting.Database.TableNamePrefix = form.DbTableNamePrefix

	if (setting.Database.Type == "sqlite3") &&
		len(setting.Database.Path) == 0 {
		ctx.Data["Err_DbPath"] = true
		ctx.RenderWithErr(ctx.Tr("install.err_empty_db_path"), tplInstall, &form)
		return
	}

	// Set test engine.
	if err = models.NewTestEngine(); err != nil {
		if strings.Contains(err.Error(), `Unknown database type: sqlite3`) {
			ctx.Data["Err_DbType"] = true
			ctx.RenderWithErr(ctx.Tr("install.sqlite3_not_available", "https://docs.gitea.io/en-us/install-from-binary/"), tplInstall, &form)
		} else {
			ctx.Data["Err_DbSetting"] = true
			ctx.RenderWithErr(ctx.Tr("install.invalid_db_setting", err), tplInstall, &form)
		}
		return
	}

	// Test repository root path.
	form.RepoRootPath = strings.Replace(form.RepoRootPath, "\\", "/", -1)
	if err = os.MkdirAll(form.RepoRootPath, os.ModePerm); err != nil {
		ctx.Data["Err_RepoRootPath"] = true
		ctx.RenderWithErr(ctx.Tr("install.invalid_repo_path", err), tplInstall, &form)
		return
	}

	// Test LFS root path if not empty, empty meaning disable LFS
	if form.LFSRootPath != "" {
		form.LFSRootPath = strings.Replace(form.LFSRootPath, "\\", "/", -1)
		if err := os.MkdirAll(form.LFSRootPath, os.ModePerm); err != nil {
			ctx.Data["Err_LFSRootPath"] = true
			ctx.RenderWithErr(ctx.Tr("install.invalid_lfs_path", err), tplInstall, &form)
			return
		}
	}

	// Test log root path.
	form.LogRootPath = strings.Replace(form.LogRootPath, "\\", "/", -1)
	if err = os.MkdirAll(form.LogRootPath, os.ModePerm); err != nil {
		ctx.Data["Err_LogRootPath"] = true
		ctx.RenderWithErr(ctx.Tr("install.invalid_log_root_path", err), tplInstall, &form)
		return
	}

	currentUser, match := setting.IsRunUserMatchCurrentUser(form.RunUser)
	if !match {
		ctx.Data["Err_RunUser"] = true
		ctx.RenderWithErr(ctx.Tr("install.run_user_not_match", form.RunUser, currentUser), tplInstall, &form)
		return
	}

	// Check logic loophole between disable self-registration and no admin account.
	if form.DisableRegistration && len(form.AdminName) == 0 {
		ctx.Data["Err_Services"] = true
		ctx.Data["Err_Admin"] = true
		ctx.RenderWithErr(ctx.Tr("install.no_admin_and_disable_registration"), tplInstall, form)
		return
	}

	// Check admin user creation
	if len(form.AdminName) > 0 {
		// Ensure AdminName is valid
		if err := models.IsUsableUsername(form.AdminName); err != nil {
			ctx.Data["Err_Admin"] = true
			ctx.Data["Err_AdminName"] = true
			if models.IsErrNameReserved(err) {
				ctx.RenderWithErr(ctx.Tr("install.err_admin_name_is_reserved"), tplInstall, form)
				return
			} else if models.IsErrNamePatternNotAllowed(err) {
				ctx.RenderWithErr(ctx.Tr("install.err_admin_name_pattern_not_allowed"), tplInstall, form)
				return
			}
			ctx.RenderWithErr(ctx.Tr("install.err_admin_name_is_invalid"), tplInstall, form)
			return
		}
		// Check Admin email
		if len(form.AdminEmail) == 0 {
			ctx.Data["Err_Admin"] = true
			ctx.Data["Err_AdminEmail"] = true
			ctx.RenderWithErr(ctx.Tr("install.err_empty_admin_email"), tplInstall, form)
			return
		}
		// Check admin password.
		if len(form.AdminPasswd) == 0 {
			ctx.Data["Err_Admin"] = true
			ctx.Data["Err_AdminPasswd"] = true
			ctx.RenderWithErr(ctx.Tr("install.err_empty_admin_password"), tplInstall, form)
			return
		}
		if form.AdminPasswd != form.AdminConfirmPasswd {
			ctx.Data["Err_Admin"] = true
			ctx.Data["Err_AdminPasswd"] = true
			ctx.RenderWithErr(ctx.Tr("form.password_not_match"), tplInstall, form)
			return
		}
	}

	if form.AppURL[len(form.AppURL)-1] != '/' {
		form.AppURL += "/"
	}

	// Save settings.
	cfg := ini.Empty()
	if com.IsFile(setting.CustomConf) {
		// Keeps custom settings if there is already something.
		if err = cfg.Append(setting.CustomConf); err != nil {
			log.Error("Failed to load custom conf '%s': %v", setting.CustomConf, err)
		}
	}
	cfg.Section("database").Key("DB_TYPE").SetValue(setting.Database.Type)
	cfg.Section("database").Key("HOST").SetValue(setting.Database.Host)
	cfg.Section("database").Key("NAME").SetValue(setting.Database.Name)
	cfg.Section("database").Key("USER").SetValue(setting.Database.User)
	cfg.Section("database").Key("PASSWD").SetValue(setting.Database.Passwd)
	cfg.Section("database").Key("SCHEMA").SetValue(setting.Database.Schema)
	cfg.Section("database").Key("SSL_MODE").SetValue(setting.Database.SSLMode)
	cfg.Section("database").Key("CHARSET").SetValue(setting.Database.Charset)
	cfg.Section("database").Key("PATH").SetValue(setting.Database.Path)
	cfg.Section("database").Key("DB_TABLE_NAME_PREFIX").SetValue(setting.Database.TableNamePrefix)

	cfg.Section("").Key("APP_NAME").SetValue(form.AppName)
	cfg.Section("repository").Key("ROOT").SetValue(form.RepoRootPath)
	cfg.Section("").Key("RUN_USER").SetValue(form.RunUser)
	cfg.Section("server").Key("SSH_DOMAIN").SetValue(form.Domain)
	cfg.Section("server").Key("DOMAIN").SetValue(form.Domain)
	cfg.Section("server").Key("HTTP_PORT").SetValue(form.HTTPPort)
	cfg.Section("server").Key("ROOT_URL").SetValue(form.AppURL)

	if form.SSHPort == 0 {
		cfg.Section("server").Key("DISABLE_SSH").SetValue("true")
	} else {
		cfg.Section("server").Key("DISABLE_SSH").SetValue("false")
		cfg.Section("server").Key("SSH_PORT").SetValue(com.ToStr(form.SSHPort))
	}

	if form.LFSRootPath != "" {
		cfg.Section("server").Key("LFS_START_SERVER").SetValue("true")
		cfg.Section("server").Key("LFS_CONTENT_PATH").SetValue(form.LFSRootPath)
		var secretKey string
		if secretKey, err = generate.NewJwtSecret(); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.lfs_jwt_secret_failed", err), tplInstall, &form)
			return
		}
		cfg.Section("server").Key("LFS_JWT_SECRET").SetValue(secretKey)
	} else {
		cfg.Section("server").Key("LFS_START_SERVER").SetValue("false")
	}

	if len(strings.TrimSpace(form.SMTPHost)) > 0 {
		cfg.Section("mailer").Key("ENABLED").SetValue("true")
		cfg.Section("mailer").Key("HOST").SetValue(form.SMTPHost)
		cfg.Section("mailer").Key("FROM").SetValue(form.SMTPFrom)
		cfg.Section("mailer").Key("USER").SetValue(form.SMTPUser)
		cfg.Section("mailer").Key("PASSWD").SetValue(form.SMTPPasswd)
	} else {
		cfg.Section("mailer").Key("ENABLED").SetValue("false")
	}
	cfg.Section("service").Key("REGISTER_EMAIL_CONFIRM").SetValue(com.ToStr(form.RegisterConfirm))
	cfg.Section("service").Key("ENABLE_NOTIFY_MAIL").SetValue(com.ToStr(form.MailNotify))

	cfg.Section("server").Key("OFFLINE_MODE").SetValue(com.ToStr(form.OfflineMode))
	cfg.Section("picture").Key("DISABLE_GRAVATAR").SetValue(com.ToStr(form.DisableGravatar))
	cfg.Section("picture").Key("ENABLE_FEDERATED_AVATAR").SetValue(com.ToStr(form.EnableFederatedAvatar))
	cfg.Section("openid").Key("ENABLE_OPENID_SIGNIN").SetValue(com.ToStr(form.EnableOpenIDSignIn))
	cfg.Section("openid").Key("ENABLE_OPENID_SIGNUP").SetValue(com.ToStr(form.EnableOpenIDSignUp))
	cfg.Section("service").Key("DISABLE_REGISTRATION").SetValue(com.ToStr(form.DisableRegistration))
	cfg.Section("service").Key("ALLOW_ONLY_EXTERNAL_REGISTRATION").SetValue(com.ToStr(form.AllowOnlyExternalRegistration))
	cfg.Section("service").Key("ENABLE_CAPTCHA").SetValue(com.ToStr(form.EnableCaptcha))
	cfg.Section("service").Key("REQUIRE_SIGNIN_VIEW").SetValue(com.ToStr(form.RequireSignInView))
	cfg.Section("service").Key("DEFAULT_KEEP_EMAIL_PRIVATE").SetValue(com.ToStr(form.DefaultKeepEmailPrivate))
	cfg.Section("service").Key("DEFAULT_ALLOW_CREATE_ORGANIZATION").SetValue(com.ToStr(form.DefaultAllowCreateOrganization))
	cfg.Section("service").Key("DEFAULT_ENABLE_TIMETRACKING").SetValue(com.ToStr(form.DefaultEnableTimetracking))
	cfg.Section("service").Key("NO_REPLY_ADDRESS").SetValue(com.ToStr(form.NoReplyAddress))

	cfg.Section("").Key("RUN_MODE").SetValue("prod")

	cfg.Section("session").Key("PROVIDER").SetValue("file")

	cfg.Section("log").Key("MODE").SetValue("file")
	cfg.Section("log").Key("LEVEL").SetValue(setting.LogLevel)
	cfg.Section("log").Key("ROOT_PATH").SetValue(form.LogRootPath)

	cfg.Section("security").Key("INSTALL_LOCK").SetValue("true")
	var secretKey string
	if secretKey, err = generate.NewSecretKey(); err != nil {
		ctx.RenderWithErr(ctx.Tr("install.secret_key_failed", err), tplInstall, &form)
		return
	}
	cfg.Section("security").Key("SECRET_KEY").SetValue(secretKey)

	err = os.MkdirAll(filepath.Dir(setting.CustomConf), os.ModePerm)
	if err != nil {
		ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
		return
	}

	if err = cfg.SaveTo(setting.CustomConf); err != nil {
		ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
		return
	}

	GlobalInit(graceful.GetManager().HammerContext())

	// Create admin account
	if len(form.AdminName) > 0 {
		u := &models.User{
			Name:     form.AdminName,
			Email:    form.AdminEmail,
			Passwd:   form.AdminPasswd,
			IsAdmin:  true,
			IsActive: true,
		}
		if err = models.CreateUser(u); err != nil {
			if !models.IsErrUserAlreadyExist(err) {
				setting.InstallLock = false
				ctx.Data["Err_AdminName"] = true
				ctx.Data["Err_AdminEmail"] = true
				ctx.RenderWithErr(ctx.Tr("install.invalid_admin_setting", err), tplInstall, &form)
				return
			}
			log.Info("Admin account already exist")
			u, _ = models.GetUserByName(u.Name)
		}

		// Auto-login for admin
		if err = ctx.Session.Set("uid", u.ID); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
			return
		}
		if err = ctx.Session.Set("uname", u.Name); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
			return
		}

		if err = ctx.Session.Release(); err != nil {
			ctx.RenderWithErr(ctx.Tr("install.save_config_failed", err), tplInstall, &form)
			return
		}
	}

	log.Info("First-time run install finished!")
	// FIXME: This isn't really enough to completely take account of new configuration
	// We should really be restarting:
	// - On windows this is probably just a simple restart
	// - On linux we can't just use graceful.RestartProcess() everything that was passed in on LISTEN_FDS
	//   (active or not) needs to be passed out and everything new passed out too.
	//   This means we need to prevent the cleanup goroutine from running prior to the second GlobalInit
	ctx.Flash.Success(ctx.Tr("install.install_success"))
	ctx.Redirect(form.AppURL + "user/login")
}
