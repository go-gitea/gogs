// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"net/http"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"

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

func runRestoreRepository(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	setting.NewContext()

	statusCode, errStr := private.RestoreRepo(
		ctx,
		c.String("repo_dir"),
		c.String("owner_name"),
		c.String("repo_name"),
		c.StringSlice("units"),
	)
	if statusCode == http.StatusOK {
		return nil
	}

	log.Fatal("Failed to restore repository: %v", errStr)
	return errors.New(errStr)
}
