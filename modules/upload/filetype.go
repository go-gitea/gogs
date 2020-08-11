// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package upload

import (
	"fmt"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// ErrFileTypeForbidden not allowed file type error
type ErrFileTypeForbidden struct {
	Type string
}

// IsErrFileTypeForbidden checks if an error is a ErrFileTypeForbidden.
func IsErrFileTypeForbidden(err error) bool {
	_, ok := err.(ErrFileTypeForbidden)
	return ok
}

func (err ErrFileTypeForbidden) Error() string {
	return fmt.Sprintf("File type is not allowed: %s", err.Type)
}

// VerifyAllowedContentType validates a file is allowed to be uploaded.
func VerifyAllowedContentType(buf []byte, allowedTypes []string) error {
	fileType := http.DetectContentType(buf)

	for _, t := range allowedTypes {
		t := strings.Trim(t, " ")

		if t == fileType ||
			// Allow wildcard */* to match all mime types
			t == "*/*" ||
			// Allow directives after type, like 'text/plain; charset=utf-8'
			strings.HasPrefix(fileType, t+";") ||
			// Allow a class whitelist, like 'image/*'
			(strings.HasSuffix(t, "/*") && strings.HasPrefix(fileType, strings.TrimRight(t, "*"))) {
			return nil
		}
	}

	log.Info("Attachment with type %s blocked from upload", fileType)
	return ErrFileTypeForbidden{Type: fileType}
}
