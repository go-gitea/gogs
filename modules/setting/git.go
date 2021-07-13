// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"time"

	"code.gitea.io/gitea/modules/log"
)

var (
	// Git settings
	Git = struct {
		Path                      string
		DisableDiffHighlight      bool
		MaxGitDiffLines           int
		MaxGitDiffLineCharacters  int
		MaxGitDiffFiles           int
		CommitsRangeSize          int // CommitsRangeSize the default commits range size
		BranchesRangeSize         int // BranchesRangeSize the default branches range size
		VerbosePush               bool
		VerbosePushDelay          time.Duration
		GCArgs                    []string `ini:"GC_ARGS" delim:" "`
		EnableAutoGitWireProtocol bool
		PullRequestPushMessage    bool
		LargeObjectThreshold      int64
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
		LargeObjectThreshold:      1024 * 1024,
		Timeout: struct {
			Default int
			Migrate int
			Mirror  int
			Clone   int
			Pull    int
			GC      int `ini:"GC"`
		}{
			Default: 360,
			Migrate: 600,
			Mirror:  300,
			Clone:   300,
			Pull:    300,
			GC:      60,
		},
	}
)

func newGit() {
	if err := Cfg.Section("git").MapTo(&Git); err != nil {
		log.Fatal("Failed to map Git settings: %v", err)
	}
}
