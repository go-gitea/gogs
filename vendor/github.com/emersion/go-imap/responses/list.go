package responses

import (
	"github.com/emersion/go-imap"
)

const (
	listName = "LIST"
	lsubName = "LSUB"
)

// A LIST response.
// If Subscribed is set to true, LSUB will be used instead.
// See RFC 3501 section 7.2.2
type List struct {
	Mailboxes  chan *imap.MailboxInfo
	Subscribed bool
}

func (r *List) Name() string {
	if r.Subscribed {
		return lsubName
	} else {
		return listName
	}
}

func (r *List) Handle(resp imap.Resp) error {
	name, fields, ok := imap.ParseNamedResp(resp)
	if !ok || name != r.Name() {
		return ErrUnhandled
	}

	mbox := &imap.MailboxInfo{}
	if err := mbox.Parse(fields); err != nil {
		return err
	}

	r.Mailboxes <- mbox
	return nil
}

func (r *List) WriteTo(w *imap.Writer) error {
	respName := r.Name()

	for mbox := range r.Mailboxes {
		fields := []interface{}{imap.RawString(respName)}
		fields = append(fields, mbox.Format()...)

		resp := imap.NewUntaggedResp(fields)
		if err := resp.WriteTo(w); err != nil {
			return err
		}
	}

	return nil
}
