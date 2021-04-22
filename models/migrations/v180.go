// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

func addWhitespaceBehaviorUserColumn(x *xorm.Engine) error {
	return addColumn(x, "user", &schemas.Column{
		Name: "WhitespaceBehavior",
		SQLType: schemas.SQLType{
			Name: schemas.Text,
		},
		Default:  "",
		Nullable: true,
	})
}
