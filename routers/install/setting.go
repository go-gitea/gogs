// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package install

import (
	"context"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/svg"
	"code.gitea.io/gitea/modules/translation"
	"code.gitea.io/gitea/routers/common"
)

// PreloadSettings preloads the configuration to check if we need to run install
func PreloadSettings(ctx context.Context) bool {
	setting.NewContext()
	if !setting.InstallLock {
		log.Info("AppPath: %s", setting.AppPath)
		log.Info("AppWorkPath: %s", setting.AppWorkPath)
		log.Info("Custom path: %s", setting.CustomPath)
		log.Info("Log path: %s", setting.LogRootPath)
		log.Info("Preparing to run install page")
		translation.InitLocales()
		if setting.EnableSQLite3 {
			log.Info("SQLite3 Supported")
		}
		setting.InitDBConfig()
		setting.NewServicesForInstall()
		svg.Init()
	}

	return !setting.InstallLock
}

// ReloadSettings rereads the settings and starts up the database
func ReloadSettings(ctx context.Context) {
	setting.NewContext()
	setting.InitDBConfig()
	if setting.InstallLock {
		if err := common.InitDBEngine(ctx); err == nil {
			log.Info("ORM engine initialization successful!")
		} else {
			log.Fatal("ORM engine initialization failed: %v", err)
		}
		svg.Init()
	}
}
