// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addPackageTable(x *xorm.Engine) error {
	type PackageType int64

	type Package struct {
		ID        int64 `xorm:"pk autoincr"`
		Name      string
		LowerName string `xorm:"INDEX"`

		RepoID int64 `xorm:"INDEX"`

		Type PackageType

		CreatedUnix int64 `xorm:"INDEX created"`
		UpdatedUnix int64 `xorm:"INDEX updated"`
	}

	return x.Sync(new(Package))
}
