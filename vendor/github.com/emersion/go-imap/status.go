package imap

import (
	"errors"
)

// A status response type.
type StatusRespType string

// Status response types defined in RFC 3501 section 7.1.
const (
	// The OK response indicates an information message from the server.  When
	// tagged, it indicates successful completion of the associated command.
	// The untagged form indicates an information-only message.
	StatusRespOk StatusRespType = "OK"

	// The NO response indicates an operational error message from the
	// server.  When tagged, it indicates unsuccessful completion of the
	// associated command.  The untagged form indicates a warning; the
	// command can still complete successfully.
	StatusRespNo StatusRespType = "NO"

	// The BAD response indicates an error message from the server.  When
	// tagged, it reports a protocol-level error in the client's command;
	// the tag indicates the command that caused the error.  The untagged
	// form indicates a protocol-level error for which the associated
	// command can not be determined; it can also indicate an internal
	// server failure.
	StatusRespBad StatusRespType = "BAD"

	// The PREAUTH response is always untagged, and is one of three
	// possible greetings at connection startup.  It indicates that the
	// connection has already been authenticated by external means; thus
	// no LOGIN command is needed.
	StatusRespPreauth StatusRespType = "PREAUTH"

	// The BYE response is always untagged, and indicates that the server
	// is about to close the connection.
	StatusRespBye StatusRespType = "BYE"
)

type StatusRespCode string

// Status response codes defined in RFC 3501 section 7.1.
const (
	CodeAlert          StatusRespCode = "ALERT"
	CodeBadCharset     StatusRespCode = "BADCHARSET"
	CodeCapability     StatusRespCode = "CAPABILITY"
	CodeParse          StatusRespCode = "PARSE"
	CodePermanentFlags StatusRespCode = "PERMANENTFLAGS"
	CodeReadOnly       StatusRespCode = "READ-ONLY"
	CodeReadWrite      StatusRespCode = "READ-WRITE"
	CodeTryCreate      StatusRespCode = "TRYCREATE"
	CodeUidNext        StatusRespCode = "UIDNEXT"
	CodeUidValidity    StatusRespCode = "UIDVALIDITY"
	CodeUnseen         StatusRespCode = "UNSEEN"
)

// A status response.
// See RFC 3501 section 7.1
type StatusResp struct {
	// The response tag. If empty, it defaults to *.
	Tag string
	// The status type.
	Type StatusRespType
	// The status code.
	// See https://www.iana.org/assignments/imap-response-codes/imap-response-codes.xhtml
	Code StatusRespCode
	// Arguments provided with the status code.
	Arguments []interface{}
	// The status info.
	Info string
}

func (r *StatusResp) resp() {}

// If this status is NO or BAD, returns an error with the status info.
// Otherwise, returns nil.
func (r *StatusResp) Err() error {
	if r == nil {
		// No status response, connection closed before we get one
		return errors.New("imap: connection closed during command execution")
	}

	if r.Type == StatusRespNo || r.Type == StatusRespBad {
		return errors.New(r.Info)
	}
	return nil
}

func (r *StatusResp) WriteTo(w *Writer) error {
	tag := RawString(r.Tag)
	if tag == "" {
		tag = "*"
	}

	if err := w.writeFields([]interface{}{RawString(tag), RawString(r.Type)}); err != nil {
		return err
	}

	if err := w.writeString(string(sp)); err != nil {
		return err
	}

	if r.Code != "" {
		if err := w.writeRespCode(r.Code, r.Arguments); err != nil {
			return err
		}

		if err := w.writeString(string(sp)); err != nil {
			return err
		}
	}

	if err := w.writeString(r.Info); err != nil {
		return err
	}

	return w.writeCrlf()
}

// ErrStatusResp can be returned by a server.Handler to replace the default status
// response. The response tag must be empty.
//
// To suppress default response, Resp should be set to nil.
type ErrStatusResp struct {
	// Response to send instead of default.
	Resp *StatusResp
}

func (err *ErrStatusResp) Error() string {
	if err.Resp == nil {
		return "imap: suppressed response"
	}
	return err.Resp.Info
}
