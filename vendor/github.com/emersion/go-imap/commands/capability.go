package commands

import (
	"github.com/emersion/go-imap"
)

// Capability is a CAPABILITY command, as defined in RFC 3501 section 6.1.1.
type Capability struct{}

func (c *Capability) Command() *imap.Command {
	return &imap.Command{
		Name: "CAPABILITY",
	}
}

func (c *Capability) Parse(fields []interface{}) error {
	return nil
}
