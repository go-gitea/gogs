package imap

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/emersion/go-imap/utf7"
)

// The primary mailbox, as defined in RFC 3501 section 5.1.
const InboxName = "INBOX"

// CanonicalMailboxName returns the canonical form of a mailbox name. Mailbox names can be
// case-sensitive or case-insensitive depending on the backend implementation.
// The special INBOX mailbox is case-insensitive.
func CanonicalMailboxName(name string) string {
	if strings.ToUpper(name) == InboxName {
		return InboxName
	}
	return name
}

// Mailbox attributes definied in RFC 3501 section 7.2.2.
const (
	// It is not possible for any child levels of hierarchy to exist under this\
	// name; no child levels exist now and none can be created in the future.
	NoInferiorsAttr = "\\Noinferiors"
	// It is not possible to use this name as a selectable mailbox.
	NoSelectAttr = "\\Noselect"
	// The mailbox has been marked "interesting" by the server; the mailbox
	// probably contains messages that have been added since the last time the
	// mailbox was selected.
	MarkedAttr = "\\Marked"
	// The mailbox does not contain any additional messages since the last time
	// the mailbox was selected.
	UnmarkedAttr = "\\Unmarked"
)

// Basic mailbox info.
type MailboxInfo struct {
	// The mailbox attributes.
	Attributes []string
	// The server's path separator.
	Delimiter string
	// The mailbox name.
	Name string
}

// Parse mailbox info from fields.
func (info *MailboxInfo) Parse(fields []interface{}) error {
	if len(fields) < 3 {
		return errors.New("Mailbox info needs at least 3 fields")
	}

	var err error
	if info.Attributes, err = ParseStringList(fields[0]); err != nil {
		return err
	}

	var ok bool
	if info.Delimiter, ok = fields[1].(string); !ok {
		// The delimiter may be specified as NIL, which gets converted to a nil interface.
		if fields[1] != nil {
			return errors.New("Mailbox delimiter must be a string")
		}
		info.Delimiter = ""
	}

	if name, err := ParseString(fields[2]); err != nil {
		return err
	} else if name, err := utf7.Encoding.NewDecoder().String(name); err != nil {
		return err
	} else {
		info.Name = CanonicalMailboxName(name)
	}

	return nil
}

// Format mailbox info to fields.
func (info *MailboxInfo) Format() []interface{} {
	name, _ := utf7.Encoding.NewEncoder().String(info.Name)
	attrs := make([]interface{}, len(info.Attributes))
	for i, attr := range info.Attributes {
		attrs[i] = RawString(attr)
	}

	// If the delimiter is NIL, we need to treat it specially by inserting
	// a nil field (so that it's later converted to an unquoted NIL atom).
	var del interface{}

	if info.Delimiter != "" {
		del = info.Delimiter
	}

	// Thunderbird doesn't understand delimiters if not quoted
	return []interface{}{attrs, del, FormatMailboxName(name)}
}

// TODO: optimize this
func (info *MailboxInfo) match(name, pattern string) bool {
	i := strings.IndexAny(pattern, "*%")
	if i == -1 {
		// No more wildcards
		return name == pattern
	}

	// Get parts before and after wildcard
	chunk, wildcard, rest := pattern[0:i], pattern[i], pattern[i+1:]

	// Check that name begins with chunk
	if len(chunk) > 0 && !strings.HasPrefix(name, chunk) {
		return false
	}
	name = strings.TrimPrefix(name, chunk)

	// Expand wildcard
	var j int
	for j = 0; j < len(name); j++ {
		if wildcard == '%' && string(name[j]) == info.Delimiter {
			break // Stop on delimiter if wildcard is %
		}
		// Try to match the rest from here
		if info.match(name[j:], rest) {
			return true
		}
	}

	return info.match(name[j:], rest)
}

// Match checks if a reference and a pattern matches this mailbox name, as
// defined in RFC 3501 section 6.3.8.
func (info *MailboxInfo) Match(reference, pattern string) bool {
	name := info.Name

	if info.Delimiter != "" && strings.HasPrefix(pattern, info.Delimiter) {
		reference = ""
		pattern = strings.TrimPrefix(pattern, info.Delimiter)
	}
	if reference != "" {
		if info.Delimiter != "" && !strings.HasSuffix(reference, info.Delimiter) {
			reference += info.Delimiter
		}
		if !strings.HasPrefix(name, reference) {
			return false
		}
		name = strings.TrimPrefix(name, reference)
	}

	return info.match(name, pattern)
}

// A mailbox status.
type MailboxStatus struct {
	// The mailbox name.
	Name string
	// True if the mailbox is open in read-only mode.
	ReadOnly bool
	// The mailbox items that are currently filled in. This map's values
	// should not be used directly, they must only be used by libraries
	// implementing extensions of the IMAP protocol.
	Items map[StatusItem]interface{}

	// The Items map may be accessed in different goroutines. Protect
	// concurrent writes.
	ItemsLocker sync.Mutex

	// The mailbox flags.
	Flags []string
	// The mailbox permanent flags.
	PermanentFlags []string
	// The sequence number of the first unseen message in the mailbox.
	UnseenSeqNum uint32

	// The number of messages in this mailbox.
	Messages uint32
	// The number of messages not seen since the last time the mailbox was opened.
	Recent uint32
	// The number of unread messages.
	Unseen uint32
	// The next UID.
	UidNext uint32
	// Together with a UID, it is a unique identifier for a message.
	// Must be greater than or equal to 1.
	UidValidity uint32
}

// Create a new mailbox status that will contain the specified items.
func NewMailboxStatus(name string, items []StatusItem) *MailboxStatus {
	status := &MailboxStatus{
		Name:  name,
		Items: make(map[StatusItem]interface{}),
	}

	for _, k := range items {
		status.Items[k] = nil
	}

	return status
}

func (status *MailboxStatus) Parse(fields []interface{}) error {
	status.Items = make(map[StatusItem]interface{})

	var k StatusItem
	for i, f := range fields {
		if i%2 == 0 {
			if kstr, ok := f.(string); !ok {
				return fmt.Errorf("cannot parse mailbox status: key is not a string, but a %T", f)
			} else {
				k = StatusItem(strings.ToUpper(kstr))
			}
		} else {
			status.Items[k] = nil

			var err error
			switch k {
			case StatusMessages:
				status.Messages, err = ParseNumber(f)
			case StatusRecent:
				status.Recent, err = ParseNumber(f)
			case StatusUnseen:
				status.Unseen, err = ParseNumber(f)
			case StatusUidNext:
				status.UidNext, err = ParseNumber(f)
			case StatusUidValidity:
				status.UidValidity, err = ParseNumber(f)
			default:
				status.Items[k] = f
			}

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (status *MailboxStatus) Format() []interface{} {
	var fields []interface{}
	for k, v := range status.Items {
		switch k {
		case StatusMessages:
			v = status.Messages
		case StatusRecent:
			v = status.Recent
		case StatusUnseen:
			v = status.Unseen
		case StatusUidNext:
			v = status.UidNext
		case StatusUidValidity:
			v = status.UidValidity
		}

		fields = append(fields, RawString(k), v)
	}
	return fields
}

func FormatMailboxName(name string) interface{} {
	// Some e-mails servers don't handle quoted INBOX names correctly so we special-case it.
	if strings.EqualFold(name, "INBOX") {
		return RawString(name)
	}
	return name
}
