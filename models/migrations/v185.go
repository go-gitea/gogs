// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addRepoArchiver(x *xorm.Engine) error {
	// RepoArchiver represents all archivers
	type RepoArchiver struct {
		ID          int64 `xorm:"pk autoincr"`
		RepoID      int64 `xorm:"index unique(s)"`
		Type        int   `xorm:"unique(s)"`
		Status      int
		CommitID    string `xorm:"VARCHAR(40) unique(s)"`
		CreatedUnix int64  `xorm:"INDEX NOT NULL created"`
	}
	return x.Sync2(new(RepoArchiver))
}
