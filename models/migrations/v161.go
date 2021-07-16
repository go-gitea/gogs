// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func convertTaskTypeToString(x *xorm.Engine) error {
	const (
		GOGS int = iota + 1
		SLACK
		GITEA
		DISCORD
		DINGTALK
		TELEGRAM
		MSTEAMS
		FEISHU
		MATRIX
		WECHATWORK
	)

	hookTaskTypes := map[int]string{
		GITEA:      "gitea",
		GOGS:       "gogs",
		SLACK:      "slack",
		DISCORD:    "discord",
		DINGTALK:   "dingtalk",
		TELEGRAM:   "telegram",
		MSTEAMS:    "msteams",
		FEISHU:     "feishu",
		MATRIX:     "matrix",
		WECHATWORK: "wechatwork",
	}

	type HookTask struct {
		Typ string `xorm:"VARCHAR(16) index"`
	}
	if err := x.Sync2(new(HookTask)); err != nil {
		return err
	}

	for i, s := range hookTaskTypes {
		if _, err := x.Exec("UPDATE hook_task set typ = ? where type=?", s, i); err != nil {
			return err
		}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := dropTableColumns(sess, "hook_task", "type"); err != nil {
		return err
	}

	return sess.Commit()
}
