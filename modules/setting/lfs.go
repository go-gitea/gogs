// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"encoding/base64"
	"time"

	"code.gitea.io/gitea/modules/generate"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"

	ini "gopkg.in/ini.v1"
)

// LFS represents the configuration for Git LFS
var LFS = struct {
	StartServer     bool          `ini:"LFS_START_SERVER"`
	JWTSecretBase64 string        `ini:"LFS_JWT_SECRET"`
	JWTSecretBytes  []byte        `ini:"-"`
	HTTPAuthExpiry  time.Duration `ini:"LFS_HTTP_AUTH_EXPIRY"`
	MaxFileSize     int64         `ini:"LFS_MAX_FILE_SIZE"`
	LocksPagingNum  int           `ini:"LFS_LOCKS_PAGING_NUM"`

	Storage
}{}

func newLFSService() {
	sec := Cfg.Section("server")
	if err := sec.MapTo(&LFS); err != nil {
		log.Fatal("Failed to map LFS settings: %v", err)
	}

	lfsSec := Cfg.Section("lfs")
	storageType := lfsSec.Key("STORAGE_TYPE").MustString("")

	// Specifically default PATH to LFS_CONTENT_PATH
	lfsSec.Key("PATH").MustString(
		sec.Key("LFS_CONTENT_PATH").String())

	LFS.Storage = getStorage("lfs", storageType, lfsSec)

	// Rest of LFS service settings
	if LFS.LocksPagingNum == 0 {
		LFS.LocksPagingNum = 50
	}

	LFS.HTTPAuthExpiry = sec.Key("LFS_HTTP_AUTH_EXPIRY").MustDuration(20 * time.Minute)

	if LFS.StartServer {
		LFS.JWTSecretBytes = make([]byte, 32)
		n, err := base64.RawURLEncoding.Decode(LFS.JWTSecretBytes, []byte(LFS.JWTSecretBase64))

		if err != nil || n != 32 {
			LFS.JWTSecretBase64, err = generate.NewJwtSecretBase64()
			if err != nil {
				log.Fatal("Error generating JWT Secret for custom config: %v", err)
				return
			}

			// Save secret
			CreateOrAppendToCustomConf(func(cfg *ini.File) {
				cfg.Section("server").Key("LFS_JWT_SECRET").SetValue(LFS.JWTSecretBase64)
			})
		}
	}
}

// CheckLFSVersion will check lfs version, if not satisfied, then disable it.
func CheckLFSVersion() {
	if LFS.StartServer {
		//Disable LFS client hooks if installed for the current OS user
		//Needs at least git v2.1.2

		err := git.LoadGitVersion()
		if err != nil {
			log.Fatal("Error retrieving git version: %v", err)
		}

		if git.CheckGitVersionAtLeast("2.1.2") != nil {
			LFS.StartServer = false
			log.Error("LFS server support needs at least Git v2.1.2")
		} else {
			git.GlobalCommandArgs = append(git.GlobalCommandArgs, "-c", "filter.lfs.required=",
				"-c", "filter.lfs.smudge=", "-c", "filter.lfs.clean=")
		}
	}
}
