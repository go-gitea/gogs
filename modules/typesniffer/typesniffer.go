// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package typesniffer

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// Use at most this many bytes to determine Content Type.
const sniffLen = 1024

// SvgMimeType MIME type of SVG images.
const SvgMimeType = "image/svg+xml"

var svgTagRegex = regexp.MustCompile(`(?si)\A\s*(?:(<!--.*?-->|<!DOCTYPE\s+svg([\s:]+.*?>|>))\s*)*<svg[\s>\/]`)
var svgTagInXMLRegex = regexp.MustCompile(`(?si)\A<\?xml\b.*?\?>\s*(?:(<!--.*?-->|<!DOCTYPE\s+svg([\s:]+.*?>|>))\s*)*<svg[\s>\/]`)

// SniffedType contains information about a blobs type.
type SniffedType struct {
	contentType string
}

// IsText etects if content format is plain text.
func (ct SniffedType) IsText() bool {
	return strings.Contains(ct.contentType, "text/")
}

// IsImage detects if data is an image format
func (ct SniffedType) IsImage() bool {
	return strings.Contains(ct.contentType, "image/")
}

// IsSvgImage detects if data is an SVG image format
func (ct SniffedType) IsSvgImage() bool {
	return strings.Contains(ct.contentType, SvgMimeType)
}

// IsPDF detects if data is a PDF format
func (ct SniffedType) IsPDF() bool {
	return strings.Contains(ct.contentType, "application/pdf")
}

// IsVideo detects if data is an video format
func (ct SniffedType) IsVideo() bool {
	return strings.Contains(ct.contentType, "video/")
}

// IsAudio detects if data is an video format
func (ct SniffedType) IsAudio() bool {
	return strings.Contains(ct.contentType, "audio/")
}

// IsRepresentableAsText returns true if file content can be represented as
// plain text or is empty.
func (ct SniffedType) IsRepresentableAsText() bool {
	return ct.IsText() || ct.IsSvgImage()
}

// DetectContentType extends http.DetectContentType with more content types. Defaults to text/unknown if input is empty.
func DetectContentType(data []byte) SniffedType {
	if len(data) == 0 {
		return SniffedType{"text/unknown"}
	}

	ct := http.DetectContentType(data)

	if len(data) > sniffLen {
		data = data[:sniffLen]
	}

	if (strings.Contains(ct, "text/plain") || strings.Contains(ct, "text/html")) && svgTagRegex.Match(data) ||
		strings.Contains(ct, "text/xml") && svgTagInXMLRegex.Match(data) {
		// SVG is unsupported. https://github.com/golang/go/issues/15888
		ct = SvgMimeType
	}

	return SniffedType{ct}
}

// DetectContentTypeFromReader guesses the content type contained in the reader.
func DetectContentTypeFromReader(r io.Reader) (SniffedType, error) {
	buf := make([]byte, sniffLen)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return SniffedType{}, fmt.Errorf("DetectContentTypeFromReader io error: %w", err)
	}
	buf = buf[:n]

	return DetectContentType(buf), nil
}
