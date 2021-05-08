package commands

import (
	"errors"

	"github.com/emersion/go-imap"
)

// Login is a LOGIN command, as defined in RFC 3501 section 6.2.2.
type Login struct {
	Username string
	Password string
}

func (cmd *Login) Command() *imap.Command {
	return &imap.Command{
		Name:      "LOGIN",
		Arguments: []interface{}{cmd.Username, cmd.Password},
	}
}

func (cmd *Login) Parse(fields []interface{}) error {
	if len(fields) < 2 {
		return errors.New("Not enough arguments")
	}

	var err error
	if cmd.Username, err = imap.ParseString(fields[0]); err != nil {
		return err
	}
	if cmd.Password, err = imap.ParseString(fields[1]); err != nil {
		return err
	}

	return nil
}
