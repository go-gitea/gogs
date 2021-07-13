// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/urfave/cli"
)

// CmdConvert represents the available convert sub-command.
var CmdConvert = cli.Command{
	Name:        "convert",
	Usage:       "Convert the database",
	Description: "A command to convert an existing MySQL database from utf8 to utf8mb4",
	Action:      runConvert,
}

func runConvert(ctx *cli.Context) error {
	if err := initDB(); err != nil {
		return err
	}

	log.Info("AppPath: %s", setting.AppPath)
	log.Info("AppWorkPath: %s", setting.AppWorkPath)
	log.Info("Custom path: %s", setting.CustomPath)
	log.Info("Log path: %s", setting.LogRootPath)
	setting.InitDBConfig()

	if !setting.Database.UseMySQL {
		fmt.Println("This command can only be used with a MySQL database")
		return nil
	}

	if err := models.ConvertUtf8ToUtf8mb4(); err != nil {
		log.Fatal("Failed to convert database from utf8 to utf8mb4: %v", err)
		return err
	}

	fmt.Println("Converted successfully, please confirm your database's character set is now utf8mb4")

	return nil
}
