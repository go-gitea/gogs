// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addOrgIDHookTaskColumn(x *xorm.Engine) error {
	type HookTask struct {
		OrgID int64 `xorm:"INDEX NOT NULL DEFAULT 0"`
	}

	if err := x.Sync2(new(HookTask)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
