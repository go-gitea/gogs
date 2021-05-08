package commands

import (
	"errors"
	"strings"

	"github.com/emersion/go-imap"
)

// Uid is a UID command, as defined in RFC 3501 section 6.4.8. It wraps another
// command (e.g. wrapping a Fetch command will result in a UID FETCH).
type Uid struct {
	Cmd imap.Commander
}

func (cmd *Uid) Command() *imap.Command {
	inner := cmd.Cmd.Command()

	args := []interface{}{imap.RawString(inner.Name)}
	args = append(args, inner.Arguments...)

	return &imap.Command{
		Name:      "UID",
		Arguments: args,
	}
}

func (cmd *Uid) Parse(fields []interface{}) error {
	if len(fields) < 0 {
		return errors.New("No command name specified")
	}

	name, ok := fields[0].(string)
	if !ok {
		return errors.New("Command name must be a string")
	}

	cmd.Cmd = &imap.Command{
		Name:      strings.ToUpper(name), // Command names are case-insensitive
		Arguments: fields[1:],
	}

	return nil
}
