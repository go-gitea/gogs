// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func dropWebhookColumns(x *xorm.Engine) error {
	// Make sure the columns exist before dropping them
	type Webhook struct {
		Signature string `xorm:"TEXT"`
		IsSSL     bool   `xorm:"is_ssl"`
	}
	if err := x.Sync2(new(Webhook)); err != nil {
		return err
	}

	type HookTask struct {
		Typ         string `xorm:"VARCHAR(16) index"`
		URL         string `xorm:"TEXT"`
		Signature   string `xorm:"TEXT"`
		HTTPMethod  string `xorm:"http_method"`
		ContentType int
		IsSSL       bool
	}
	if err := x.Sync2(new(HookTask)); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := dropTableColumns(sess, "webhook", "signature", "is_ssl"); err != nil {
		return err
	}
	if err := dropTableColumns(sess, "hook_task", "typ", "url", "signature", "http_method", "content_type", "is_ssl"); err != nil {
		return err
	}

	return sess.Commit()
}
