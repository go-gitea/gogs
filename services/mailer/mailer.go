// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"os"
	"os/exec"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"

	"github.com/jaytaylor/html2text"
	"gopkg.in/gomail.v2"
)

// Message mail body and log info
type Message struct {
	Info            string // Message information for log purpose.
	FromAddress     string
	FromDisplayName string
	To              []string
	Subject         string
	Date            time.Time
	Body            string
	Headers         map[string][]string
}

// ToMessage converts a Message to gomail.Message
func (m *Message) ToMessage() *gomail.Message {
	msg := gomail.NewMessage()
	msg.SetAddressHeader("From", m.FromAddress, m.FromDisplayName)
	msg.SetHeader("To", m.To...)
	for header := range m.Headers {
		msg.SetHeader(header, m.Headers[header]...)
	}

	if len(setting.MailService.SubjectPrefix) > 0 {
		msg.SetHeader("Subject", setting.MailService.SubjectPrefix+" "+m.Subject)
	} else {
		msg.SetHeader("Subject", m.Subject)
	}
	msg.SetDateHeader("Date", m.Date)
	msg.SetHeader("X-Auto-Response-Suppress", "All")

	plainBody, err := html2text.FromString(m.Body)
	if err != nil || setting.MailService.SendAsPlainText {
		if strings.Contains(base.TruncateString(m.Body, 100), "<html>") {
			log.Warn("Mail contains HTML but configured to send as plain text.")
		}
		msg.SetBody("text/plain", plainBody)
	} else {
		msg.SetBody("text/plain", plainBody)
		msg.AddAlternative("text/html", m.Body)
	}
	return msg
}

// SetHeader adds additional headers to a message
func (m *Message) SetHeader(field string, value ...string) {
	m.Headers[field] = value
}

// NewMessageFrom creates new mail message object with custom From header.
func NewMessageFrom(to []string, fromDisplayName, fromAddress, subject, body string) *Message {
	log.Trace("NewMessageFrom (body):\n%s", body)

	return &Message{
		FromAddress:     fromAddress,
		FromDisplayName: fromDisplayName,
		To:              to,
		Subject:         subject,
		Date:            time.Now(),
		Body:            body,
		Headers:         map[string][]string{},
	}
}

// NewMessage creates new mail message object with default From header.
func NewMessage(to []string, subject, body string) *Message {
	return NewMessageFrom(to, setting.MailService.FromName, setting.MailService.FromEmail, subject, body)
}

type loginAuth struct {
	username, password string
}

// LoginAuth SMTP AUTH LOGIN Auth Handler
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

// Start start SMTP login auth
func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

// Next next step of SMTP login auth
func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, fmt.Errorf("unknown fromServer: %s", string(fromServer))
		}
	}
	return nil, nil
}

// Sender SMTP mail sender
type smtpSender struct {
}

