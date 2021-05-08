package responses

import (
	"github.com/emersion/go-imap"
)

// A CAPABILITY response.
// See RFC 3501 section 7.2.1
type Capability struct {
	Caps []string
}

func (r *Capability) WriteTo(w *imap.Writer) error {
	fields := []interface{}{imap.RawString("CAPABILITY")}
	for _, cap := range r.Caps {
		fields = append(fields, imap.RawString(cap))
	}

	return imap.NewUntaggedResp(fields).WriteTo(w)
}
