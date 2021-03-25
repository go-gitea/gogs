// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTreeEntries(t *testing.T) {
	testCases := []struct {
		Input    string
		Expected []*TreeEntry
	}{
		{
			Input: `100644 blob ea0d83c9081af9500ac9f804101b3fd0a5c293af    8218	README.md
100644 blob 037f27dc9d353ae4fd50f0474b2194c593914e35    4681	README_ZH.md
100644 blob 9846a94f7e8350a916632929d0fda38c90dd2ca8     429	SECURITY.md
040000 tree 84b90550547016f73c5dd3f50dea662389e67b6d       -	assets
`,
			Expected: []*TreeEntry{
				{
					ID:        MustIDFromString("ea0d83c9081af9500ac9f804101b3fd0a5c293af"),
					name:      "README.md",
					entryMode: EntryModeBlob,
					size:      8218,
					sized:     true,
				},
				{
					ID:        MustIDFromString("037f27dc9d353ae4fd50f0474b2194c593914e35"),
					name:      "README_ZH.md",
					entryMode: EntryModeBlob,
					size:      4681,
					sized:     true,
				},
				{
					ID:        MustIDFromString("9846a94f7e8350a916632929d0fda38c90dd2ca8"),
					name:      "SECURITY.md",
					entryMode: EntryModeBlob,
					size:      429,
					sized:     true,
				},
				{
					ID:        MustIDFromString("84b90550547016f73c5dd3f50dea662389e67b6d"),
					name:      "assets",
					entryMode: EntryModeTree,
					sized:     true,
				},
			},
		},
	}
	for _, testCase := range testCases {
		entries, err := ParseTreeEntries([]byte(testCase.Input))
		assert.NoError(t, err)
		assert.EqualValues(t, len(testCase.Expected), len(entries))
		for i, entry := range entries {
			assert.EqualValues(t, testCase.Expected[i].ID, entry.ID)
			assert.EqualValues(t, testCase.Expected[i].name, entry.name)
			assert.EqualValues(t, testCase.Expected[i].entryMode, entry.entryMode)
			assert.EqualValues(t, testCase.Expected[i].sized, entry.sized)
			assert.EqualValues(t, testCase.Expected[i].size, entry.size)
		}
	}
}
