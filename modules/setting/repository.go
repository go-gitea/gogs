// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// enumerates all the policy repository creating
const (
	RepoCreatingLastUserVisibility = "last"
	RepoCreatingPrivate            = "private"
	RepoCreatingPublic             = "public"
)

// Repository settings
var (
	Repository = struct {
		DetectedCharsetsOrder                   []string
		DetectedCharsetScore                    map[string]int `ini:"-"`
		AnsiCharset                             string
		ForcePrivate                            bool
		DefaultPrivate                          string
		DefaultPushCreatePrivate                bool
		MaxCreationLimit                        int
		MirrorQueueLength                       int
		PullRequestQueueLength                  int
		PreferredLicenses                       []string
		DisableHTTPGit                          bool
		AccessControlAllowOrigin                string
		UseCompatSSHURI                         bool
		DefaultCloseIssuesViaCommitsInAnyBranch bool
		EnablePushCreateUser                    bool
		EnablePushCreateOrg                     bool
		DisabledRepoUnits                       []string
		DefaultRepoUnits                        []string
		PrefixArchiveFiles                      bool
		DisableMirrors                          bool
		DisableMigrations                       bool
		DisableStars                            bool `ini:"DISABLE_STARS"`
		DefaultBranch                           string
		AllowAdoptionOfUnadoptedRepositories    bool
		AllowDeleteOfUnadoptedRepositories      bool

		// Repository editor settings
		Editor struct {
			LineWrapExtensions   []string
			PreviewableFileModes []string
		} `ini:"-"`

		// Repository upload settings
		Upload struct {
			Enabled      bool
			TempPath     string
			AllowedTypes string
			FileMaxSize  int64
			MaxFiles     int
		} `ini:"-"`

		// Repository local settings
		Local struct {
			LocalCopyPath string
		} `ini:"-"`

		// Pull request settings
		PullRequest struct {
			WorkInProgressPrefixes                   []string
			CloseKeywords                            []string
			ReopenKeywords                           []string
			DefaultMergeMessageCommitsLimit          int
			DefaultMergeMessageSize                  int
			DefaultMergeMessageAllAuthors            bool
			DefaultMergeMessageMaxApprovers          int
			DefaultMergeMessageOfficialApproversOnly bool
			PopulateSquashCommentWithCommitMessages  bool
		} `ini:"repository.pull-request"`

		// Issue Setting
		Issue struct {
			LockReasons []string
		} `ini:"repository.issue"`

		Release struct {
			AllowedTypes string
		} `ini:"repository.release"`

		Signing struct {
			SigningKey        string
			SigningName       string
			SigningEmail      string
			InitialCommit     []string
			CRUDActions       []string `ini:"CRUD_ACTIONS"`
			Merges            []string
			Wiki              []string
			DefaultTrustModel string
		} `ini:"repository.signing"`
	}{
		DetectedCharsetsOrder: []string{
			"UTF-8",
			"UTF-16BE",
			"UTF-16LE",
			"UTF-32BE",
			"UTF-32LE",
			"ISO-8859-1",
			"windows-1252",
			"ISO-8859-2",
			"windows-1250",
			"ISO-8859-5",
			"ISO-8859-6",
			"ISO-8859-7",
			"windows-1253",
			"ISO-8859-8-I",
			"windows-1255",
			"ISO-8859-8",
			"windows-1251",
			"windows-1256",
			"KOI8-R",
			"ISO-8859-9",
			"windows-1254",
			"Shift_JIS",
			"GB18030",
			"EUC-JP",
			"EUC-KR",
			"Big5",
			"ISO-2022-JP",
			"ISO-2022-KR",
			"ISO-2022-CN",
			"IBM424_rtl",
			"IBM424_ltr",
			"IBM420_rtl",
			"IBM420_ltr",
		},
		DetectedCharsetScore:                    map[string]int{},
		AnsiCharset:                             "",
		ForcePrivate:                            false,
		DefaultPrivate:                          RepoCreatingLastUserVisibility,
		DefaultPushCreatePrivate:                true,
		MaxCreationLimit:                        -1,
		MirrorQueueLength:                       1000,
		PullRequestQueueLength:                  1000,
		PreferredLicenses:                       []string{"Apache License 2.0", "MIT License"},
		DisableHTTPGit:                          false,
		AccessControlAllowOrigin:                "",
		UseCompatSSHURI:                         false,
		DefaultCloseIssuesViaCommitsInAnyBranch: false,
		EnablePushCreateUser:                    false,
		EnablePushCreateOrg:                     false,
		DisabledRepoUnits:                       []string{},
		DefaultRepoUnits:                        []string{},
		PrefixArchiveFiles:                      true,
		DisableMirrors:                          false,
		DisableMigrations:                       false,
		DisableStars:                            false,
		DefaultBranch:                           "master",

		// Repository editor settings
		Editor: struct {
			LineWrapExtensions   []string
			PreviewableFileModes []string
		}{
			LineWrapExtensions:   strings.Split(".txt,.md,.markdown,.mdown,.mkd,", ","),
			PreviewableFileModes: []string{"markdown"},
		},

		// Repository upload settings
		Upload: struct {
			Enabled      bool
			TempPath     string
			AllowedTypes string
			FileMaxSize  int64
			MaxFiles     int
		}{
			Enabled:      true,
			TempPath:     "data/tmp/uploads",
			AllowedTypes: "",
			FileMaxSize:  3,
			MaxFiles:     5,
		},

		// Repository local settings
		Local: struct {
			LocalCopyPath string
		}{
			LocalCopyPath: "tmp/local-repo",
		},

		// Pull request settings
		PullRequest: struct {
			WorkInProgressPrefixes                   []string
			CloseKeywords                            []string
			ReopenKeywords                           []string
			DefaultMergeMessageCommitsLimit          int
			DefaultMergeMessageSize                  int
			DefaultMergeMessageAllAuthors            bool
			DefaultMergeMessageMaxApprovers          int
			DefaultMergeMessageOfficialApproversOnly bool
			PopulateSquashCommentWithCommitMessages  bool
		}{
			WorkInProgressPrefixes: []string{"WIP:", "[WIP]"},
			// Same as GitHub. See
			// https://help.github.com/articles/closing-issues-via-commit-messages
			CloseKeywords:                            strings.Split("close,closes,closed,fix,fixes,fixed,resolve,resolves,resolved", ","),
			ReopenKeywords:                           strings.Split("reopen,reopens,reopened", ","),
			DefaultMergeMessageCommitsLimit:          50,
			DefaultMergeMessageSize:                  5 * 1024,
			DefaultMergeMessageAllAuthors:            false,
			DefaultMergeMessageMaxApprovers:          10,
			DefaultMergeMessageOfficialApproversOnly: true,
			PopulateSquashCommentWithCommitMessages:  false,
		},

		// Issue settings
		Issue: struct {
			LockReasons []string
		}{
			LockReasons: strings.Split("Too heated,Off-topic,Spam,Resolved", ","),
		},

		Release: struct {
			AllowedTypes string
		}{
			AllowedTypes: "",
		},

		// Signing settings
		Signing: struct {
			SigningKey        string
			SigningName       string
			SigningEmail      string
			InitialCommit     []string
			CRUDActions       []string `ini:"CRUD_ACTIONS"`
			Merges            []string
			Wiki              []string
			DefaultTrustModel string
		}{
			SigningKey:        "default",
			SigningName:       "",
			SigningEmail:      "",
			InitialCommit:     []string{"always"},
			CRUDActions:       []string{"pubkey", "twofa", "parentsigned"},
			Merges:            []string{"pubkey", "twofa", "basesigned", "commitssigned"},
			Wiki:              []string{"never"},
			DefaultTrustModel: "collaborator",
		},
	}
	RepoRootPath string
	ScriptType   = "bash"
)

