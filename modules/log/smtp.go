// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"net/smtp"
	"strings"

	"github.com/goccy/go-json"
)

type smtpWriter struct {
	owner *SMTPLogger
}

// Write sends the message as an email
func (s *smtpWriter) Write(p []byte) (int, error) {
	return s.owner.sendMail(p)
}

// Close does nothing
func (s *smtpWriter) Close() error {
	return nil
}

// SMTPLogger implements LoggerProvider and is used to send emails via given SMTP-server.
type SMTPLogger struct {
	WriterLogger
	Username           string   `json:"Username"`
	Password           string   `json:"password"`
	Host               string   `json:"host"`
	Subject            string   `json:"subject"`
	RecipientAddresses []string `json:"sendTos"`
	sendMailFn         func(string, smtp.Auth, string, []string, []byte) error
}

// NewSMTPLogger creates smtp writer.
func NewSMTPLogger() LoggerProvider {
	s := &SMTPLogger{}
	s.Level = TRACE
	s.sendMailFn = smtp.SendMail
	return s
}

// Init smtp writer with json config.
// config like:
//	{
//		"Username":"example@gmail.com",
//		"password:"password",
//		"host":"smtp.gmail.com:465",
//		"subject":"email title",
//		"sendTos":["email1","email2"],
//		"level":LevelError
//	}
func (log *SMTPLogger) Init(jsonconfig string) error {
	err := json.Unmarshal([]byte(jsonconfig), log)
	if err != nil {
		return err
	}
	log.NewWriterLogger(&smtpWriter{
		owner: log,
	})
	log.sendMailFn = smtp.SendMail
	return nil
}

// WriteMsg writes message in smtp writer.
// it will send an email with subject and only this message.
func (log *SMTPLogger) sendMail(p []byte) (int, error) {
	hp := strings.Split(log.Host, ":")

	// Set up authentication information.
	auth := smtp.PlainAuth(
		"",
		log.Username,
		log.Password,
		hp[0],
	)
	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	contentType := "Content-Type: text/plain" + "; charset=UTF-8"
	mailmsg := []byte("To: " + strings.Join(log.RecipientAddresses, ";") + "\r\nFrom: " + log.Username + "<" + log.Username +
		">\r\nSubject: " + log.Subject + "\r\n" + contentType + "\r\n\r\n")
	mailmsg = append(mailmsg, p...)
	return len(p), log.sendMailFn(
		log.Host,
		auth,
		log.Username,
		log.RecipientAddresses,
		mailmsg,
	)
}

// Flush when log should be flushed
func (log *SMTPLogger) Flush() {
}

// ReleaseReopen does nothing
func (log *SMTPLogger) ReleaseReopen() error {
	return nil
}

// GetName returns the default name for this implementation
func (log *SMTPLogger) GetName() string {
	return "smtp"
}

func init() {
	Register("smtp", NewSMTPLogger)
}
