// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package imap

import (
	"context"
	"errors"
	"io"
	"io/ioutil"
	"mime"
	net_mail "net/mail"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/log"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func init() {
	charset.RegisterEncoding("gb18030", simplifiedchinese.GB18030)
}

var testMode bool

// Client an imap clientor
type Client struct {
	Client   ClientPort
	UserName string
	Passwd   string
	Addr     string
	IsTLS    bool
	Lock     sync.Mutex
}

// ClientPort client port to imap server or test code
type ClientPort interface {
	Login(username, password string) error
	Logout() error
	Select(name string, readOnly bool) (*imap.MailboxStatus, error)
	Search(criteria *imap.SearchCriteria) (seqNums []uint32, err error)
	Store(seqset *imap.SeqSet, item imap.StoreItem, value interface{}, ch chan *imap.Message) error
	Expunge(ch chan uint32) error
	Fetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error
}

// ClientInitOpt options to init an Client
type ClientInitOpt struct {
	Addr     string
	UserName string
	Passwd   string
	IsTLS    bool
}

// NewImapClient init a imap Client
func NewImapClient(opt ClientInitOpt) (c *Client, err error) {
	c = new(Client)

	c.UserName = opt.UserName
	c.Passwd = opt.Passwd
	c.Addr = opt.Addr
	c.IsTLS = opt.IsTLS

	// try login
	if err = c.Login(); err != nil {
		return nil, err
	}

	if err = c.LogOut(); err != nil {
		return nil, err
	}

	return c, nil
}

// Login login to service
func (c *Client) Login() error {
	var err error

	c.Lock.Lock()

	// test mode
	if testMode {
		return c.Client.Login(c.UserName, c.Passwd)
	}

	// Connect to server
	if c.IsTLS {
		c.Client, err = client.DialTLS(c.Addr, nil)
	} else {
		c.Client, err = client.Dial(c.Addr)
	}
	if err != nil {
		return err
	}

	return c.Client.Login(c.UserName, c.Passwd)
}

// LogOut LogOut from service
func (c *Client) LogOut() error {
	err := c.Client.Logout()
	c.Lock.Unlock()
	return err
}

// GetUnReadMailIDs get all unread mails
func (c *Client) GetUnReadMailIDs(mailBox string) ([]uint32, error) {
	if err := c.Login(); err != nil {
		return nil, err
	}
	defer func() {
		err := c.LogOut()
		if err != nil {
			log.Warn("Imap.Logout", err)
		}
	}()

	if len(mailBox) == 0 {
		mailBox = "INBOX"
	}

	// Select mail box
	_, err := c.Client.Select(mailBox, false)
	if err != nil {
		return nil, err
	}

	// Set search criteria
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}
	ids, err := c.Client.Search(criteria)
	if err != nil {
		return nil, err
	}

	return ids, err
}

// Store store status
func (c *Client) Store(mailBox string, mID uint32, isAdd bool, flags []interface{}) error {
	if err := c.Login(); err != nil {
		return err
	}
	defer func() {
		err := c.LogOut()
		if err != nil {
			log.Warn("Imap.Logout", err)
		}
	}()

	return c.store(mailBox, mID, isAdd, flags)
}

// store store status without login
func (c *Client) store(mailBox string, mID uint32, isAdd bool, flags []interface{}) error {
	if len(mailBox) == 0 {
		mailBox = "INBOX"
	}

	// Select INBOX
	_, err := c.Client.Select(mailBox, false)
	if err != nil {
		return err
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(mID)

	var opt imap.FlagsOp
	if isAdd {
		opt = imap.AddFlags
	} else {
		opt = imap.RemoveFlags
	}

	item := imap.FormatFlagsOp(opt, true)

	return c.Client.Store(seqSet, item, flags, nil)
}

// DeleteMail delete one mail
func (c *Client) DeleteMail(mailBox string, mID uint32) error {
	if err := c.Login(); err != nil {
		return err
	}
	defer func() {
		err := c.LogOut()
		if err != nil {
			log.Warn("Imap.Logout", err)
		}
	}()

	// First mark the message as deleted
	if err := c.store(mailBox, mID, true, []interface{}{imap.DeletedFlag}); err != nil {
		return err
	}

	// Then delete it
	err := c.Client.Expunge(nil)
	return err
}

// FetchMail fetch a mail
func (c *Client) FetchMail(id uint32, box string, requestBody bool) (*mail.Reader, error) {
	var err error

	if err = c.Login(); err != nil {
		return nil, err
	}
	defer func() {
		err := c.LogOut()
		if err != nil {
			log.Warn("Imap.Logout", err)
		}
	}()

	if len(box) == 0 {
		box = "INBOX"
	}

	// Select mail box
	_, err = c.Client.Select(box, false)
	if err != nil {
		return nil, err
	}

	seqSet := new(imap.SeqSet)
	seqSet.AddNum(id)

	var section = imap.BodySectionName{}
	if !requestBody {
		section.BodyPartName.Specifier = imap.HeaderSpecifier
	}
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	finished := false

	go func() {
		err = c.Client.Fetch(seqSet, items, messages)
		finished = true
		cancel()
	}()

	var msg *imap.Message
	for !finished {
		select {
		case msg = <-messages:
			if msg != nil {
				if !finished {
					close(messages)
				}
				break
			}
		case <-ctx.Done():
			if !finished {
				close(messages)
			}
			break
		}
	}

	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, errors.New("Server didn't returned message")
	}

	r := msg.GetBody(&section)
	if r == nil {
		return nil, errors.New("Server didn't returned message body")
	}

	// Create a new mail reader
	mr, err := mail.CreateReader(r)
	if err != nil {
		return nil, err
	}

	return mr, nil
}

