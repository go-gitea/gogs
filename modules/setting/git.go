// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
)

// Git settings
var Git = struct {
	Path                      string
	DisableDiffHighlight      bool
	MaxGitDiffLines           int
	MaxGitDiffLineCharacters  int
	MaxGitDiffFiles           int
	CommitsRangeSize          int
	BranchesRangeSize         int
	VerbosePush               bool
	VerbosePushDelay          time.Duration
	GCArgs                    []string `ini:"GC_ARGS" delim:" "`
	EnableAutoGitWireProtocol bool
	PullRequestPushMessage    bool
	Timeout                   struct {
		Default int
		Migrate int
		Mirror  int
		Clone   int
		Pull    int
		GC      int `ini:"GC"`
	} `ini:"git.timeout"`
}{
	DisableDiffHighlight:      false,
	MaxGitDiffLines:           1000,
	MaxGitDiffLineCharacters:  5000,
	MaxGitDiffFiles:           100,
	CommitsRangeSize:          50,
	BranchesRangeSize:         20,
	VerbosePush:               true,
	VerbosePushDelay:          5 * time.Second,
	GCArgs:                    []string{},
	EnableAutoGitWireProtocol: true,
	PullRequestPushMessage:    true,
	Timeout: struct {
		Default int
		Migrate int
		Mirror  int
		Clone   int
		Pull    int
		GC      int `ini:"GC"`
	}{
		Default: int(git.DefaultCommandExecutionTimeout / time.Second),
		Migrate: 600,
		Mirror:  300,
		Clone:   300,
		Pull:    300,
		GC:      60,
	},
}

func newGit() {
	if err := Cfg.Section("git").MapTo(&Git); err != nil {
		log.Fatal("Failed to map Git settings: %v", err)
	}
	if err := git.SetExecutablePath(Git.Path); err != nil {
		log.Fatal("Failed to initialize Git settings: %v", err)
	}
	git.DefaultCommandExecutionTimeout = time.Duration(Git.Timeout.Default) * time.Second

	version, err := git.LocalVersion()
	if err != nil {
		log.Fatal("Error retrieving git version: %v", err)
	}

	// force cleanup args
	git.GlobalCommandArgs = []string{}

	if git.CheckGitVersionAtLeast("2.9") == nil {
		// Explicitly disable credential helper, otherwise Git credentials might leak
		git.GlobalCommandArgs = append(git.GlobalCommandArgs, "-c", "credential.helper=")
	}

	format := "Git Version: %s"
	args := []interface{}{version.Original()}
	// Since git wire protocol has been released from git v2.18
	if Git.EnableAutoGitWireProtocol && git.CheckGitVersionAtLeast("2.18") == nil {
		git.GlobalCommandArgs = append(git.GlobalCommandArgs, "-c", "protocol.version=2")
		format += ", Wire Protocol %s Enabled"
		args = append(args, "Version 2") // for focus color
	}

	git.CommitsRangeSize = Git.CommitsRangeSize
	git.BranchesRangeSize = Git.BranchesRangeSize

	log.Info(format, args...)
}
