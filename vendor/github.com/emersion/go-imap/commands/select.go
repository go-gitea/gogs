package commands

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/utf7"
)

// Select is a SELECT command, as defined in RFC 3501 section 6.3.1. If ReadOnly
// is set to true, the EXAMINE command will be used instead.
type Select struct {
	Mailbox  string
	ReadOnly bool
}

func (cmd *Select) Command() *imap.Command {
	name := "SELECT"
	if cmd.ReadOnly {
		name = "EXAMINE"
	}

	mailbox, _ := utf7.Encoding.NewEncoder().String(cmd.Mailbox)

	return &imap.Command{
		Name:      name,
		Arguments: []interface{}{imap.FormatMailboxName(mailbox)},
	}
}

func (cmd *Select) Parse(fields []interface{}) error {
	if len(fields) < 1 {
		return errors.New("No enough arguments")
	}

	if mailbox, err := imap.ParseString(fields[0]); err != nil {
		return err
	} else if mailbox, err := utf7.Encoding.NewDecoder().String(mailbox); err != nil {
		return err
	} else {
		cmd.Mailbox = imap.CanonicalMailboxName(mailbox)
	}

	return nil
}
