package commands

import (
	"github.com/emersion/go-imap"
)

// Check is a CHECK command, as defined in RFC 3501 section 6.4.1.
type Check struct{}

func (cmd *Check) Command() *imap.Command {
	return &imap.Command{
		Name: "CHECK",
	}
}

func (cmd *Check) Parse(fields []interface{}) error {
	return nil
}
