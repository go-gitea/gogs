// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import "code.gitea.io/gitea/modules/log"

// MailReciver represents mail recive service.
type MailReciver struct {
	ReciveEmail string
	ReciveBox   string
	QueueLength int

	Host         string
	User, Passwd string
	IsTLSEnabled bool

	DeleteRodeMail bool
}

var (
	// MailReciveService mail recive config
	MailReciveService *MailReciver
)

func newMailReciveService() {
	sec := Cfg.Section("mail_recive")
	// Check mailer setting.
	if !sec.Key("ENABLED").MustBool() {
		return
	}

	MailReciveService = &MailReciver{
		ReciveEmail:    sec.Key("RECIVE_EMAIL").String(),
		ReciveBox:      sec.Key("RECIVE_BOX").MustString("INBOX"),
		QueueLength:    sec.Key("READ_BUFFER_LEN").MustInt(100),
		Host:           sec.Key("HOST").String(),
		User:           sec.Key("USER").String(),
		Passwd:         sec.Key("PASSWD").String(),
		IsTLSEnabled:   sec.Key("IS_TLS_ENABLED").MustBool(true),
		DeleteRodeMail: sec.Key("DELETE_RODE_MAIL").MustBool(false),
	}

	log.Info("Mail Recive Service Enabled")
}
