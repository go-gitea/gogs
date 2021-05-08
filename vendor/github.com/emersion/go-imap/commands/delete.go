package commands

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/utf7"
)

// Delete is a DELETE command, as defined in RFC 3501 section 6.3.3.
type Delete struct {
	Mailbox string
}

func (cmd *Delete) Command() *imap.Command {
	mailbox, _ := utf7.Encoding.NewEncoder().String(cmd.Mailbox)

	return &imap.Command{
		Name:      "DELETE",
		Arguments: []interface{}{imap.FormatMailboxName(mailbox)},
	}
}

func (cmd *Delete) Parse(fields []interface{}) error {
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
