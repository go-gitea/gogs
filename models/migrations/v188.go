// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func addAutoMergeTable(x *xorm.Engine) error {
	type MergeStyle string
	type ScheduledPullRequestMerge struct {
		ID          int64      `xorm:"pk autoincr"`
		PullID      int64      `xorm:"BIGINT"`
		DoerID      int64      `xorm:"BIGINT"`
		MergeStyle  MergeStyle `xorm:"varchar(50)"`
		Message     string     `xorm:"TEXT"`
		CreatedUnix int64      `xorm:"created"`
	}

	return x.Sync2(&ScheduledPullRequestMerge{})
}
