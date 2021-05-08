package commands

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/utf7"
)

// Copy is a COPY command, as defined in RFC 3501 section 6.4.7.
type Copy struct {
	SeqSet  *imap.SeqSet
	Mailbox string
}

func (cmd *Copy) Command() *imap.Command {
	mailbox, _ := utf7.Encoding.NewEncoder().String(cmd.Mailbox)

	return &imap.Command{
		Name:      "COPY",
		Arguments: []interface{}{cmd.SeqSet, imap.FormatMailboxName(mailbox)},
	}
}

func (cmd *Copy) Parse(fields []interface{}) error {
	if len(fields) < 2 {
		return errors.New("No enough arguments")
	}

	if seqSet, ok := fields[0].(string); !ok {
		return errors.New("Invalid sequence set")
	} else if seqSet, err := imap.ParseSeqSet(seqSet); err != nil {
		return err
	} else {
		cmd.SeqSet = seqSet
	}

	if mailbox, err := imap.ParseString(fields[1]); err != nil {
		return err
	} else if mailbox, err := utf7.Encoding.NewDecoder().String(mailbox); err != nil {
		return err
	} else {
		cmd.Mailbox = imap.CanonicalMailboxName(mailbox)
	}

	return nil
}
