// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routers

import (
	"context"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/cron"
	"code.gitea.io/gitea/modules/eventsource"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/highlight"
	code_indexer "code.gitea.io/gitea/modules/indexer/code"
	issue_indexer "code.gitea.io/gitea/modules/indexer/issues"
	stats_indexer "code.gitea.io/gitea/modules/indexer/stats"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/external"
	repo_migrations "code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/notification"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/ssh"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/task"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/modules/web"
	apiv1 "code.gitea.io/gitea/routers/api/v1"
	"code.gitea.io/gitea/routers/common"
	"code.gitea.io/gitea/routers/private"
	web_routers "code.gitea.io/gitea/routers/web"
	"code.gitea.io/gitea/services/archiver"
	"code.gitea.io/gitea/services/auth"
	"code.gitea.io/gitea/services/auth/source/oauth2"
	"code.gitea.io/gitea/services/mailer"
	mirror_service "code.gitea.io/gitea/services/mirror"
	pull_service "code.gitea.io/gitea/services/pull"
	"code.gitea.io/gitea/services/repository"
	"code.gitea.io/gitea/services/webhook"
)

// NewServices init new services
func NewServices() {
	setting.NewServices()
	if err := storage.Init(); err != nil {
		log.Fatal("storage init failed: %v", err)
	}
	if err := repository.NewContext(); err != nil {
		log.Fatal("repository init failed: %v", err)
	}
	mailer.NewContext()
	if err := cache.NewContext(); err != nil {
		log.Fatal("Unable to start cache service: %v", err)
	}
	notification.NewContext()
	if err := archiver.Init(); err != nil {
		log.Fatal("archiver init failed: %v", err)
	}
}

// GlobalInit is for global configuration reload-able.
func GlobalInit(ctx context.Context) {
	setting.NewContext()
	if !setting.InstallLock {
		log.Fatal("Gitea is not installed")
	}

	if err := git.Init(ctx); err != nil {
		log.Fatal("Git module init failed: %v", err)
	}
	log.Info(git.VersionInfo())

	git.CheckLFSVersion()
	log.Info("AppPath: %s", setting.AppPath)
	log.Info("AppWorkPath: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Log path: %s", setting.LogRootPath)
	log.Info("Run Mode: %s", strings.Title(setting.RunMode))

	// Setup i18n
	translation.InitLocales()

	NewServices()

	highlight.NewContext()
	external.RegisterRenderers()
	markup.Init()

	if setting.EnableSQLite3 {
		log.Info("SQLite3 Supported")
	} else if setting.Database.UseSQLite3 {
		log.Fatal("SQLite3 is set in settings but NOT Supported")
	}
	if err := common.InitDBEngine(ctx); err == nil {
		log.Info("ORM engine initialization successful!")
	} else {
		log.Fatal("ORM engine initialization failed: %v", err)
	}

	if err := oauth2.Init(); err != nil {
		log.Fatal("Failed to initialize OAuth2 support: %v", err)
	}

	models.NewRepoContext()

	// Booting long running goroutines.
	cron.NewContext()
	issue_indexer.InitIssueIndexer(false)
	code_indexer.Init()
	if err := stats_indexer.Init(); err != nil {
		log.Fatal("Failed to initialize repository stats indexer queue: %v", err)
	}
	mirror_service.InitSyncMirrors()
	webhook.InitDeliverHooks()
	if err := pull_service.Init(); err != nil {
		log.Fatal("Failed to initialize test pull requests queue: %v", err)
	}
	if err := task.Init(); err != nil {
		log.Fatal("Failed to initialize task scheduler: %v", err)
	}
	if err := repo_migrations.Init(); err != nil {
		log.Fatal("Failed to initialize repository migrations: %v", err)
	}
	eventsource.GetManager().Init()

	if setting.SSH.StartBuiltinServer {
		ssh.Listen(setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)
		log.Info("SSH server started on %s:%d. Cipher list (%v), key exchange algorithms (%v), MACs (%v)", setting.SSH.ListenHost, setting.SSH.ListenPort, setting.SSH.ServerCiphers, setting.SSH.ServerKeyExchanges, setting.SSH.ServerMACs)
	} else {
		ssh.Unused()
	}
	auth.Init()

	svg.Init()
}

// NormalRoutes represents non install routes
func NormalRoutes() *web.Route {
	r := web.NewRoute()
	for _, middle := range common.Middlewares() {
		r.Use(middle)
	}

	r.Mount("/", web_routers.Routes())
	r.Mount("/api/v1", apiv1.Routes())
	r.Mount("/api/internal", private.Routes())
	return r
}
