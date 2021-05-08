package commands

import (
	"errors"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/utf7"
)

// Status is a STATUS command, as defined in RFC 3501 section 6.3.10.
type Status struct {
	Mailbox string
	Items   []imap.StatusItem
}

func (cmd *Status) Command() *imap.Command {
	mailbox, _ := utf7.Encoding.NewEncoder().String(cmd.Mailbox)

	items := make([]interface{}, len(cmd.Items))
	for i, item := range cmd.Items {
		items[i] = imap.RawString(item)
	}

	return &imap.Command{
		Name:      "STATUS",
		Arguments: []interface{}{imap.FormatMailboxName(mailbox), items},
	}
}

func (cmd *Status) Parse(fields []interface{}) error {
	if len(fields) < 2 {
		return errors.New("No enough arguments")
	}

	if mailbox, err := imap.ParseString(fields[0]); err != nil {
		return err
	} else if mailbox, err := utf7.Encoding.NewDecoder().String(mailbox); err != nil {
		return err
	} else {
		cmd.Mailbox = imap.CanonicalMailboxName(mailbox)
	}

	items, ok := fields[1].([]interface{})
	if !ok {
		return errors.New("STATUS command parameter is not a list")
	}
	cmd.Items = make([]imap.StatusItem, len(items))
	for i, f := range items {
		if s, ok := f.(string); !ok {
			return errors.New("Got a non-string field in a STATUS command parameter")
		} else {
			cmd.Items[i] = imap.StatusItem(strings.ToUpper(s))
		}
	}

	return nil
}
