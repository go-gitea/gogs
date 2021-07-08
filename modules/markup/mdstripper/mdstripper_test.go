// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mdstripper

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMarkdownStripper(t *testing.T) {
	type testItem struct {
		markdown      string
		expectedText  []string
		expectedLinks []string
	}

	list := []testItem{
		{
			`
## This is a title

This is [one](link) to paradise.
This **is emphasized**.
This: should coalesce.

` + "```" + `
This is a code block.
This should not appear in the output at all.
` + "```" + `

* Bullet 1
* Bullet 2

A HIDDEN ` + "`" + `GHOST` + "`" + ` IN THIS LINE.
		`,
			[]string{
				"This is a title",
				"This is",
				"to paradise.",
				"This",
				"is emphasized",
				".",
				"This: should coalesce.",
				"Bullet 1",
				"Bullet 2",
				"A HIDDEN",
				"IN THIS LINE.",
			},
			[]string{
				"link",
			}},
		{
			"Simply closes: #29 yes",
			[]string{
				"Simply closes: #29 yes",
			},
			[]string{},
		},
		{
			"Simply closes: !29 yes",
			[]string{
				"Simply closes: !29 yes",
			},
			[]string{},
		},
	}

	for _, test := range list {
		text, links := StripMarkdown([]byte(test.markdown))
		rawlines := strings.Split(text, "\n")
		lines := make([]string, 0, len(rawlines))
		for _, line := range rawlines {
			line := strings.TrimSpace(line)
			if line != "" {
				lines = append(lines, line)
			}
		}
		assert.EqualValues(t, test.expectedText, lines)
		assert.EqualValues(t, test.expectedLinks, links)
	}
}
