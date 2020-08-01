// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"net/url"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// urlSafeError wraps an error whose message may contain a sensitive URL
type urlSafeError struct {
	err            error
	unsanitizedURL string
}

func (err urlSafeError) Error() string {
	return SanitizeMessage(err.err.Error(), err.unsanitizedURL)
}

// URLSanitizedError returns the sanitized version an error whose message may
// contain a sensitive URL
func URLSanitizedError(err error, unsanitizedURL string) error {
	return urlSafeError{err: err, unsanitizedURL: unsanitizedURL}
}

// SanitizeMessage sanitizes a message which may contains a sensitive URL
func SanitizeMessage(message, unsanitizedURL string) string {
	sanitizedURL := SanitizeURLCredentials(unsanitizedURL, true)
	return strings.Replace(message, unsanitizedURL, sanitizedURL, -1)
}

// SanitizeURLCredentials sanitizes a url, either removing user credentials
// or replacing them with a placeholder.
func SanitizeURLCredentials(unsanitizedURL string, usePlaceholder bool) string {
	u, err := url.Parse(unsanitizedURL)
	if err != nil {
		log.Error("parse url %s failed: %v", unsanitizedURL, err)
		// don't log the error, since it might contain unsanitized URL.
		return "(unparsable url)"
	}
	if u.Scheme == "ssh" {
		return u.String()
	}
	if u.User != nil && usePlaceholder {
		u.User = url.User("<credentials>")
	} else {
		u.User = nil
	}
	return u.String()
}
