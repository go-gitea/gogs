package commands

import (
	"github.com/emersion/go-imap"
)

// Close is a CLOSE command, as defined in RFC 3501 section 6.4.2.
type Close struct{}

func (cmd *Close) Command() *imap.Command {
	return &imap.Command{
		Name: "CLOSE",
	}
}

func (cmd *Close) Parse(fields []interface{}) error {
	return nil
}
