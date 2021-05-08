// Copyright 2021 The Gitea Authors.
// All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package imap

import (
	"fmt"
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"github.com/emersion/go-imap"
	"github.com/stretchr/testify/assert"
)

var logs string

type testIMAPClient struct {
	// t *testing.T
}

func TestIMAP(t *testing.T) {
	// test GetUnReadMails
	logs = ""
	mails, err := c.GetUnReadMails(setting.MailReciveService.ReciveBox, 100)
	assert.NoError(t, err)
	if !assert.Equal(t, 2, len(mails)) {
		return
	}
	assert.Equal(t, "login: receive@gitea.io 123456\n"+
		"Select: INBOX, false\n"+
		"Search: [\\Seen]\n"+
		"logout\n", logs)

	// TODO
	// test handleReciveEmail
	// c.Client.(*testIMAPClient).t = t

	// for _, mail := range mails {
	// 	logs = ""
	// 	if !assert.NoError(t, mail.LoadHeader([]string{"From", "To", "In-Reply-To", "References"})) {
	// 		return
	// 	}
	// 	if !assert.NoError(t, handleReciveEmail(mail)) {
	// 		return
	// 	}
	// 	assert.Equal(t, "", logs)
	// }
}

func (c *testIMAPClient) Login(username, password string) error {
	logs += "login: " + username + " " + password + "\n"
	return nil
}

func (c *testIMAPClient) Logout() error {
	logs += "logout\n"
	return nil
}

func (c *testIMAPClient) Select(name string, readOnly bool) (*imap.MailboxStatus, error) {
	logs += fmt.Sprintf("Select: %v, %v\n", name, readOnly)
	if name != "INBOX" {
		return nil, fmt.Errorf("not found mail box")
	}

	return nil, nil
}

func (c *testIMAPClient) Search(criteria *imap.SearchCriteria) (seqNums []uint32, err error) {
	logs += fmt.Sprintf("Search: %v\n", criteria.WithoutFlags)

	return []uint32{1, 2}, nil
}

func (c *testIMAPClient) Store(seqset *imap.SeqSet, item imap.StoreItem, value interface{}, ch chan *imap.Message) error {
	logs += "Store\n"
	return nil
}

func (c *testIMAPClient) Expunge(ch chan uint32) error {
	logs += "Expunge\n"
	return nil
}

// var testMails []string

func (c *testIMAPClient) Fetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error {
	defer close(ch)

	// logs += fmt.Sprintf("Fetch: %v, %v\n", seqset.String(), items)
	// if len(seqset.Set) != 1 {
	// 	return nil
	// }
	// if seqset.Set[0].Start < 1 || seqset.Set[0].Start > 2 {
	// 	return nil
	// }

	// if testMails == nil {
	// 	// load test data
	// 	testMails = make([]string, 2)
	// 	issue1 := models.AssertExistsAndLoadBean(c.t, &models.Issue{ID: 1}).(*models.Issue)
	// 	// comment2 := models.AssertExistsAndLoadBean(c.t, &models.Comment{ID: 2}).(*models.Comment)
	// 	user2 := models.AssertExistsAndLoadBean(c.t, &models.User{ID: 2}).(*models.User)

	// 	testMails[0] = "From: " + user2.Email + "\r\n" +
	// 		"To: receive@gitea.io\r\n" +
	// 		"Subject: Re: " + issue1.Title + "\r\n" +
	// 		"Date: Wed, 11 May 2016 14:31:59 +0000\r\n" +
	// 		"Message-ID: <0000000@localhost/>\r\n" +
	// 		"" +
	// 		"Content-Type: text/plain\r\n" +
	// 		"\r\n" +
	// 		"test reply\r\n" +
	// 		"----- origin mail ------\r\n" +
	// 		issue1.Content
	// }

	return nil
}
