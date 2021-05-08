package imap

import (
	"errors"
	"strings"
)

// A value that can be converted to a command.
type Commander interface {
	Command() *Command
}

// A command.
type Command struct {
	// The command tag. It acts as a unique identifier for this command. If empty,
	// the command is untagged.
	Tag string
	// The command name.
	Name string
	// The command arguments.
	Arguments []interface{}
}

// Implements the Commander interface.
func (cmd *Command) Command() *Command {
	return cmd
}

func (cmd *Command) WriteTo(w *Writer) error {
	tag := cmd.Tag
	if tag == "" {
		tag = "*"
	}

	fields := []interface{}{RawString(tag), RawString(cmd.Name)}
	fields = append(fields, cmd.Arguments...)
	return w.writeLine(fields...)
}

// Parse a command from fields.
func (cmd *Command) Parse(fields []interface{}) error {
	if len(fields) < 2 {
		return errors.New("imap: cannot parse command: no enough fields")
	}

	var ok bool
	if cmd.Tag, ok = fields[0].(string); !ok {
		return errors.New("imap: cannot parse command: invalid tag")
	}
	if cmd.Name, ok = fields[1].(string); !ok {
		return errors.New("imap: cannot parse command: invalid name")
	}
	cmd.Name = strings.ToUpper(cmd.Name) // Command names are case-insensitive

	cmd.Arguments = fields[2:]
	return nil
}
