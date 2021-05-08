package responses

import (
	"github.com/emersion/go-imap"
)

const fetchName = "FETCH"

// A FETCH response.
// See RFC 3501 section 7.4.2
type Fetch struct {
	Messages chan *imap.Message
}

func (r *Fetch) Handle(resp imap.Resp) error {
	name, fields, ok := imap.ParseNamedResp(resp)
	if !ok || name != fetchName {
		return ErrUnhandled
	} else if len(fields) < 1 {
		return errNotEnoughFields
	}

	seqNum, err := imap.ParseNumber(fields[0])
	if err != nil {
		return err
	}

	msgFields, _ := fields[1].([]interface{})
	msg := &imap.Message{SeqNum: seqNum}
	if err := msg.Parse(msgFields); err != nil {
		return err
	}

	r.Messages <- msg
	return nil
}

func (r *Fetch) WriteTo(w *imap.Writer) error {
	for msg := range r.Messages {
		resp := imap.NewUntaggedResp([]interface{}{msg.SeqNum, imap.RawString(fetchName), msg.Format()})
		if err := resp.WriteTo(w); err != nil {
			return err
		}
	}

	return nil
}
