package responses

import (
	"encoding/base64"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-sasl"
)

// An AUTHENTICATE response.
type Authenticate struct {
	Mechanism       sasl.Client
	InitialResponse []byte
	RepliesCh       chan []byte
}

// Implements
func (r *Authenticate) Replies() <-chan []byte {
	return r.RepliesCh
}

func (r *Authenticate) writeLine(l string) error {
	r.RepliesCh <- []byte(l + "\r\n")
	return nil
}

func (r *Authenticate) cancel() error {
	return r.writeLine("*")
}

func (r *Authenticate) Handle(resp imap.Resp) error {
	cont, ok := resp.(*imap.ContinuationReq)
	if !ok {
		return ErrUnhandled
	}

	// Empty challenge, send initial response as stated in RFC 2222 section 5.1
	if cont.Info == "" && r.InitialResponse != nil {
		encoded := base64.StdEncoding.EncodeToString(r.InitialResponse)
		if err := r.writeLine(encoded); err != nil {
			return err
		}
		r.InitialResponse = nil
		return nil
	}

	challenge, err := base64.StdEncoding.DecodeString(cont.Info)
	if err != nil {
		r.cancel()
		return err
	}

	reply, err := r.Mechanism.Next(challenge)
	if err != nil {
		r.cancel()
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(reply)
	return r.writeLine(encoded)
}
