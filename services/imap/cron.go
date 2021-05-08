// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package imap

import (
	"code.gitea.io/gitea/modules/setting"
)

var (
	c *Client
)

// FetchAllUnReadMails fetch all unread mails
func FetchAllUnReadMails() (err error) {
	if c == nil {
		c, err = NewImapClient(ClientInitOpt{
			Addr:     setting.MailReciveService.Host,
			UserName: setting.MailReciveService.User,
			Passwd:   setting.MailReciveService.Passwd,
			IsTLS:    setting.MailReciveService.IsTLSEnabled,
		})
		if err != nil {
			return
		}
	}

	if mailReadQueue != nil && !mailReadQueue.IsEmpty() {
		return
	}

	mails, err := c.GetUnReadMails(setting.MailReciveService.ReciveBox, 100)
	if err != nil {
		return
	}

	for _, mail := range mails {
		err = mailReadQueue.Push(mail)
		if err != nil {
			return err
		}
	}

	return nil
}
