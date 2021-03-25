// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderCSV(t *testing.T) {
	var parser Parser
	kases := map[string]string{
		"a":                         "<table class=\"table\"><tr><td>a</td></tr></table>",
		"1,2":                       "<table class=\"table\"><tr><td>1</td><td>2</td></tr></table>",
		"1;2":                       "<table class=\"table\"><tr><td>1</td><td>2</td></tr></table>",
		"1\t2":                      "<table class=\"table\"><tr><td>1</td><td>2</td></tr></table>",
		"1|2":                       "<table class=\"table\"><tr><td>1</td><td>2</td></tr></table>",
		"1,2,3;4,5,6;7,8,9\na;b;c":  "<table class=\"table\"><tr><td>1,2,3</td><td>4,5,6</td><td>7,8,9</td></tr><tr><td>a</td><td>b</td><td>c</td></tr></table>",
		"\"1,2,3,4\";\"a\nb\"\nc;d": "<table class=\"table\"><tr><td>1,2,3,4</td><td>a\nb</td></tr><tr><td>c</td><td>d</td></tr></table>",
		"<br/>":                     "<table class=\"table\"><tr><td>&lt;br/&gt;</td></tr></table>",
	}

	for k, v := range kases {
		res := parser.Render([]byte(k), "", nil, false)
		assert.EqualValues(t, v, string(res))
	}
}