// Mail save an mail data
type Mail struct {
	Client *Client
	ID     uint32
	Box    string

	// header
	Date        time.Time
	Heads       map[string][]*mail.Address
	SimpleHeads map[string]string

	// body
	ContentText string

	Deleted bool
}

// GetUnReadMails get all unread mails
func (c *Client) GetUnReadMails(mailBox string, limit int) ([]*Mail, error) {
	ids, err := c.GetUnReadMailIDs(mailBox)
	if err != nil {
		return nil, err
	}

	last := len(ids)
	if last > limit {
		last = limit
	}

	mails := make([]*Mail, last)
	for index, id := range ids[0:last] {
		mails[index] = &Mail{
			ID:     id,
			Client: c,
			Box:    mailBox,
		}
	}

	return mails, nil
}

// LoadHeader load Head data
func (m *Mail) LoadHeader(requestHeads []string) error {
	mr, err := m.Client.FetchMail(m.ID, m.Box, false)
	if err != nil {
		return err
	}
	defer mr.Close()

	m.Date, err = mr.Header.Date()
	if err != nil {
		return err
	}

	if m.Heads == nil {
		m.Heads = make(map[string][]*mail.Address)
	}

	if m.SimpleHeads == nil {
		m.SimpleHeads = make(map[string]string)
	}

	var addrs []*mail.Address
	for _, head := range requestHeads {
		if addrs, err = mr.Header.AddressList(head); err != nil {
			if !strings.HasPrefix(err.Error(), "mail:") {
				return err
			}
		}

		if err == nil {
			m.Heads[head] = addrs
			continue
		}

		if !strings.Contains(err.Error(), "expected comma") {
			// It's not address list, get it as simple heads
			m.Heads[head] = nil
			m.SimpleHeads[head] = mr.Header.Get(head)
			continue
		}

		// try to fetch "<aa@bb.com> <bb@cc.com>" or
		//  aa@bb.com bb@cc.com style email list
		splitMails := strings.Split(mr.Header.Get(head), " ")
		if len(splitMails) == 0 {
			m.Heads[head] = nil
			m.SimpleHeads[head] = mr.Header.Get(head)
			continue
		}

		parser := net_mail.AddressParser{
			WordDecoder: &mime.WordDecoder{
				CharsetReader: message.CharsetReader,
			},
		}
		addrs = make([]*mail.Address, 0, len(splitMails))
		for _, addrString := range splitMails {
			var addr *net_mail.Address
			addr, err = parser.Parse(addrString)
			if err != nil {
				continue
			}
			addrs = append(addrs, (*mail.Address)(addr))
		}

		m.Heads[head] = addrs
	}

	return nil
}

// LoadBody load body data
func (m *Mail) LoadBody() error {
	mr, err := m.Client.FetchMail(m.ID, m.Box, true)
	if err != nil {
		return err
	}
	defer mr.Close()

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		var (
			header *mail.InlineHeader
			ok     bool
		)
		if header, ok = p.Header.(*mail.InlineHeader); !ok {
			continue
		}

		var contentType string
		contentType, _, err = header.ContentType()
		if err != nil {
			return err
		}
		if contentType == "text/plain" {
			if len(m.ContentText) != 0 {
				continue
			}

			content, err := ioutil.ReadAll(p.Body)
			if err != nil {
				return err
			}

			m.ContentText = string(content)
			return nil
		}

		// case *mail.AttachmentHeader:
		// TODO: how to handle attachment
		// This is an attachment
		// filename, err := h.Filename()
		// if err != nil {

		// }
	}

	return nil
}

// SetRead set read status
func (m *Mail) SetRead(isRead bool) error {
	return m.Client.Store(m.Box, m.ID, isRead, []interface{}{imap.SeenFlag})
}

// Delete delet this mail
func (m *Mail) Delete() error {
	if m.Deleted {
		return nil
	}
	err := m.Client.DeleteMail(m.Box, m.ID)
	if err != nil {
		return err
	}
	m.Deleted = true

	return nil
}
