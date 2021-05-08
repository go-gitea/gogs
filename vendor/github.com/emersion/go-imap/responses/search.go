package responses

import (
	"github.com/emersion/go-imap"
)

const searchName = "SEARCH"

// A SEARCH response.
// See RFC 3501 section 7.2.5
type Search struct {
	Ids []uint32
}

func (r *Search) Handle(resp imap.Resp) error {
	name, fields, ok := imap.ParseNamedResp(resp)
	if !ok || name != searchName {
		return ErrUnhandled
	}

	r.Ids = make([]uint32, len(fields))
	for i, f := range fields {
		if id, err := imap.ParseNumber(f); err != nil {
			return err
		} else {
			r.Ids[i] = id
		}
	}

	return nil
}

func (r *Search) WriteTo(w *imap.Writer) (err error) {
	fields := []interface{}{imap.RawString(searchName)}
	for _, id := range r.Ids {
		fields = append(fields, id)
	}

	resp := imap.NewUntaggedResp(fields)
	return resp.WriteTo(w)
}
