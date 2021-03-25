// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type indexerMatchList struct {
	value    string
	position int
}

func Test_newIndexerGlobSettings(t *testing.T) {
	checkGlobMatch(t, "", []indexerMatchList{})
	checkGlobMatch(t, "     ", []indexerMatchList{})
	checkGlobMatch(t, "data, */data, */data/*, **/data/*, **/data/**", []indexerMatchList{
		{"", -1},
		{"don't", -1},
		{"data", 0},
		{"/data", 1},
		{"x/data", 1},
		{"x/data/y", 2},
		{"a/b/c/data/z", 3},
		{"a/b/c/data/x/y/z", 4},
	})
	checkGlobMatch(t, "*.txt, txt, **.txt, **txt, **txt*", []indexerMatchList{
		{"my.txt", 0},
		{"don't", -1},
		{"mytxt", 3},
		{"/data/my.txt", 2},
		{"data/my.txt", 2},
		{"data/txt", 3},
		{"data/thistxtfile", 4},
		{"/data/thistxtfile", 4},
	})
	checkGlobMatch(t, "data/**/*.txt, data/**.txt", []indexerMatchList{
		{"data/a/b/c/d.txt", 0},
		{"data/a.txt", 1},
	})
	checkGlobMatch(t, "**/*.txt, data/**.txt", []indexerMatchList{
		{"data/a/b/c/d.txt", 0},
		{"data/a.txt", 0},
		{"a.txt", -1},
	})
}

func checkGlobMatch(t *testing.T, globstr string, list []indexerMatchList) {
	glist := IndexerGlobFromString(globstr)
	if len(list) == 0 {
		assert.Empty(t, glist)
		return
	}
	assert.NotEmpty(t, glist)
	for _, m := range list {
		found := false
		for pos, g := range glist {
			if g.Match(m.value) {
				assert.Equal(t, m.position, pos, "Test string `%s` doesn't match `%s`@%d, but matches @%d", m.value, globstr, m.position, pos)
				found = true
				break
			}
		}
		if !found {
			assert.Equal(t, m.position, -1, "Test string `%s` doesn't match `%s` anywhere; expected @%d", m.value, globstr, m.position)
		}
	}
}