func newRepository() {
	var err error
	// Determine and create root git repository path.
	sec := Cfg.Section("repository")
	Repository.DisableHTTPGit = sec.Key("DISABLE_HTTP_GIT").MustBool()
	Repository.UseCompatSSHURI = sec.Key("USE_COMPAT_SSH_URI").MustBool()
	Repository.MaxCreationLimit = sec.Key("MAX_CREATION_LIMIT").MustInt(-1)
	Repository.DefaultBranch = sec.Key("DEFAULT_BRANCH").MustString(Repository.DefaultBranch)
	RepoRootPath = sec.Key("ROOT").MustString(path.Join(AppDataPath, "gitea-repositories"))
	forcePathSeparator(RepoRootPath)
	if !filepath.IsAbs(RepoRootPath) {
		RepoRootPath = filepath.Join(AppWorkPath, RepoRootPath)
	} else {
		RepoRootPath = filepath.Clean(RepoRootPath)
	}
	defaultDetectedCharsetsOrder := make([]string, 0, len(Repository.DetectedCharsetsOrder))
	for _, charset := range Repository.DetectedCharsetsOrder {
		defaultDetectedCharsetsOrder = append(defaultDetectedCharsetsOrder, strings.ToLower(strings.TrimSpace(charset)))
	}
	ScriptType = sec.Key("SCRIPT_TYPE").MustString("bash")

	if err = Cfg.Section("repository").MapTo(&Repository); err != nil {
		log.Fatal("Failed to map Repository settings: %v", err)
	} else if err = Cfg.Section("repository.editor").MapTo(&Repository.Editor); err != nil {
		log.Fatal("Failed to map Repository.Editor settings: %v", err)
	} else if err = Cfg.Section("repository.upload").MapTo(&Repository.Upload); err != nil {
		log.Fatal("Failed to map Repository.Upload settings: %v", err)
	} else if err = Cfg.Section("repository.local").MapTo(&Repository.Local); err != nil {
		log.Fatal("Failed to map Repository.Local settings: %v", err)
	} else if err = Cfg.Section("repository.pull-request").MapTo(&Repository.PullRequest); err != nil {
		log.Fatal("Failed to map Repository.PullRequest settings: %v", err)
	}

	// Handle default trustmodel settings
	Repository.Signing.DefaultTrustModel = strings.ToLower(strings.TrimSpace(Repository.Signing.DefaultTrustModel))
	if Repository.Signing.DefaultTrustModel == "default" {
		Repository.Signing.DefaultTrustModel = "collaborator"
	}

	// Handle preferred charset orders
	preferred := make([]string, 0, len(Repository.DetectedCharsetsOrder))
	for _, charset := range Repository.DetectedCharsetsOrder {
		canonicalCharset := strings.ToLower(strings.TrimSpace(charset))
		preferred = append(preferred, canonicalCharset)
		// remove it from the defaults
		for i, charset := range defaultDetectedCharsetsOrder {
			if charset == canonicalCharset {
				defaultDetectedCharsetsOrder = append(defaultDetectedCharsetsOrder[:i], defaultDetectedCharsetsOrder[i+1:]...)
				break
			}
		}
	}

	i := 0
	for _, charset := range preferred {
		// Add the defaults
		if charset == "defaults" {
			for _, charset := range defaultDetectedCharsetsOrder {
				canonicalCharset := strings.ToLower(strings.TrimSpace(charset))
				if _, has := Repository.DetectedCharsetScore[canonicalCharset]; !has {
					Repository.DetectedCharsetScore[canonicalCharset] = i
					i++
				}
			}
			continue
		}
		if _, has := Repository.DetectedCharsetScore[charset]; !has {
			Repository.DetectedCharsetScore[charset] = i
			i++
		}
	}

	if !filepath.IsAbs(Repository.Upload.TempPath) {
		Repository.Upload.TempPath = path.Join(AppWorkPath, Repository.Upload.TempPath)
	}
}
