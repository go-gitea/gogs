// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAttributes(t *testing.T) {
	attributes, err := ParseAttributes(strings.NewReader("* foo -bar foo2 = bar2 foobar=test"))
	assert.NoError(t, err)
	assert.Len(t, attributes, 1)
	assert.Len(t, attributes[0].attributes, 5)
	assert.Contains(t, attributes[0].attributes, "foo")
	assert.True(t, attributes[0].attributes["foo"].(bool))
	assert.Contains(t, attributes[0].attributes, "bar")
	assert.False(t, attributes[0].attributes["bar"].(bool))
	assert.Contains(t, attributes[0].attributes, "foo2")
	assert.True(t, attributes[0].attributes["foo2"].(bool))
	assert.Contains(t, attributes[0].attributes, "bar2")
	assert.True(t, attributes[0].attributes["bar2"].(bool))
	assert.Contains(t, attributes[0].attributes, "foobar")
	assert.Equal(t, "test", attributes[0].attributes["foobar"].(string))
}

func TestForFile(t *testing.T) {
	input := `* text=auto eol=lf
/vendor/** -text -eol linguist-vendored
/public/vendor/** -text -eol linguist-vendored
/templates/**/*.tmpl linguist-language=Handlebars
/.eslintrc linguist-language=YAML
/.stylelintrc linguist-language=YAML`

	attributes, err := ParseAttributes(strings.NewReader(input))
	assert.NoError(t, err)
	assert.Len(t, attributes, 6)

	cases := []struct {
		filepath      string
		expectedKey   string
		expectedValue interface{}
	}{
		// case 0
		{
			"test.txt",
			"text",
			"auto",
		},
		// case 1
		{
			"test.txt",
			"eol",
			"lf",
		},
		// case 2
		{
			"/vendor/test.txt",
			"text",
			false,
		},
		// case 3
		{
			"/vendor/test.txt",
			"eol",
			false,
		},
		// case 4
		{
			"vendor/test.txt",
			"linguist-vendored",
			true,
		},
		// case 5
		{
			"/vendor/dir/dir/dir/test.txt",
			"linguist-vendored",
			true,
		},
		// case 6
		{
			".eslintrc",
			"linguist-language",
			"YAML",
		},
		// case 7
		{
			"/.eslintrc",
			"linguist-language",
			"YAML",
		},
	}

	for n, c := range cases {
		fa := attributes.ForFile(c.filepath)
		assert.Contains(t, fa, c.expectedKey, "case %d", n)
		assert.Equal(t, c.expectedValue, fa[c.expectedKey], "case %d", n)
	}
}

func TestForFileSecondWins(t *testing.T) {
	input := `*.txt foo
*.txt bar`

	attributes, err := ParseAttributes(strings.NewReader(input))
	assert.NoError(t, err)
	assert.Len(t, attributes, 2)

	fa := attributes.ForFile("test.txt")
	assert.Contains(t, fa, "bar")
	assert.NotContains(t, fa, "foo")
}
