package imap

import (
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strings"
	"time"
)

func maybeString(mystery interface{}) string {
	if s, ok := mystery.(string); ok {
		return s
	}
	return ""
}

func convertField(f interface{}, charsetReader func(io.Reader) io.Reader) string {
	// An IMAP string contains only 7-bit data, no need to decode it
	if s, ok := f.(string); ok {
		return s
	}

	// If no charset is provided, getting directly the string is faster
	if charsetReader == nil {
		if stringer, ok := f.(fmt.Stringer); ok {
			return stringer.String()
		}
	}

	// Not a string, it must be a literal
	l, ok := f.(Literal)
	if !ok {
		return ""
	}

	var r io.Reader = l
	if charsetReader != nil {
		if dec := charsetReader(r); dec != nil {
			r = dec
		}
	}

	b := make([]byte, l.Len())
	if _, err := io.ReadFull(r, b); err != nil {
		return ""
	}
	return string(b)
}

func popSearchField(fields []interface{}) (interface{}, []interface{}, error) {
	if len(fields) == 0 {
		return nil, nil, errors.New("imap: no enough fields for search key")
	}
	return fields[0], fields[1:], nil
}

// SearchCriteria is a search criteria. A message matches the criteria if and
// only if it matches each one of its fields.
type SearchCriteria struct {
	SeqNum *SeqSet // Sequence number is in sequence set
	Uid    *SeqSet // UID is in sequence set

	// Time and timezone are ignored
	Since      time.Time // Internal date is since this date
	Before     time.Time // Internal date is before this date
	SentSince  time.Time // Date header field is since this date
	SentBefore time.Time // Date header field is before this date

	Header textproto.MIMEHeader // Each header field value is present
	Body   []string             // Each string is in the body
	Text   []string             // Each string is in the text (header + body)

	WithFlags    []string // Each flag is present
	WithoutFlags []string // Each flag is not present

	Larger  uint32 // Size is larger than this number
	Smaller uint32 // Size is smaller than this number

	Not []*SearchCriteria    // Each criteria doesn't match
	Or  [][2]*SearchCriteria // Each criteria pair has at least one match of two
}

// NewSearchCriteria creates a new search criteria.
func NewSearchCriteria() *SearchCriteria {
	return &SearchCriteria{Header: make(textproto.MIMEHeader)}
}

func (c *SearchCriteria) parseField(fields []interface{}, charsetReader func(io.Reader) io.Reader) ([]interface{}, error) {
	if len(fields) == 0 {
		return nil, nil
	}

	f := fields[0]
	fields = fields[1:]

	if subfields, ok := f.([]interface{}); ok {
		return fields, c.ParseWithCharset(subfields, charsetReader)
	}

	key, ok := f.(string)
	if !ok {
		return nil, fmt.Errorf("imap: invalid search criteria field type: %T", f)
	}
	key = strings.ToUpper(key)

	var err error
	switch key {
	case "ALL":
		// Nothing to do
	case "ANSWERED", "DELETED", "DRAFT", "FLAGGED", "RECENT", "SEEN":
		c.WithFlags = append(c.WithFlags, CanonicalFlag("\\"+key))
	case "BCC", "CC", "FROM", "SUBJECT", "TO":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		}
		if c.Header == nil {
			c.Header = make(textproto.MIMEHeader)
		}
		c.Header.Add(key, convertField(f, charsetReader))
	case "BEFORE":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else if t, err := time.Parse(DateLayout, maybeString(f)); err != nil {
			return nil, err
		} else if c.Before.IsZero() || t.Before(c.Before) {
			c.Before = t
		}
	case "BODY":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else {
			c.Body = append(c.Body, convertField(f, charsetReader))
		}
	case "HEADER":
		var f1, f2 interface{}
		if f1, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else if f2, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else {
			if c.Header == nil {
				c.Header = make(textproto.MIMEHeader)
			}
			c.Header.Add(maybeString(f1), convertField(f2, charsetReader))
		}
	case "KEYWORD":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else {
			c.WithFlags = append(c.WithFlags, CanonicalFlag(maybeString(f)))
		}
	case "LARGER":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else if n, err := ParseNumber(f); err != nil {
			return nil, err
		} else if c.Larger == 0 || n > c.Larger {
			c.Larger = n
		}
	case "NEW":
		c.WithFlags = append(c.WithFlags, RecentFlag)
		c.WithoutFlags = append(c.WithoutFlags, SeenFlag)
	case "NOT":
		not := new(SearchCriteria)
		if fields, err = not.parseField(fields, charsetReader); err != nil {
			return nil, err
		}
		c.Not = append(c.Not, not)
	case "OLD":
		c.WithoutFlags = append(c.WithoutFlags, RecentFlag)
	case "ON":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else if t, err := time.Parse(DateLayout, maybeString(f)); err != nil {
			return nil, err
		} else {
			c.Since = t
			c.Before = t.Add(24 * time.Hour)
		}
	case "OR":
		c1, c2 := new(SearchCriteria), new(SearchCriteria)
		if fields, err = c1.parseField(fields, charsetReader); err != nil {
			return nil, err
		} else if fields, err = c2.parseField(fields, charsetReader); err != nil {
			return nil, err
		}
		c.Or = append(c.Or, [2]*SearchCriteria{c1, c2})
	case "SENTBEFORE":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else if t, err := time.Parse(DateLayout, maybeString(f)); err != nil {
			return nil, err
		} else if c.SentBefore.IsZero() || t.Before(c.SentBefore) {
			c.SentBefore = t
		}
	case "SENTON":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else if t, err := time.Parse(DateLayout, maybeString(f)); err != nil {
			return nil, err
		} else {
			c.SentSince = t
			c.SentBefore = t.Add(24 * time.Hour)
		}
	case "SENTSINCE":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else if t, err := time.Parse(DateLayout, maybeString(f)); err != nil {
			return nil, err
		} else if c.SentSince.IsZero() || t.After(c.SentSince) {
			c.SentSince = t
		}
	case "SINCE":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else if t, err := time.Parse(DateLayout, maybeString(f)); err != nil {
			return nil, err
		} else if c.Since.IsZero() || t.After(c.Since) {
			c.Since = t
		}
	case "SMALLER":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else if n, err := ParseNumber(f); err != nil {
			return nil, err
		} else if c.Smaller == 0 || n < c.Smaller {
			c.Smaller = n
		}
	case "TEXT":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else {
			c.Text = append(c.Text, convertField(f, charsetReader))
		}
	case "UID":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else if c.Uid, err = ParseSeqSet(maybeString(f)); err != nil {
			return nil, err
		}
	case "UNANSWERED", "UNDELETED", "UNDRAFT", "UNFLAGGED", "UNSEEN":
		unflag := strings.TrimPrefix(key, "UN")
		c.WithoutFlags = append(c.WithoutFlags, CanonicalFlag("\\"+unflag))
	case "UNKEYWORD":
		if f, fields, err = popSearchField(fields); err != nil {
			return nil, err
		} else {
			c.WithoutFlags = append(c.WithoutFlags, CanonicalFlag(maybeString(f)))
		}
	default: // Try to parse a sequence set
		if c.SeqNum, err = ParseSeqSet(key); err != nil {
			return nil, err
		}
	}

	return fields, nil
}

