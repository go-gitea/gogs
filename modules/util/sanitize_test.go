// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeURLCredentials(t *testing.T) {
	var kases = map[string]string{
		"https://github.com/go-gitea/test_repo.git":         "https://github.com/go-gitea/test_repo.git",
		"https://mytoken@github.com/go-gitea/test_repo.git": "https://github.com/go-gitea/test_repo.git",
		"http://github.com/go-gitea/test_repo.git":          "http://github.com/go-gitea/test_repo.git",
		"/test/repos/repo1":                                 "/test/repos/repo1",
		"git@github.com:go-gitea/test_repo.git":             "(unparsable url)",
		"ssh://user@server/absolute/path/to/repo.git":       "ssh://user@server/absolute/path/to/repo.git",
		"ssh://user@server/~/relative/path/to/repo.git":     "ssh://user@server/~/relative/path/to/repo.git",
		"ssh://user@server:relative/path/to/repo.git":       "(unparsable url)",
	}

	for source, value := range kases {
		assert.EqualValues(t, value, SanitizeURLCredentials(source, false))
	}
}
