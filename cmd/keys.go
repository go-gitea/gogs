// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/private"

	"github.com/urfave/cli"
)

// CmdKeys represents the available keys sub-command
var CmdKeys = cli.Command{
	Name:   "keys",
	Usage:  "This command queries the Gitea database to get the authorized command for a given ssh key fingerprint",
	Action: runKeys,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "expected, e",
			Value: "git",
			Usage: "Expected user for whom provide key commands",
		},
		cli.StringFlag{
			Name:  "username, u",
			Value: "",
			Usage: "Username trying to log in by SSH",
		},
		cli.StringFlag{
			Name:  "type, t",
			Value: "",
			Usage: "Type of the SSH key provided to the SSH Server (requires content to be provided too)",
		},
		cli.StringFlag{
			Name:  "content, k",
			Value: "",
			Usage: "Base64 encoded content of the SSH key provided to the SSH Server (requires type to be provided too)",
		},
	},
}

func runKeys(c *cli.Context) error {
	if !c.IsSet("username") {
		return errors.New("No username provided")
	}
	// Check username matches the expected username
	if strings.TrimSpace(c.String("username")) != strings.TrimSpace(c.String("expected")) {
		return nil
	}

	content := ""

	if c.IsSet("type") && c.IsSet("content") {
		content = fmt.Sprintf("%s %s", strings.TrimSpace(c.String("type")), strings.TrimSpace(c.String("content")))
	}

	if content == "" {
		return errors.New("No key type and content provided")
	}

	ctx, cancel := installSignals()
	defer cancel()

	setup("keys.log", false)

	authorizedString, err := private.AuthorizedPublicKeyByContent(ctx, content)
	if err != nil {
		return err
	}
	fmt.Println(strings.TrimSpace(authorizedString))
	return nil
}
