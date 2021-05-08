package commands

import (
	"github.com/emersion/go-imap"
)

// Noop is a NOOP command, as defined in RFC 3501 section 6.1.2.
type Noop struct{}

func (c *Noop) Command() *imap.Command {
	return &imap.Command{
		Name: "NOOP",
	}
}

func (c *Noop) Parse(fields []interface{}) error {
	return nil
}
