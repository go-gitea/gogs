// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm/schemas"
)

// ConvertUtf8ToUtf8mb4 converts database and tables from utf8 to utf8mb4 if it's mysql and set ROW_FORMAT=dynamic
func ConvertUtf8ToUtf8mb4() error {
	if x.Dialect().URI().DBType != schemas.MYSQL {
		return nil
	}

	_, err := x.Exec(fmt.Sprintf("ALTER DATABASE `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci", setting.Database.Name))
	if err != nil {
		return err
	}

	tables, err := x.DBMetas()
	if err != nil {
		return err
	}
	for _, table := range tables {
		if _, err := x.Exec(fmt.Sprintf("ALTER TABLE `%s` ROW_FORMAT=dynamic;", table.Name)); err != nil {
			return err
		}

		if _, err := x.Exec(fmt.Sprintf("ALTER TABLE `%s` CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;", table.Name)); err != nil {
			return err
		}
	}

	return nil
}
