package responses

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/utf7"
)

const statusName = "STATUS"

// A STATUS response.
// See RFC 3501 section 7.2.4
type Status struct {
	Mailbox *imap.MailboxStatus
}

func (r *Status) Handle(resp imap.Resp) error {
	if r.Mailbox == nil {
		r.Mailbox = &imap.MailboxStatus{}
	}
	mbox := r.Mailbox

	name, fields, ok := imap.ParseNamedResp(resp)
	if !ok || name != statusName {
		return ErrUnhandled
	} else if len(fields) < 2 {
		return errNotEnoughFields
	}

	if name, err := imap.ParseString(fields[0]); err != nil {
		return err
	} else if name, err := utf7.Encoding.NewDecoder().String(name); err != nil {
		return err
	} else {
		mbox.Name = imap.CanonicalMailboxName(name)
	}

	var items []interface{}
	if items, ok = fields[1].([]interface{}); !ok {
		return errors.New("STATUS response expects a list as second argument")
	}

	mbox.Items = nil
	return mbox.Parse(items)
}

func (r *Status) WriteTo(w *imap.Writer) error {
	mbox := r.Mailbox
	name, _ := utf7.Encoding.NewEncoder().String(mbox.Name)
	fields := []interface{}{imap.RawString(statusName), imap.FormatMailboxName(name), mbox.Format()}
	return imap.NewUntaggedResp(fields).WriteTo(w)
}
