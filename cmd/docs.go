// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

// CmdDocs represents the available docs sub-command.
var CmdDocs = cli.Command{
	Name:        "docs",
	Usage:       "Output CLI documentation",
	Description: "A command to output Gitea's CLI documentation, optionally to a file.",
	Action:      runDocs,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "man",
			Usage: "Output man pages instead",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Path to output to instead of stdout (will overwrite if exists)",
		},
	},
}

func runDocs(ctx *cli.Context) error {
	docs, err := ctx.App.ToMarkdown()
	if ctx.Bool("man") {
		docs, err = ctx.App.ToMan()
	}
	if err != nil {
		return err
	}

	out := os.Stdout
	if ctx.String("output") != "" {
		fi, err := os.Create(ctx.String("output"))
		if err != nil {
			return err
		}
		defer fi.Close()
		out = fi
	}

	_, err = fmt.Fprintln(out, docs)
	return err
}
