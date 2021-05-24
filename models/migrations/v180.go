// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addRenamedBranchTable(x *xorm.Engine) error {
	type RenamedBranch struct {
		ID          int64 `xorm:"pk autoincr"`
		RepoID      int64 `xorm:"INDEX NOT NULL"`
		From        string
		To          string
		CreatedUnix int64 `xorm:"created"`
	}
	return x.Sync2(new(RenamedBranch))
}
