package commands

import (
	"errors"
	"io"
	"strings"

	"github.com/emersion/go-imap"
)

// Search is a SEARCH command, as defined in RFC 3501 section 6.4.4.
type Search struct {
	Charset  string
	Criteria *imap.SearchCriteria
}

func (cmd *Search) Command() *imap.Command {
	var args []interface{}
	if cmd.Charset != "" {
		args = append(args, imap.RawString("CHARSET"), imap.RawString(cmd.Charset))
	}
	args = append(args, cmd.Criteria.Format()...)

	return &imap.Command{
		Name:      "SEARCH",
		Arguments: args,
	}
}

func (cmd *Search) Parse(fields []interface{}) error {
	if len(fields) == 0 {
		return errors.New("Missing search criteria")
	}

	// Parse charset
	if f, ok := fields[0].(string); ok && strings.EqualFold(f, "CHARSET") {
		if len(fields) < 2 {
			return errors.New("Missing CHARSET value")
		}
		if cmd.Charset, ok = fields[1].(string); !ok {
			return errors.New("Charset must be a string")
		}
		fields = fields[2:]
	}

	var charsetReader func(io.Reader) io.Reader
	charset := strings.ToLower(cmd.Charset)
	if charset != "utf-8" && charset != "us-ascii" && charset != "" {
		charsetReader = func(r io.Reader) io.Reader {
			r, _ = imap.CharsetReader(charset, r)
			return r
		}
	}

	cmd.Criteria = new(imap.SearchCriteria)
	return cmd.Criteria.ParseWithCharset(fields, charsetReader)
}
