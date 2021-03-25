// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Gitea (git with a cup of tea) is a painless self-hosted Git Service.
package main // import "code.gitea.io/gitea"

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"code.gitea.io/gitea/cmd"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	// register supported doc types
	_ "code.gitea.io/gitea/modules/markup/csv"
	_ "code.gitea.io/gitea/modules/markup/markdown"
	_ "code.gitea.io/gitea/modules/markup/orgmode"

	"github.com/urfave/cli"
)

var (
	// Version holds the current Gitea version
	Version = "development"
	// Tags holds the build tags used
	Tags = ""
	// MakeVersion holds the current Make version if built with make
	MakeVersion = ""

	originalAppHelpTemplate        = ""
	originalCommandHelpTemplate    = ""
	originalSubcommandHelpTemplate = ""
)

func init() {
	setting.AppVer = Version
	setting.AppBuiltWith = formatBuiltWith()
	setting.AppStartTime = time.Now().UTC()

	// Grab the original help templates
	originalAppHelpTemplate = cli.AppHelpTemplate
	originalCommandHelpTemplate = cli.CommandHelpTemplate
	originalSubcommandHelpTemplate = cli.SubcommandHelpTemplate
}

func main() {
	app := cli.NewApp()
	app.Name = "Gitea"
	app.Usage = "A painless self-hosted Git service"
	app.Description = `By default, gitea will start serving using the webserver with no
arguments - which can alternatively be run by running the subcommand web.`
	app.Version = Version + formatBuiltWith()
	app.Commands = []cli.Command{
		cmd.CmdWeb,
		cmd.CmdServ,
		cmd.CmdHook,
		cmd.CmdDump,
		cmd.CmdCert,
		cmd.CmdAdmin,
		cmd.CmdGenerate,
		cmd.CmdMigrate,
		cmd.CmdKeys,
		cmd.CmdConvert,
		cmd.CmdDoctor,
		cmd.CmdManager,
		cmd.Cmdembedded,
		cmd.CmdMigrateStorage,
		cmd.CmdDocs,
		cmd.CmdDumpRepository,
		cmd.CmdRestoreRepository,
	}
	// Now adjust these commands to add our global configuration options

	// First calculate the default paths and set the AppHelpTemplates in this context
	setting.SetCustomPathAndConf("", "", "")
	setAppHelpTemplates()

	// default configuration flags
	defaultFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "custom-path, C",
			Value: setting.CustomPath,
			Usage: "Custom path file path",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: setting.CustomConf,
			Usage: "Custom configuration file path",
		},
		cli.VersionFlag,
		cli.StringFlag{
			Name:  "work-path, w",
			Value: setting.AppWorkPath,
			Usage: "Set the gitea working path",
		},
	}

	// Set the default to be equivalent to cmdWeb and add the default flags
	app.Flags = append(app.Flags, cmd.CmdWeb.Flags...)
	app.Flags = append(app.Flags, defaultFlags...)
	app.Action = cmd.CmdWeb.Action

	// Add functions to set these paths and these flags to the commands
	app.Before = establishCustomPath
	for i := range app.Commands {
		setFlagsAndBeforeOnSubcommands(&app.Commands[i], defaultFlags, establishCustomPath)
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal("Failed to run app with %s: %v", os.Args, err)
	}
}

func setFlagsAndBeforeOnSubcommands(command *cli.Command, defaultFlags []cli.Flag, before cli.BeforeFunc) {
	command.Flags = append(command.Flags, defaultFlags...)
	command.Before = establishCustomPath
	for i := range command.Subcommands {
		setFlagsAndBeforeOnSubcommands(&command.Subcommands[i], defaultFlags, before)
	}
}

func establishCustomPath(ctx *cli.Context) error {
	var providedCustom string
	var providedConf string
	var providedWorkPath string

	currentCtx := ctx
	for {
		if len(providedCustom) != 0 && len(providedConf) != 0 && len(providedWorkPath) != 0 {
			break
		}
		if currentCtx == nil {
			break
		}
		if currentCtx.IsSet("custom-path") && len(providedCustom) == 0 {
			providedCustom = currentCtx.String("custom-path")
		}
		if currentCtx.IsSet("config") && len(providedConf) == 0 {
			providedConf = currentCtx.String("config")
		}
		if currentCtx.IsSet("work-path") && len(providedWorkPath) == 0 {
			providedWorkPath = currentCtx.String("work-path")
		}
		currentCtx = currentCtx.Parent()

	}
	setting.SetCustomPathAndConf(providedCustom, providedConf, providedWorkPath)

	setAppHelpTemplates()

	if ctx.IsSet("version") {
		cli.ShowVersion(ctx)
		os.Exit(0)
	}

	return nil
}

func setAppHelpTemplates() {
	cli.AppHelpTemplate = adjustHelpTemplate(originalAppHelpTemplate)
	cli.CommandHelpTemplate = adjustHelpTemplate(originalCommandHelpTemplate)
	cli.SubcommandHelpTemplate = adjustHelpTemplate(originalSubcommandHelpTemplate)
}

func adjustHelpTemplate(originalTemplate string) string {
	overrided := ""
	if _, ok := os.LookupEnv("GITEA_CUSTOM"); ok {
		overrided = "(GITEA_CUSTOM)"
	}

	return fmt.Sprintf(`%s
DEFAULT CONFIGURATION:
     CustomPath:  %s %s
     CustomConf:  %s
     AppPath:     %s
     AppWorkPath: %s

`, originalTemplate, setting.CustomPath, overrided, setting.CustomConf, setting.AppPath, setting.AppWorkPath)
}

func formatBuiltWith() string {
	version := runtime.Version()
	if len(MakeVersion) > 0 {
		version = MakeVersion + ", " + runtime.Version()
	}
	if len(Tags) == 0 {
		return " built with " + version
	}

	return " built with " + version + " : " + strings.ReplaceAll(Tags, " ", ", ")
}
