// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"code.gitea.io/gitea/modules/log"
)

// Package package plugin config
var Package struct {
	EnableRegistry bool
}

func newPackages() {
	cfg := Cfg.Section("package.container_registry")

	Package.EnableRegistry = cfg.Key("ENABLED_REGISTRY").MustBool(false)
	if Package.EnableRegistry {
		log.Info("Container Registry Enabled")
	}
}