// ParseWithCharset parses a search criteria from the provided fields.
// charsetReader is an optional function that converts from the fields charset
// to UTF-8.
func (c *SearchCriteria) ParseWithCharset(fields []interface{}, charsetReader func(io.Reader) io.Reader) error {
	for len(fields) > 0 {
		var err error
		if fields, err = c.parseField(fields, charsetReader); err != nil {
			return err
		}
	}
	return nil
}

// Format formats search criteria to fields. UTF-8 is used.
func (c *SearchCriteria) Format() []interface{} {
	var fields []interface{}

	if c.SeqNum != nil {
		fields = append(fields, c.SeqNum)
	}
	if c.Uid != nil {
		fields = append(fields, RawString("UID"), c.Uid)
	}

	if !c.Since.IsZero() && !c.Before.IsZero() && c.Before.Sub(c.Since) == 24*time.Hour {
		fields = append(fields, RawString("ON"), searchDate(c.Since))
	} else {
		if !c.Since.IsZero() {
			fields = append(fields, RawString("SINCE"), searchDate(c.Since))
		}
		if !c.Before.IsZero() {
			fields = append(fields, RawString("BEFORE"), searchDate(c.Before))
		}
	}
	if !c.SentSince.IsZero() && !c.SentBefore.IsZero() && c.SentBefore.Sub(c.SentSince) == 24*time.Hour {
		fields = append(fields, RawString("SENTON"), searchDate(c.SentSince))
	} else {
		if !c.SentSince.IsZero() {
			fields = append(fields, RawString("SENTSINCE"), searchDate(c.SentSince))
		}
		if !c.SentBefore.IsZero() {
			fields = append(fields, RawString("SENTBEFORE"), searchDate(c.SentBefore))
		}
	}

	for key, values := range c.Header {
		var prefields []interface{}
		switch key {
		case "Bcc", "Cc", "From", "Subject", "To":
			prefields = []interface{}{RawString(strings.ToUpper(key))}
		default:
			prefields = []interface{}{RawString("HEADER"), key}
		}
		for _, value := range values {
			fields = append(fields, prefields...)
			fields = append(fields, value)
		}
	}

	for _, value := range c.Body {
		fields = append(fields, RawString("BODY"), value)
	}
	for _, value := range c.Text {
		fields = append(fields, RawString("TEXT"), value)
	}

	for _, flag := range c.WithFlags {
		var subfields []interface{}
		switch flag {
		case AnsweredFlag, DeletedFlag, DraftFlag, FlaggedFlag, RecentFlag, SeenFlag:
			subfields = []interface{}{RawString(strings.ToUpper(strings.TrimPrefix(flag, "\\")))}
		default:
			subfields = []interface{}{RawString("KEYWORD"), RawString(flag)}
		}
		fields = append(fields, subfields...)
	}
	for _, flag := range c.WithoutFlags {
		var subfields []interface{}
		switch flag {
		case AnsweredFlag, DeletedFlag, DraftFlag, FlaggedFlag, SeenFlag:
			subfields = []interface{}{RawString("UN" + strings.ToUpper(strings.TrimPrefix(flag, "\\")))}
		case RecentFlag:
			subfields = []interface{}{RawString("OLD")}
		default:
			subfields = []interface{}{RawString("UNKEYWORD"), RawString(flag)}
		}
		fields = append(fields, subfields...)
	}

	if c.Larger > 0 {
		fields = append(fields, RawString("LARGER"), c.Larger)
	}
	if c.Smaller > 0 {
		fields = append(fields, RawString("SMALLER"), c.Smaller)
	}

	for _, not := range c.Not {
		fields = append(fields, RawString("NOT"), not.Format())
	}

	for _, or := range c.Or {
		fields = append(fields, RawString("OR"), or[0].Format(), or[1].Format())
	}

	// Not a single criteria given, add ALL criteria as fallback
	if len(fields) == 0 {
		fields = append(fields, RawString("ALL"))
	}

	return fields
}