// Send send email
func (s *smtpSender) Send(from string, to []string, msg io.WriterTo) error {
	opts := setting.MailService

	host, port, err := net.SplitHostPort(opts.Host)
	if err != nil {
		return err
	}

	tlsconfig := &tls.Config{
		InsecureSkipVerify: opts.SkipVerify,
		ServerName:         host,
	}

	if opts.UseCertificate {
		cert, err := tls.LoadX509KeyPair(opts.CertFile, opts.KeyFile)
		if err != nil {
			return err
		}
		tlsconfig.Certificates = []tls.Certificate{cert}
	}

	conn, err := net.Dial("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return err
	}
	defer conn.Close()

	isSecureConn := opts.IsTLSEnabled || (strings.HasSuffix(port, "465"))
	// Start TLS directly if the port ends with 465 (SMTPS protocol)
	if isSecureConn {
		conn = tls.Client(conn, tlsconfig)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("NewClient: %v", err)
	}

	if !opts.DisableHelo {
		hostname := opts.HeloHostname
		if len(hostname) == 0 {
			hostname, err = os.Hostname()
			if err != nil {
				return err
			}
		}

		if err = client.Hello(hostname); err != nil {
			return fmt.Errorf("Hello: %v", err)
		}
	}

	// If not using SMTPS, always use STARTTLS if available
	hasStartTLS, _ := client.Extension("STARTTLS")
	if !isSecureConn && hasStartTLS {
		if err = client.StartTLS(tlsconfig); err != nil {
			return fmt.Errorf("StartTLS: %v", err)
		}
	}

	canAuth, options := client.Extension("AUTH")
	if canAuth && len(opts.User) > 0 {
		var auth smtp.Auth

		if strings.Contains(options, "CRAM-MD5") {
			auth = smtp.CRAMMD5Auth(opts.User, opts.Passwd)
		} else if strings.Contains(options, "PLAIN") {
			auth = smtp.PlainAuth("", opts.User, opts.Passwd, host)
		} else if strings.Contains(options, "LOGIN") {
			// Patch for AUTH LOGIN
			auth = LoginAuth(opts.User, opts.Passwd)
		}

		if auth != nil {
			if err = client.Auth(auth); err != nil {
				return fmt.Errorf("Auth: %v", err)
			}
		}
	}

	if err = client.Mail(from); err != nil {
		return fmt.Errorf("Mail: %v", err)
	}

	for _, rec := range to {
		if err = client.Rcpt(rec); err != nil {
			return fmt.Errorf("Rcpt: %v", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("Data: %v", err)
	} else if _, err = msg.WriteTo(w); err != nil {
		return fmt.Errorf("WriteTo: %v", err)
	} else if err = w.Close(); err != nil {
		return fmt.Errorf("Close: %v", err)
	}

	return client.Quit()
}

// Sender sendmail mail sender
type sendmailSender struct {
}

// Send send email
func (s *sendmailSender) Send(from string, to []string, msg io.WriterTo) error {
	var err error
	var closeError error
	var waitError error

	args := []string{"-f", from, "-i"}
	args = append(args, setting.MailService.SendmailArgs...)
	args = append(args, to...)
	log.Trace("Sending with: %s %v", setting.MailService.SendmailPath, args)

	pm := process.GetManager()
	desc := fmt.Sprintf("SendMail: %s %v", setting.MailService.SendmailPath, args)

	ctx, cancel := context.WithTimeout(graceful.GetManager().HammerContext(), setting.MailService.SendmailTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, setting.MailService.SendmailPath, args...)
	pipe, err := cmd.StdinPipe()

	if err != nil {
		return err
	}

	if err = cmd.Start(); err != nil {
		return err
	}

	pid := pm.Add(desc, cancel)

	_, err = msg.WriteTo(pipe)

	// we MUST close the pipe or sendmail will hang waiting for more of the message
	// Also we should wait on our sendmail command even if something fails
	closeError = pipe.Close()
	waitError = cmd.Wait()
	pm.Remove(pid)
	if err != nil {
		return err
	} else if closeError != nil {
		return closeError
	} else {
		return waitError
	}
}

// Sender sendmail mail sender
type dummySender struct {
}

// Send send email
func (s *dummySender) Send(from string, to []string, msg io.WriterTo) error {
	buf := bytes.Buffer{}
	if _, err := msg.WriteTo(&buf); err != nil {
		return err
	}
	log.Info("Mail From: %s To: %v Body: %s", from, to, buf.String())
	return nil
}

var mailQueue queue.Queue

// Sender sender for sending mail synchronously
var Sender gomail.Sender

// NewContext start mail queue service
func NewContext() {
	// Need to check if mailQueue is nil because in during reinstall (user had installed
	// before but switched install lock off), this function will be called again
	// while mail queue is already processing tasks, and produces a race condition.
	if setting.MailService == nil || mailQueue != nil {
		return
	}

	switch setting.MailService.MailerType {
	case "smtp":
		Sender = &smtpSender{}
	case "sendmail":
		Sender = &sendmailSender{}
	case "dummy":
		Sender = &dummySender{}
	}

	mailQueue = queue.CreateQueue("mail", func(data ...queue.Data) {
		for _, datum := range data {
			msg := datum.(*Message)
			gomailMsg := msg.ToMessage()
			log.Trace("New e-mail sending request %s: %s", gomailMsg.GetHeader("To"), msg.Info)
			if err := gomail.Send(Sender, gomailMsg); err != nil {
				log.Error("Failed to send emails %s: %s - %v", gomailMsg.GetHeader("To"), msg.Info, err)
			} else {
				log.Trace("E-mails sent %s: %s", gomailMsg.GetHeader("To"), msg.Info)
			}
		}
	}, &Message{})

	go graceful.GetManager().RunWithShutdownFns(mailQueue.Run)
}

// SendAsync send mail asynchronously
func SendAsync(msg *Message) {
	SendAsyncs([]*Message{msg})
}

// SendAsyncs send mails asynchronously
func SendAsyncs(msgs []*Message) {
	if setting.MailService == nil {
		log.Error("Mailer: SendAsyncs is being invoked but mail service hasn't been initialized")
		return
	}

	go func() {
		for _, msg := range msgs {
			_ = mailQueue.Push(msg)
		}
	}()
}
