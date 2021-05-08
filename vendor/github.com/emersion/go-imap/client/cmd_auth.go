package client

import (
	"errors"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/commands"
	"github.com/emersion/go-imap/responses"
)

// ErrNotLoggedIn is returned if a function that requires the client to be
// logged in is called then the client isn't.
var ErrNotLoggedIn = errors.New("Not logged in")

func (c *Client) ensureAuthenticated() error {
	state := c.State()
	if state != imap.AuthenticatedState && state != imap.SelectedState {
		return ErrNotLoggedIn
	}
	return nil
}

// Select selects a mailbox so that messages in the mailbox can be accessed. Any
// currently selected mailbox is deselected before attempting the new selection.
// Even if the readOnly parameter is set to false, the server can decide to open
// the mailbox in read-only mode.
func (c *Client) Select(name string, readOnly bool) (*imap.MailboxStatus, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	cmd := &commands.Select{
		Mailbox:  name,
		ReadOnly: readOnly,
	}

	mbox := &imap.MailboxStatus{Name: name, Items: make(map[imap.StatusItem]interface{})}
	res := &responses.Select{
		Mailbox: mbox,
	}
	c.locker.Lock()
	c.mailbox = mbox
	c.locker.Unlock()

	status, err := c.execute(cmd, res)
	if err != nil {
		c.locker.Lock()
		c.mailbox = nil
		c.locker.Unlock()
		return nil, err
	}
	if err := status.Err(); err != nil {
		c.locker.Lock()
		c.mailbox = nil
		c.locker.Unlock()
		return nil, err
	}

	c.locker.Lock()
	mbox.ReadOnly = (status.Code == imap.CodeReadOnly)
	c.state = imap.SelectedState
	c.locker.Unlock()
	return mbox, nil
}

// Create creates a mailbox with the given name.
func (c *Client) Create(name string) error {
	if err := c.ensureAuthenticated(); err != nil {
		return err
	}

	cmd := &commands.Create{
		Mailbox: name,
	}

	status, err := c.execute(cmd, nil)
	if err != nil {
		return err
	}
	return status.Err()
}

// Delete permanently removes the mailbox with the given name.
func (c *Client) Delete(name string) error {
	if err := c.ensureAuthenticated(); err != nil {
		return err
	}

	cmd := &commands.Delete{
		Mailbox: name,
	}

	status, err := c.execute(cmd, nil)
	if err != nil {
		return err
	}
	return status.Err()
}

// Rename changes the name of a mailbox.
func (c *Client) Rename(existingName, newName string) error {
	if err := c.ensureAuthenticated(); err != nil {
		return err
	}

	cmd := &commands.Rename{
		Existing: existingName,
		New:      newName,
	}

	status, err := c.execute(cmd, nil)
	if err != nil {
		return err
	}
	return status.Err()
}

// Subscribe adds the specified mailbox name to the server's set of "active" or
// "subscribed" mailboxes.
func (c *Client) Subscribe(name string) error {
	if err := c.ensureAuthenticated(); err != nil {
		return err
	}

	cmd := &commands.Subscribe{
		Mailbox: name,
	}

	status, err := c.execute(cmd, nil)
	if err != nil {
		return err
	}
	return status.Err()
}

// Unsubscribe removes the specified mailbox name from the server's set of
// "active" or "subscribed" mailboxes.
func (c *Client) Unsubscribe(name string) error {
	if err := c.ensureAuthenticated(); err != nil {
		return err
	}

	cmd := &commands.Unsubscribe{
		Mailbox: name,
	}

	status, err := c.execute(cmd, nil)
	if err != nil {
		return err
	}
	return status.Err()
}

// List returns a subset of names from the complete set of all names available
// to the client.
//
// An empty name argument is a special request to return the hierarchy delimiter
// and the root name of the name given in the reference. The character "*" is a
// wildcard, and matches zero or more characters at this position. The
// character "%" is similar to "*", but it does not match a hierarchy delimiter.
func (c *Client) List(ref, name string, ch chan *imap.MailboxInfo) error {
	defer close(ch)

	if err := c.ensureAuthenticated(); err != nil {
		return err
	}

	cmd := &commands.List{
		Reference: ref,
		Mailbox:   name,
	}
	res := &responses.List{Mailboxes: ch}

	status, err := c.execute(cmd, res)
	if err != nil {
		return err
	}
	return status.Err()
}

// Lsub returns a subset of names from the set of names that the user has
// declared as being "active" or "subscribed".
func (c *Client) Lsub(ref, name string, ch chan *imap.MailboxInfo) error {
	defer close(ch)

	if err := c.ensureAuthenticated(); err != nil {
		return err
	}

	cmd := &commands.List{
		Reference:  ref,
		Mailbox:    name,
		Subscribed: true,
	}
	res := &responses.List{
		Mailboxes:  ch,
		Subscribed: true,
	}

	status, err := c.execute(cmd, res)
	if err != nil {
		return err
	}
	return status.Err()
}

// Status requests the status of the indicated mailbox. It does not change the
// currently selected mailbox, nor does it affect the state of any messages in
// the queried mailbox.
//
// See RFC 3501 section 6.3.10 for a list of items that can be requested.
func (c *Client) Status(name string, items []imap.StatusItem) (*imap.MailboxStatus, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}

	cmd := &commands.Status{
		Mailbox: name,
		Items:   items,
	}
	res := &responses.Status{
		Mailbox: new(imap.MailboxStatus),
	}

	status, err := c.execute(cmd, res)
	if err != nil {
		return nil, err
	}
	return res.Mailbox, status.Err()
}

// Append appends the literal argument as a new message to the end of the
// specified destination mailbox. This argument SHOULD be in the format of an
// RFC 2822 message. flags and date are optional arguments and can be set to
// nil.
func (c *Client) Append(mbox string, flags []string, date time.Time, msg imap.Literal) error {
	if err := c.ensureAuthenticated(); err != nil {
		return err
	}

	cmd := &commands.Append{
		Mailbox: mbox,
		Flags:   flags,
		Date:    date,
		Message: msg,
	}

	status, err := c.execute(cmd, nil)
	if err != nil {
		return err
	}
	return status.Err()
}
