// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"net/mail"
	"time"

	"code.gitea.io/gitea/modules/log"

	shellquote "github.com/kballard/go-shellquote"
)

// Mailer represents mail service.
type Mailer struct {
	// Mailer
	QueueLength     int
	Name            string
	From            string
	FromName        string
	FromEmail       string
	SendAsPlainText bool
	MailerType      string
	SubjectPrefix   string

	// SMTP sender
	Host              string
	User, Passwd      string
	DisableHelo       bool
	HeloHostname      string
	SkipVerify        bool
	UseCertificate    bool
	CertFile, KeyFile string
	IsTLSEnabled      bool

	// Sendmail sender
	SendmailPath    string
	SendmailArgs    []string
	SendmailTimeout time.Duration
}

// MailService the global mailer
var MailService *Mailer

func newMailService() {
	sec := Cfg.Section("mailer")
	// Check mailer setting.
	if !sec.Key("ENABLED").MustBool() {
		return
	}

	MailService = &Mailer{
		QueueLength:     sec.Key("SEND_BUFFER_LEN").MustInt(100),
		Name:            sec.Key("NAME").MustString(AppName),
		SendAsPlainText: sec.Key("SEND_AS_PLAIN_TEXT").MustBool(false),
		MailerType:      sec.Key("MAILER_TYPE").In("", []string{"smtp", "sendmail", "dummy"}),

		Host:           sec.Key("HOST").String(),
		User:           sec.Key("USER").String(),
		Passwd:         sec.Key("PASSWD").String(),
		DisableHelo:    sec.Key("DISABLE_HELO").MustBool(),
		HeloHostname:   sec.Key("HELO_HOSTNAME").String(),
		SkipVerify:     sec.Key("SKIP_VERIFY").MustBool(),
		UseCertificate: sec.Key("USE_CERTIFICATE").MustBool(),
		CertFile:       sec.Key("CERT_FILE").String(),
		KeyFile:        sec.Key("KEY_FILE").String(),
		IsTLSEnabled:   sec.Key("IS_TLS_ENABLED").MustBool(),
		SubjectPrefix:  sec.Key("SUBJECT_PREFIX").MustString(""),

		SendmailPath:    sec.Key("SENDMAIL_PATH").MustString("sendmail"),
		SendmailTimeout: sec.Key("SENDMAIL_TIMEOUT").MustDuration(5 * time.Minute),
	}
	MailService.From = sec.Key("FROM").MustString(MailService.User)

	if sec.HasKey("ENABLE_HTML_ALTERNATIVE") {
		log.Warn("ENABLE_HTML_ALTERNATIVE is deprecated, use SEND_AS_PLAIN_TEXT")
		MailService.SendAsPlainText = !sec.Key("ENABLE_HTML_ALTERNATIVE").MustBool(false)
	}

	if sec.HasKey("USE_SENDMAIL") {
		log.Warn("USE_SENDMAIL is deprecated, use MAILER_TYPE=sendmail")
		if MailService.MailerType == "" && sec.Key("USE_SENDMAIL").MustBool(false) {
			MailService.MailerType = "sendmail"
		}
	}

	parsed, err := mail.ParseAddress(MailService.From)
	if err != nil {
		log.Fatal("Invalid mailer.FROM (%s): %v", MailService.From, err)
	}
	MailService.FromName = parsed.Name
	MailService.FromEmail = parsed.Address

	if MailService.MailerType == "" {
		MailService.MailerType = "smtp"
	}

	if MailService.MailerType == "sendmail" {
		MailService.SendmailArgs, err = shellquote.Split(sec.Key("SENDMAIL_ARGS").String())
		if err != nil {
			log.Error("Failed to parse Sendmail args: %s with error %v", CustomConf, err)
		}
	}

	log.Info("Mail Service Enabled")
}

func newRegisterMailService() {
	if !Cfg.Section("service").Key("REGISTER_EMAIL_CONFIRM").MustBool() {
		return
	} else if MailService == nil {
		log.Warn("Register Mail Service: Mail Service is not enabled")
		return
	}
	Service.RegisterEmailConfirm = true
	log.Info("Register Mail Service Enabled")
}

func newNotifyMailService() {
	if !Cfg.Section("service").Key("ENABLE_NOTIFY_MAIL").MustBool() {
		return
	} else if MailService == nil {
		log.Warn("Notify Mail Service: Mail Service is not enabled")
		return
	}
	Service.EnableNotifyMail = true
	log.Info("Notify Mail Service Enabled")
}
