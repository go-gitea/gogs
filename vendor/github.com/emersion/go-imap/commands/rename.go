package commands

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/utf7"
)

// Rename is a RENAME command, as defined in RFC 3501 section 6.3.5.
type Rename struct {
	Existing string
	New      string
}

func (cmd *Rename) Command() *imap.Command {
	enc := utf7.Encoding.NewEncoder()
	existingName, _ := enc.String(cmd.Existing)
	newName, _ := enc.String(cmd.New)

	return &imap.Command{
		Name:      "RENAME",
		Arguments: []interface{}{imap.FormatMailboxName(existingName), imap.FormatMailboxName(newName)},
	}
}

func (cmd *Rename) Parse(fields []interface{}) error {
	if len(fields) < 2 {
		return errors.New("No enough arguments")
	}

	dec := utf7.Encoding.NewDecoder()

	if existingName, err := imap.ParseString(fields[0]); err != nil {
		return err
	} else if existingName, err := dec.String(existingName); err != nil {
		return err
	} else {
		cmd.Existing = imap.CanonicalMailboxName(existingName)
	}

	if newName, err := imap.ParseString(fields[1]); err != nil {
		return err
	} else if newName, err := dec.String(newName); err != nil {
		return err
	} else {
		cmd.New = imap.CanonicalMailboxName(newName)
	}

	return nil
}
