package responses

import (
	"fmt"

	"github.com/emersion/go-imap"
)

// A SELECT response.
type Select struct {
	Mailbox *imap.MailboxStatus
}

func (r *Select) Handle(resp imap.Resp) error {
	if r.Mailbox == nil {
		r.Mailbox = &imap.MailboxStatus{Items: make(map[imap.StatusItem]interface{})}
	}
	mbox := r.Mailbox

	switch resp := resp.(type) {
	case *imap.DataResp:
		name, fields, ok := imap.ParseNamedResp(resp)
		if !ok || name != "FLAGS" {
			return ErrUnhandled
		} else if len(fields) < 1 {
			return errNotEnoughFields
		}

		flags, _ := fields[0].([]interface{})
		mbox.Flags, _ = imap.ParseStringList(flags)
	case *imap.StatusResp:
		if len(resp.Arguments) < 1 {
			return ErrUnhandled
		}

		var item imap.StatusItem
		switch resp.Code {
		case "UNSEEN":
			mbox.UnseenSeqNum, _ = imap.ParseNumber(resp.Arguments[0])
		case "PERMANENTFLAGS":
			flags, _ := resp.Arguments[0].([]interface{})
			mbox.PermanentFlags, _ = imap.ParseStringList(flags)
		case "UIDNEXT":
			mbox.UidNext, _ = imap.ParseNumber(resp.Arguments[0])
			item = imap.StatusUidNext
		case "UIDVALIDITY":
			mbox.UidValidity, _ = imap.ParseNumber(resp.Arguments[0])
			item = imap.StatusUidValidity
		default:
			return ErrUnhandled
		}

		if item != "" {
			mbox.ItemsLocker.Lock()
			mbox.Items[item] = nil
			mbox.ItemsLocker.Unlock()
		}
	default:
		return ErrUnhandled
	}
	return nil
}

func (r *Select) WriteTo(w *imap.Writer) error {
	mbox := r.Mailbox

	if mbox.Flags != nil {
		flags := make([]interface{}, len(mbox.Flags))
		for i, f := range mbox.Flags {
			flags[i] = imap.RawString(f)
		}
		res := imap.NewUntaggedResp([]interface{}{imap.RawString("FLAGS"), flags})
		if err := res.WriteTo(w); err != nil {
			return err
		}
	}

	if mbox.PermanentFlags != nil {
		flags := make([]interface{}, len(mbox.PermanentFlags))
		for i, f := range mbox.PermanentFlags {
			flags[i] = imap.RawString(f)
		}
		statusRes := &imap.StatusResp{
			Type:      imap.StatusRespOk,
			Code:      imap.CodePermanentFlags,
			Arguments: []interface{}{flags},
			Info:      "Flags permitted.",
		}
		if err := statusRes.WriteTo(w); err != nil {
			return err
		}
	}

	if mbox.UnseenSeqNum > 0 {
		statusRes := &imap.StatusResp{
			Type:      imap.StatusRespOk,
			Code:      imap.CodeUnseen,
			Arguments: []interface{}{mbox.UnseenSeqNum},
			Info:      fmt.Sprintf("Message %d is first unseen", mbox.UnseenSeqNum),
		}
		if err := statusRes.WriteTo(w); err != nil {
			return err
		}
	}

	for k := range r.Mailbox.Items {
		switch k {
		case imap.StatusMessages:
			res := imap.NewUntaggedResp([]interface{}{mbox.Messages, imap.RawString("EXISTS")})
			if err := res.WriteTo(w); err != nil {
				return err
			}
		case imap.StatusRecent:
			res := imap.NewUntaggedResp([]interface{}{mbox.Recent, imap.RawString("RECENT")})
			if err := res.WriteTo(w); err != nil {
				return err
			}
		case imap.StatusUidNext:
			statusRes := &imap.StatusResp{
				Type:      imap.StatusRespOk,
				Code:      imap.CodeUidNext,
				Arguments: []interface{}{mbox.UidNext},
				Info:      "Predicted next UID",
			}
			if err := statusRes.WriteTo(w); err != nil {
				return err
			}
		case imap.StatusUidValidity:
			statusRes := &imap.StatusResp{
				Type:      imap.StatusRespOk,
				Code:      imap.CodeUidValidity,
				Arguments: []interface{}{mbox.UidValidity},
				Info:      "UIDs valid",
			}
			if err := statusRes.WriteTo(w); err != nil {
				return err
			}
		}
	}

	return nil
}
