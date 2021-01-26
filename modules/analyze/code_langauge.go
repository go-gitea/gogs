// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package analyze

import (
	"path/filepath"

	"github.com/go-enry/go-enry/v2"
)

// GetCodeLanguage detects code language based on file name and content
func GetCodeLanguage(filename string, content []byte) string {
	if language, ok := enry.GetLanguageByExtension(filename); ok {
		return language
	}

	if language, ok := enry.GetLanguageByFilename(filename); ok {
		return language
	}

	if len(content) == 0 {
		return enry.OtherLanguage
	}

	return enry.GetLanguage(filepath.Base(filename), content)
}

// DiffIgnore checks if path/file is considered either vendor or generated
func DiffIgnore(path string) bool {
	return enry.IsVendor(path) || enry.IsGenerated(path, nil)
}
