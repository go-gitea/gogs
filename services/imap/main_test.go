// Copyright 2021 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package imap

import (
	"path/filepath"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"
)

func TestMain(m *testing.M) {
	// init test config
	testMode = true
	setting.MailReciveService = &setting.MailReciver{
		ReciveEmail:    "receive@gitea.io",
		ReciveBox:      "INBOX",
		QueueLength:    100,
		Host:           "127.0.0.1:1313",
		User:           "receive@gitea.io",
		Passwd:         "123456",
		IsTLSEnabled:   false,
		DeleteRodeMail: true,
	}

	c = new(Client)

	c.UserName = setting.MailReciveService.User
	c.Passwd = setting.MailReciveService.Passwd
	c.Addr = setting.MailReciveService.Host
	c.IsTLS = setting.MailReciveService.IsTLSEnabled
	c.Client = new(testIMAPClient)

	models.MainTest(m, filepath.Join("..", ".."))
}
