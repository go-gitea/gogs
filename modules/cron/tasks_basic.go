// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cron

import (
	"context"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/migrations"
	repository_service "code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	mirror_service "code.gitea.io/gitea/services/mirror"
)

func registerUpdateMirrorTask() {
	RegisterTaskFatal("update_mirrors", &BaseConfig{
		Enabled:         true,
		RunAtStart:      false,
		Schedule:        "@every 10m",
		NoSuccessNotice: true,
	}, func(ctx context.Context, _ *models.User, _ Config) error {
		return mirror_service.Update(ctx)
	})
}

func registerRepoHealthCheck() {
	type RepoHealthCheckConfig struct {
		BaseConfig
		Timeout time.Duration
		Args    []string `delim:" "`
	}
	RegisterTaskFatal("repo_health_check", &RepoHealthCheckConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: false,
			Schedule:   "@every 24h",
		},
		Timeout: 60 * time.Second,
		Args:    []string{},
	}, func(ctx context.Context, _ *models.User, config Config) error {
		rhcConfig := config.(*RepoHealthCheckConfig)
		return repository_service.GitFsck(ctx, rhcConfig.Timeout, rhcConfig.Args)
	})
}

func registerCheckRepoStats() {
	RegisterTaskFatal("check_repo_stats", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@every 24h",
	}, func(ctx context.Context, _ *models.User, _ Config) error {
		return models.CheckRepoStats(ctx)
	})
}

func registerArchiveCleanup() {
	RegisterTaskFatal("archive_cleanup", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: true,
			Schedule:   "@every 24h",
		},
		OlderThan: 24 * time.Hour,
	}, func(ctx context.Context, _ *models.User, config Config) error {
		acConfig := config.(*OlderThanConfig)
		return models.DeleteOldRepositoryArchives(ctx, acConfig.OlderThan)
	})
}

func registerSyncExternalUsers() {
	RegisterTaskFatal("sync_external_users", &UpdateExistingConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: false,
			Schedule:   "@every 24h",
		},
		UpdateExisting: true,
	}, func(ctx context.Context, _ *models.User, config Config) error {
		realConfig := config.(*UpdateExistingConfig)
		return models.SyncExternalUsers(ctx, realConfig.UpdateExisting)
	})
}

func registerDeletedBranchesCleanup() {
	RegisterTaskFatal("deleted_branches_cleanup", &OlderThanConfig{
		BaseConfig: BaseConfig{
			Enabled:    true,
			RunAtStart: true,
			Schedule:   "@every 24h",
		},
		OlderThan: 24 * time.Hour,
	}, func(ctx context.Context, _ *models.User, config Config) error {
		realConfig := config.(*OlderThanConfig)
		models.RemoveOldDeletedBranches(ctx, realConfig.OlderThan)
		return nil
	})
}

func registerUpdateMigrationPosterID() {
	RegisterTaskFatal("update_migration_poster_id", &BaseConfig{
		Enabled:    true,
		RunAtStart: true,
		Schedule:   "@every 24h",
	}, func(ctx context.Context, _ *models.User, _ Config) error {
		return migrations.UpdateMigrationPosterID(ctx)
	})
}

func initBasicTasks() {
	if !setting.Repository.DisableMirrors {
		registerUpdateMirrorTask()
	}
	registerRepoHealthCheck()
	registerCheckRepoStats()
	registerArchiveCleanup()
	registerSyncExternalUsers()
	registerDeletedBranchesCleanup()
	registerUpdateMigrationPosterID()
}
