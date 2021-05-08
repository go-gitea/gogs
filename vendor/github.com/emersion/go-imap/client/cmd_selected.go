package client

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/commands"
	"github.com/emersion/go-imap/responses"
)

// ErrNoMailboxSelected is returned if a command that requires a mailbox to be
// selected is called when there isn't.
var ErrNoMailboxSelected = errors.New("No mailbox selected")

// Check requests a checkpoint of the currently selected mailbox. A checkpoint
// refers to any implementation-dependent housekeeping associated with the
// mailbox that is not normally executed as part of each command.
func (c *Client) Check() error {
	if c.State() != imap.SelectedState {
		return ErrNoMailboxSelected
	}

	cmd := new(commands.Check)

	status, err := c.execute(cmd, nil)
	if err != nil {
		return err
	}

	return status.Err()
}

// Close permanently removes all messages that have the \Deleted flag set from
// the currently selected mailbox, and returns to the authenticated state from
// the selected state.
func (c *Client) Close() error {
	if c.State() != imap.SelectedState {
		return ErrNoMailboxSelected
	}

	cmd := new(commands.Close)

	status, err := c.execute(cmd, nil)
	if err != nil {
		return err
	} else if err := status.Err(); err != nil {
		return err
	}

	c.locker.Lock()
	c.state = imap.AuthenticatedState
	c.mailbox = nil
	c.locker.Unlock()
	return nil
}

// Terminate closes the tcp connection
func (c *Client) Terminate() error {
	return c.conn.Close()
}

// Expunge permanently removes all messages that have the \Deleted flag set from
// the currently selected mailbox. If ch is not nil, sends sequence IDs of each
// deleted message to this channel.
func (c *Client) Expunge(ch chan uint32) error {
	if ch != nil {
		defer close(ch)
	}

	if c.State() != imap.SelectedState {
		return ErrNoMailboxSelected
	}

	cmd := new(commands.Expunge)

	var h responses.Handler
	if ch != nil {
		h = &responses.Expunge{SeqNums: ch}
	}

	status, err := c.execute(cmd, h)
	if err != nil {
		return err
	}
	return status.Err()
}

func (c *Client) executeSearch(uid bool, criteria *imap.SearchCriteria, charset string) (ids []uint32, status *imap.StatusResp, err error) {
	if c.State() != imap.SelectedState {
		err = ErrNoMailboxSelected
		return
	}

	var cmd imap.Commander = &commands.Search{
		Charset:  charset,
		Criteria: criteria,
	}
	if uid {
		cmd = &commands.Uid{Cmd: cmd}
	}

	res := new(responses.Search)

	status, err = c.execute(cmd, res)
	if err != nil {
		return
	}

	err, ids = status.Err(), res.Ids
	return
}

func (c *Client) search(uid bool, criteria *imap.SearchCriteria) (ids []uint32, err error) {
	ids, status, err := c.executeSearch(uid, criteria, "UTF-8")
	if status != nil && status.Code == imap.CodeBadCharset {
		// Some servers don't support UTF-8
		ids, _, err = c.executeSearch(uid, criteria, "US-ASCII")
	}
	return
}

// Search searches the mailbox for messages that match the given searching
// criteria. Searching criteria consist of one or more search keys. The response
// contains a list of message sequence IDs corresponding to those messages that
// match the searching criteria. When multiple keys are specified, the result is
// the intersection (AND function) of all the messages that match those keys.
// Criteria must be UTF-8 encoded. See RFC 3501 section 6.4.4 for a list of
// searching criteria. When no criteria has been set, all messages in the mailbox
// will be searched using ALL criteria.
func (c *Client) Search(criteria *imap.SearchCriteria) (seqNums []uint32, err error) {
	return c.search(false, criteria)
}

// UidSearch is identical to Search, but UIDs are returned instead of message
// sequence numbers.
func (c *Client) UidSearch(criteria *imap.SearchCriteria) (uids []uint32, err error) {
	return c.search(true, criteria)
}

