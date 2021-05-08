package imap

import (
	"strings"
)

// Resp is an IMAP response. It is either a *DataResp, a
// *ContinuationReq or a *StatusResp.
type Resp interface {
	resp()
}

// ReadResp reads a single response from a Reader.
func ReadResp(r *Reader) (Resp, error) {
	atom, err := r.ReadAtom()
	if err != nil {
		return nil, err
	}
	tag, ok := atom.(string)
	if !ok {
		return nil, newParseError("response tag is not an atom")
	}

	if tag == "+" {
		if err := r.ReadSp(); err != nil {
			r.UnreadRune()
		}

		resp := &ContinuationReq{}
		resp.Info, err = r.ReadInfo()
		if err != nil {
			return nil, err
		}

		return resp, nil
	}

	if err := r.ReadSp(); err != nil {
		return nil, err
	}

	// Can be either data or status
	// Try to parse a status
	var fields []interface{}
	if atom, err := r.ReadAtom(); err == nil {
		fields = append(fields, atom)

		if err := r.ReadSp(); err == nil {
			if name, ok := atom.(string); ok {
				status := StatusRespType(name)
				switch status {
				case StatusRespOk, StatusRespNo, StatusRespBad, StatusRespPreauth, StatusRespBye:
					resp := &StatusResp{
						Tag:  tag,
						Type: status,
					}

					char, _, err := r.ReadRune()
					if err != nil {
						return nil, err
					}
					r.UnreadRune()

					if char == '[' {
						// Contains code & arguments
						resp.Code, resp.Arguments, err = r.ReadRespCode()
						if err != nil {
							return nil, err
						}
					}

					resp.Info, err = r.ReadInfo()
					if err != nil {
						return nil, err
					}

					return resp, nil
				}
			}
		} else {
			r.UnreadRune()
		}
	} else {
		r.UnreadRune()
	}

	// Not a status so it's data
	resp := &DataResp{Tag: tag}

	var remaining []interface{}
	remaining, err = r.ReadLine()
	if err != nil {
		return nil, err
	}

	resp.Fields = append(fields, remaining...)
	return resp, nil
}

// DataResp is an IMAP response containing data.
type DataResp struct {
	// The response tag. Can be either "" for untagged responses, "+" for continuation
	// requests or a previous command's tag.
	Tag string
	// The parsed response fields.
	Fields []interface{}
}

// NewUntaggedResp creates a new untagged response.
func NewUntaggedResp(fields []interface{}) *DataResp {
	return &DataResp{
		Tag:    "*",
		Fields: fields,
	}
}

func (r *DataResp) resp() {}

func (r *DataResp) WriteTo(w *Writer) error {
	tag := RawString(r.Tag)
	if tag == "" {
		tag = RawString("*")
	}

	fields := []interface{}{RawString(tag)}
	fields = append(fields, r.Fields...)
	return w.writeLine(fields...)
}

// ContinuationReq is a continuation request response.
type ContinuationReq struct {
	// The info message sent with the continuation request.
	Info string
}

func (r *ContinuationReq) resp() {}

func (r *ContinuationReq) WriteTo(w *Writer) error {
	if err := w.writeString("+"); err != nil {
		return err
	}

	if r.Info != "" {
		if err := w.writeString(string(sp) + r.Info); err != nil {
			return err
		}
	}

	return w.writeCrlf()
}

// ParseNamedResp attempts to parse a named data response.
func ParseNamedResp(resp Resp) (name string, fields []interface{}, ok bool) {
	data, ok := resp.(*DataResp)
	if !ok || len(data.Fields) == 0 {
		return
	}

	// Some responses (namely EXISTS and RECENT) are formatted like so:
	//   [num] [name] [...]
	// Which is fucking stupid. But we handle that here by checking if the
	// response name is a number and then rearranging it.
	if len(data.Fields) > 1 {
		name, ok := data.Fields[1].(string)
		if ok {
			if _, err := ParseNumber(data.Fields[0]); err == nil {
				fields := []interface{}{data.Fields[0]}
				fields = append(fields, data.Fields[2:]...)
				return strings.ToUpper(name), fields, true
			}
		}
	}

	// IMAP commands are formatted like this:
	//   [name] [...]
	name, ok = data.Fields[0].(string)
	if !ok {
		return
	}
	return strings.ToUpper(name), data.Fields[1:], true
}
