package commands

import (
	"errors"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/utf7"
)

// Append is an APPEND command, as defined in RFC 3501 section 6.3.11.
type Append struct {
	Mailbox string
	Flags   []string
	Date    time.Time
	Message imap.Literal
}

func (cmd *Append) Command() *imap.Command {
	var args []interface{}

	mailbox, _ := utf7.Encoding.NewEncoder().String(cmd.Mailbox)
	args = append(args, imap.FormatMailboxName(mailbox))

	if cmd.Flags != nil {
		flags := make([]interface{}, len(cmd.Flags))
		for i, flag := range cmd.Flags {
			flags[i] = imap.RawString(flag)
		}
		args = append(args, flags)
	}

	if !cmd.Date.IsZero() {
		args = append(args, cmd.Date)
	}

	args = append(args, cmd.Message)

	return &imap.Command{
		Name:      "APPEND",
		Arguments: args,
	}
}

func (cmd *Append) Parse(fields []interface{}) (err error) {
	if len(fields) < 2 {
		return errors.New("No enough arguments")
	}

	// Parse mailbox name
	if mailbox, err := imap.ParseString(fields[0]); err != nil {
		return err
	} else if mailbox, err = utf7.Encoding.NewDecoder().String(mailbox); err != nil {
		return err
	} else {
		cmd.Mailbox = imap.CanonicalMailboxName(mailbox)
	}

	// Parse message literal
	litIndex := len(fields) - 1
	var ok bool
	if cmd.Message, ok = fields[litIndex].(imap.Literal); !ok {
		return errors.New("Message must be a literal")
	}

	// Remaining fields a optional
	fields = fields[1:litIndex]
	if len(fields) > 0 {
		// Parse flags list
		if flags, ok := fields[0].([]interface{}); ok {
			if cmd.Flags, err = imap.ParseStringList(flags); err != nil {
				return err
			}

			for i, flag := range cmd.Flags {
				cmd.Flags[i] = imap.CanonicalFlag(flag)
			}

			fields = fields[1:]
		}

		// Parse date
		if len(fields) > 0 {
			if date, ok := fields[0].(string); !ok {
				return errors.New("Date must be a string")
			} else if cmd.Date, err = time.Parse(imap.DateTimeLayout, date); err != nil {
				return err
			}
		}
	}

	return
}
