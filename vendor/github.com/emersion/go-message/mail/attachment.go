package mail

import (
	"github.com/emersion/go-message"
)

// An AttachmentHeader represents an attachment's header.
type AttachmentHeader struct {
	message.Header
}

// Filename parses the attachment's filename.
func (h *AttachmentHeader) Filename() (string, error) {
	_, params, err := h.ContentDisposition()

	filename, ok := params["filename"]
	if !ok {
		// Using "name" in Content-Type is discouraged
		_, params, err = h.ContentType()
		filename = params["name"]
	}

	return filename, err
}

// SetFilename formats the attachment's filename.
func (h *AttachmentHeader) SetFilename(filename string) {
	dispParams := map[string]string{"filename": filename}
	h.SetContentDisposition("attachment", dispParams)
}
