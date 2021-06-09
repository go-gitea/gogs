// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
)

func TestMakeAbsoluteAssetURL(t *testing.T) {
	assert.Equal(t, "https://localhost:2345", MakeAbsoluteAssetURL("https://localhost:1234", "https://localhost:2345"))
	assert.Equal(t, "https://localhost:2345", MakeAbsoluteAssetURL("https://localhost:1234/", "https://localhost:2345"))
	assert.Equal(t, "https://localhost:2345", MakeAbsoluteAssetURL("https://localhost:1234/", "https://localhost:2345/"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/", "/foo/"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/foo", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/foo/"))
	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL("https://localhost:1234/foo", "/bar"))
	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/bar"))
	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL("https://localhost:1234/foo/", "/bar/"))
}

func TestMakeManifestData(t *testing.T) {
	jsonBytes := MakeManifestData(`Example App '\"`, "https://example.com", "https://example.com/foo/bar")
	assert.True(t, json.Valid(jsonBytes))
}
