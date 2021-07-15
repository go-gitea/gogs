// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cron

import (
	"context"
	"time"

	"code.gitea.io/gitea/models"
	repo_module "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
)

func registerDeleteInactiveUsers() {
	RegisterTaskFatal("delete_inactive_accounts", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@annually",
		},
		OlderThan: 0 * time.Second,
	}, func(ctx context.Context, _ *models.User, config Config) error {
		olderThanConfig := config.(*OlderThanConfig)
		return models.DeleteInactiveUsers(ctx, olderThanConfig.OlderThan)
	})
}

func registerDeleteRepositoryArchives() {
	RegisterTaskFatal("delete_repo_archives", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@annually",
	}, func(ctx context.Context, _ *models.User, _ Config) error {
		return repo_module.DeleteRepositoryArchives(ctx)
	})
}

func registerGarbageCollectRepositories() {
	type RepoHealthCheckConfig struct {
		BaseConfig
		Timeout time.Duration
		Args    []string `delim:" "`
	}
	RegisterTaskFatal("git_gc_repos", &RepoHealthCheckConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 72h",
		},
		Timeout: time.Duration(setting.Git.Timeout.GC) * time.Second,
		Args:    setting.Git.GCArgs,
	}, func(ctx context.Context, _ *models.User, config Config) error {
		rhcConfig := config.(*RepoHealthCheckConfig)
		return repo_module.GitGcRepos(ctx, rhcConfig.Timeout, rhcConfig.Args...)
	})
}

func registerRewriteAllPublicKeys() {
	RegisterTaskFatal("resync_all_sshkeys", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(_ context.Context, _ *models.User, _ Config) error {
		return models.RewriteAllPublicKeys()
	})
}

func registerRewriteAllPrincipalKeys() {
	RegisterTaskFatal("resync_all_sshprincipals", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(_ context.Context, _ *models.User, _ Config) error {
		return models.RewriteAllPrincipalKeys()
	})
}

func registerRepositoryUpdateHook() {
	RegisterTaskFatal("resync_all_hooks", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *models.User, _ Config) error {
		return repo_module.SyncRepositoryHooks(ctx)
	})
}

func registerReinitMissingRepositories() {
	RegisterTaskFatal("reinit_missing_repos", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *models.User, _ Config) error {
		return repo_module.ReinitMissingRepositories(ctx)
	})
}

func registerDeleteMissingRepositories() {
	RegisterTaskFatal("delete_missing_repos", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, user *models.User, _ Config) error {
		return repo_module.DeleteMissingRepositories(ctx, user)
	})
}

func registerRemoveRandomAvatars() {
	RegisterTaskFatal("delete_generated_repository_avatars", &BaseConfig{
		Enabled:    false,
		RunAtStart: false,
		Schedule:   "@every 72h",
	}, func(ctx context.Context, _ *models.User, _ Config) error {
		return models.RemoveRandomAvatars(ctx)
	})
}

func registerDeleteOldActions() {
	RegisterTaskFatal("delete_old_actions", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    false,
			RunAtStart: false,
			Schedule:   "@every 168h",
		},
		OlderThan: 365 * 24 * time.Hour,
	}, func(ctx context.Context, _ *models.User, config Config) error {
		olderThanConfig := config.(*OlderThanConfig)
		return models.DeleteOldActions(olderThanConfig.OlderThan)
	})
}

func initExtendedTasks() {
	registerDeleteInactiveUsers()
	registerDeleteRepositoryArchives()
	registerGarbageCollectRepositories()
	registerRewriteAllPublicKeys()
	registerRewriteAllPrincipalKeys()
	registerRepositoryUpdateHook()
	registerReinitMissingRepositories()
	registerDeleteMissingRepositories()
	registerRemoveRandomAvatars()
	registerDeleteOldActions()
}
