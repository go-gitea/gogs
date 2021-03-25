// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	pull_service "code.gitea.io/gitea/services/pull"

	"github.com/urfave/cli"
)

// CmdRestoreRepository represents the available restore a repository sub-command.
var CmdRestoreRepository = cli.Command{
	Name:        "restore-repo",
	Usage:       "Restore the repository from disk",
	Description: "This is a command for restoring the repository data.",
	Action:      runRestoreRepository,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "repo_dir, r",
			Value: "./data",
			Usage: "Repository dir path to restore from",
		},
		cli.StringFlag{
			Name:  "owner_name",
			Value: "",
			Usage: "Restore destination owner name",
		},
		cli.StringFlag{
			Name:  "repo_name",
			Value: "",
			Usage: "Restore destination repository name",
		},
		cli.StringFlag{
			Name:  "units",
			Value: "",
			Usage: `Which items will be restored, one or more units should be separated as comma. 
wiki, issues, labels, releases, release_assets, milestones, pull_requests, comments are allowed. Empty means all units.`,
		},
	},
}

func runRestoreRepository(ctx *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	log.Trace("AppPath: %s", setting.AppPath)
	log.Trace("AppWorkPath: %s", setting.AppWorkPath)
	log.Trace("Custom path: %s", setting.CustomPath)
	log.Trace("Log path: %s", setting.LogRootPath)
	setting.InitDBConfig()

	if err := storage.Init(); err != nil {
		return err
	}

	if err := pull_service.Init(); err != nil {
		return err
	}

	opts := base.MigrateOptions{
		RepoName: ctx.String("repo_name"),
	}

	if len(ctx.String("units")) == 0 {
		opts.Wiki = true
		opts.Issues = true
		opts.Milestones = true
		opts.Labels = true
		opts.Releases = true
		opts.Comments = true
		opts.PullRequests = true
		opts.ReleaseAssets = true
	} else {
		units := strings.Split(ctx.String("units"), ",")
		for _, unit := range units {
			switch strings.ToLower(unit) {
			case "wiki":
				opts.Wiki = true
			case "issues":
				opts.Issues = true
			case "milestones":
				opts.Milestones = true
			case "labels":
				opts.Labels = true
			case "releases":
				opts.Releases = true
			case "release_assets":
				opts.ReleaseAssets = true
			case "comments":
				opts.Comments = true
			case "pull_requests":
				opts.PullRequests = true
			}
		}
	}

	if err := migrations.RestoreRepository(
		context.Background(),
		ctx.String("repo_dir"),
		ctx.String("owner_name"),
		ctx.String("repo_name"),
	); err != nil {
		log.Fatal("Failed to restore repository: %v", err)
		return err
	}

	return nil
}