func (c *Client) fetch(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error {
	defer close(ch)

	if c.State() != imap.SelectedState {
		return ErrNoMailboxSelected
	}

	var cmd imap.Commander = &commands.Fetch{
		SeqSet: seqset,
		Items:  items,
	}
	if uid {
		cmd = &commands.Uid{Cmd: cmd}
	}

	res := &responses.Fetch{Messages: ch}

	status, err := c.execute(cmd, res)
	if err != nil {
		return err
	}
	return status.Err()
}

// Fetch retrieves data associated with a message in the mailbox. See RFC 3501
// section 6.4.5 for a list of items that can be requested.
func (c *Client) Fetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error {
	return c.fetch(false, seqset, items, ch)
}

// UidFetch is identical to Fetch, but seqset is interpreted as containing
// unique identifiers instead of message sequence numbers.
func (c *Client) UidFetch(seqset *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error {
	return c.fetch(true, seqset, items, ch)
}

func (c *Client) store(uid bool, seqset *imap.SeqSet, item imap.StoreItem, value interface{}, ch chan *imap.Message) error {
	if ch != nil {
		defer close(ch)
	}

	if c.State() != imap.SelectedState {
		return ErrNoMailboxSelected
	}

	// TODO: this could break extensions (this only works when item is FLAGS)
	if fields, ok := value.([]interface{}); ok {
		for i, field := range fields {
			if s, ok := field.(string); ok {
				fields[i] = imap.RawString(s)
			}
		}
	}

	// If ch is nil, the updated values are data which will be lost, so don't
	// retrieve it.
	if ch == nil {
		op, _, err := imap.ParseFlagsOp(item)
		if err == nil {
			item = imap.FormatFlagsOp(op, true)
		}
	}

	var cmd imap.Commander = &commands.Store{
		SeqSet: seqset,
		Item:   item,
		Value:  value,
	}
	if uid {
		cmd = &commands.Uid{Cmd: cmd}
	}

	var h responses.Handler
	if ch != nil {
		h = &responses.Fetch{Messages: ch}
	}

	status, err := c.execute(cmd, h)
	if err != nil {
		return err
	}
	return status.Err()
}

// Store alters data associated with a message in the mailbox. If ch is not nil,
// the updated value of the data will be sent to this channel. See RFC 3501
// section 6.4.6 for a list of items that can be updated.
func (c *Client) Store(seqset *imap.SeqSet, item imap.StoreItem, value interface{}, ch chan *imap.Message) error {
	return c.store(false, seqset, item, value, ch)
}

// UidStore is identical to Store, but seqset is interpreted as containing
// unique identifiers instead of message sequence numbers.
func (c *Client) UidStore(seqset *imap.SeqSet, item imap.StoreItem, value interface{}, ch chan *imap.Message) error {
	return c.store(true, seqset, item, value, ch)
}

func (c *Client) copy(uid bool, seqset *imap.SeqSet, dest string) error {
	if c.State() != imap.SelectedState {
		return ErrNoMailboxSelected
	}

	var cmd imap.Commander = &commands.Copy{
		SeqSet:  seqset,
		Mailbox: dest,
	}
	if uid {
		cmd = &commands.Uid{Cmd: cmd}
	}

	status, err := c.execute(cmd, nil)
	if err != nil {
		return err
	}
	return status.Err()
}

// Copy copies the specified message(s) to the end of the specified destination
// mailbox.
func (c *Client) Copy(seqset *imap.SeqSet, dest string) error {
	return c.copy(false, seqset, dest)
}

// UidCopy is identical to Copy, but seqset is interpreted as containing unique
// identifiers instead of message sequence numbers.
func (c *Client) UidCopy(seqset *imap.SeqSet, dest string) error {
	return c.copy(true, seqset, dest)
}
