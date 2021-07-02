// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	golog "log"
	"os"
	"strings"
	"text/tabwriter"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/migrations"
	"code.gitea.io/gitea/modules/doctor"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"

	"github.com/urfave/cli/v2"
)

// CmdDoctor represents the available doctor sub-command.
var CmdDoctor = cli.Command{
	Name:        "doctor",
	Usage:       "Diagnose problems",
	Description: "A command to diagnose problems with the current Gitea instance according to the given configuration.",
	Action:      runDoctor,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "list",
			Usage: "List the available checks",
		},
		&cli.BoolFlag{
			Name:  "default",
			Usage: "Run the default checks (if neither --run or --all is set, this is the default behaviour)",
		},
		&cli.StringSliceFlag{
			Name:  "run",
			Usage: "Run the provided checks - (if --default is set, the default checks will also run)",
		},
		&cli.BoolFlag{
			Name:  "all",
			Usage: "Run all the available checks",
		},
		&cli.BoolFlag{
			Name:  "fix",
			Usage: "Automatically fix what we can",
		},
		&cli.StringFlag{
			Name:  "log-file",
			Usage: `Name of the log file (default: "doctor.log"). Set to "-" to output to stdout, set to "" to disable`,
		},
		&cli.BoolFlag{
			Name:    "color",
			Aliases: []string{"H"},
			Usage:   "Use color for outputted information",
		},
	},
	Subcommands: []*cli.Command{
		&cmdRecreateTable,
	},
}

var cmdRecreateTable = cli.Command{
	Name:      "recreate-table",
	Usage:     "Recreate tables from XORM definitions and copy the data.",
	ArgsUsage: "[TABLE]... : (TABLEs to recreate - leave blank for all)",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "debug",
			Usage: "Print SQL commands sent",
		},
	},
	Description: `The database definitions Gitea uses change across versions, sometimes changing default values and leaving old unused columns.

This command will cause Xorm to recreate tables, copying over the data and deleting the old table.

You should back-up your database before doing this and ensure that your database is up-to-date first.`,
	Action: runRecreateTable,
}

func runRecreateTable(ctx *cli.Context) error {
	// Redirect the default golog to here
	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.NewLoggerAsWriter("INFO", log.GetLogger(log.DEFAULT)))

	setting.NewContext()
	setting.InitDBConfig()

	setting.EnableXORMLog = ctx.Bool("debug")
	setting.Database.LogSQL = ctx.Bool("debug")
	setting.Cfg.Section("log").Key("XORM").SetValue(",")

	setting.NewXORMLogService(!ctx.Bool("debug"))
	if err := models.SetEngine(); err != nil {
		fmt.Println(err)
		fmt.Println("Check if you are using the right config file. You can use a --config directive to specify one.")
		return nil
	}

	args := ctx.Args()
	names := make([]string, 0, ctx.NArg())
	for i := 0; i < ctx.NArg(); i++ {
		names = append(names, args.Get(i))
	}

	beans, err := models.NamesToBean(names...)
	if err != nil {
		return err
	}
	recreateTables := migrations.RecreateTables(beans...)

	return models.NewEngine(context.Background(), func(x *xorm.Engine) error {
		if err := migrations.EnsureUpToDate(x); err != nil {
			return err
		}
		return recreateTables(x)
	})

}

func runDoctor(ctx *cli.Context) error {

	// Silence the default loggers
	log.DelNamedLogger("console")
	log.DelNamedLogger(log.DEFAULT)

	// Now setup our own
	logFile := ctx.String("log-file")
	if !ctx.IsSet("log-file") {
		logFile = "doctor.log"
	}

	colorize := log.CanColorStdout
	if ctx.IsSet("color") {
		colorize = ctx.Bool("color")
	}

	if len(logFile) == 0 {
		log.NewLogger(1000, "doctor", "console", fmt.Sprintf(`{"level":"NONE","stacktracelevel":"NONE","colorize":%t}`, colorize))
	} else if logFile == "-" {
		log.NewLogger(1000, "doctor", "console", fmt.Sprintf(`{"level":"trace","stacktracelevel":"NONE","colorize":%t}`, colorize))
	} else {
		log.NewLogger(1000, "doctor", "file", fmt.Sprintf(`{"filename":%q,"level":"trace","stacktracelevel":"NONE"}`, logFile))
	}

	// Finally redirect the default golog to here
	golog.SetFlags(0)
	golog.SetPrefix("")
	golog.SetOutput(log.NewLoggerAsWriter("INFO", log.GetLogger(log.DEFAULT)))

	if ctx.IsSet("list") {
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
		_, _ = w.Write([]byte("Default\tName\tTitle\n"))
		for _, check := range doctor.Checks {
			if check.IsDefault {
				_, _ = w.Write([]byte{'*'})
			}
			_, _ = w.Write([]byte{'\t'})
			_, _ = w.Write([]byte(check.Name))
			_, _ = w.Write([]byte{'\t'})
			_, _ = w.Write([]byte(check.Title))
			_, _ = w.Write([]byte{'\n'})
		}
		return w.Flush()
	}

	var checks []*doctor.Check
	if ctx.Bool("all") {
		checks = doctor.Checks
	} else if ctx.IsSet("run") {
		addDefault := ctx.Bool("default")
		names := ctx.StringSlice("run")
		for i, name := range names {
			names[i] = strings.ToLower(strings.TrimSpace(name))
		}

		for _, check := range doctor.Checks {
			if addDefault && check.IsDefault {
				checks = append(checks, check)
				continue
			}
			for _, name := range names {
				if name == check.Name {
					checks = append(checks, check)
					break
				}
			}
		}
	} else {
		for _, check := range doctor.Checks {
			if check.IsDefault {
				checks = append(checks, check)
			}
		}
	}

	// Now we can set up our own logger to return information about what the doctor is doing
	if err := log.NewNamedLogger("doctorouter",
		1000,
		"console",
		"console",
		fmt.Sprintf(`{"level":"INFO","stacktracelevel":"NONE","colorize":%t,"flags":-1}`, colorize)); err != nil {
		fmt.Println(err)
		return err
	}

	logger := log.GetLogger("doctorouter")
	defer logger.Close()
	return doctor.RunChecks(logger, ctx.Bool("fix"), checks)
}
