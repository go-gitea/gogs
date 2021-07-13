// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package utils

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"
	"github.com/stretchr/testify/assert"
)

func TestRemoveUsernameParameterSuffix(t *testing.T) {
	assert.Equal(t, "foobar", RemoveUsernameParameterSuffix("foobar (Foo Bar)"))
	assert.Equal(t, "foobar", RemoveUsernameParameterSuffix("foobar"))
	assert.Equal(t, "", RemoveUsernameParameterSuffix(""))
}

func TestIsValidSlackChannel(t *testing.T) {
	tt := []struct {
		channelName string
		expected    bool
	}{
		{"gitea", true},
		{"  ", false},
		{"#", false},
		{"gitea   ", true},
		{"  gitea", true},
	}

	for _, v := range tt {
		assert.Equal(t, v.expected, IsValidSlackChannel(v.channelName))
	}
}

func TestIsExternalURL(t *testing.T) {
	setting.AppURL = "https://try.gitea.io/"
	type test struct {
		Expected bool
		RawURL   string
	}
	newTest := func(expected bool, rawURL string) test {
		return test{Expected: expected, RawURL: rawURL}
	}
	for _, test := range []test{
		newTest(false,
			"https://try.gitea.io"),
		newTest(true,
			"https://example.com/"),
		newTest(true,
			"//example.com"),
		newTest(true,
			"http://example.com"),
		newTest(false,
			"a/"),
		newTest(false,
			"https://try.gitea.io/test?param=false"),
		newTest(false,
			"test?param=false"),
		newTest(false,
			"//try.gitea.io/test?param=false"),
		newTest(false,
			"/hey/hey/hey#3244"),
		newTest(true,
			"://missing protocol scheme"),
	} {
		assert.Equal(t, test.Expected, IsExternalURL(test.RawURL))
	}
}

func TestSanitizeFlashErrorString(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "no error",
			arg:  "",
			want: "",
		},
		{
			name: "normal error",
			arg:  "can not open file: \"abc.exe\"",
			want: "can not open file: &#34;abc.exe&#34;",
		},
		{
			name: "line break error",
			arg:  "some error:\n\nawesome!",
			want: "some error:<br><br>awesome!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeFlashErrorString(tt.arg); got != tt.want {
				t.Errorf("SanitizeFlashErrorString() = '%v', want '%v'", got, tt.want)
			}
		})
	}
}
