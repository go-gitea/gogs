package commands

import (
	"github.com/emersion/go-imap"
)

// Expunge is an EXPUNGE command, as defined in RFC 3501 section 6.4.3.
type Expunge struct{}

func (cmd *Expunge) Command() *imap.Command {
	return &imap.Command{Name: "EXPUNGE"}
}

func (cmd *Expunge) Parse(fields []interface{}) error {
	return nil
}
