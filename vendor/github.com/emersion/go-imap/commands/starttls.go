package commands

import (
	"github.com/emersion/go-imap"
)

// StartTLS is a STARTTLS command, as defined in RFC 3501 section 6.2.1.
type StartTLS struct{}

func (cmd *StartTLS) Command() *imap.Command {
	return &imap.Command{
		Name: "STARTTLS",
	}
}

func (cmd *StartTLS) Parse(fields []interface{}) error {
	return nil
}
