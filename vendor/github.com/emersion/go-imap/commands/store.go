package commands

import (
	"errors"
	"strings"

	"github.com/emersion/go-imap"
)

// Store is a STORE command, as defined in RFC 3501 section 6.4.6.
type Store struct {
	SeqSet *imap.SeqSet
	Item   imap.StoreItem
	Value  interface{}
}

func (cmd *Store) Command() *imap.Command {
	return &imap.Command{
		Name:      "STORE",
		Arguments: []interface{}{cmd.SeqSet, imap.RawString(cmd.Item), cmd.Value},
	}
}

func (cmd *Store) Parse(fields []interface{}) error {
	if len(fields) < 3 {
		return errors.New("No enough arguments")
	}

	seqset, ok := fields[0].(string)
	if !ok {
		return errors.New("Invalid sequence set")
	}
	var err error
	if cmd.SeqSet, err = imap.ParseSeqSet(seqset); err != nil {
		return err
	}

	if item, ok := fields[1].(string); !ok {
		return errors.New("Item name must be a string")
	} else {
		cmd.Item = imap.StoreItem(strings.ToUpper(item))
	}

	if len(fields[2:]) == 1 {
		cmd.Value = fields[2]
	} else {
		cmd.Value = fields[2:]
	}
	return nil
}
