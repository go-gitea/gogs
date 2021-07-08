// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import "testing"

func TestShellEscape(t *testing.T) {
	tests := []struct {
		name     string
		toEscape string
		want     string
	}{
		{
			"Simplest case - nothing to escape",
			"a/b/c/d",
			"a/b/c/d",
		}, {
			"Prefixed tilde - with normal stuff - should not escape",
			"~/src/go/gitea/gitea",
			"~/src/go/gitea/gitea",
		}, {
			"Typical windows path with spaces - should get doublequote escaped",
			`C:\Program Files\Gitea v1.13 - I like lots of spaces\gitea`,
			`"C:\\Program Files\\Gitea v1.13 - I like lots of spaces\\gitea"`,
		}, {
			"Forward-slashed windows path with spaces - should get doublequote escaped",
			"C:/Program Files/Gitea v1.13 - I like lots of spaces/gitea",
			`"C:/Program Files/Gitea v1.13 - I like lots of spaces/gitea"`,
		}, {
			"Prefixed tilde - but then a space filled path",
			"~git/Gitea v1.13/gitea",
			`~git/"Gitea v1.13/gitea"`,
		}, {
			"Bangs are unfortunately not predictable so need to be singlequoted",
			"C:/Program Files/Gitea!/gitea",
			`'C:/Program Files/Gitea!/gitea'`,
		}, {
			"Newlines are just irritating",
			"/home/git/Gitea\n\nWHY-WOULD-YOU-DO-THIS\n\nGitea/gitea",
			"'/home/git/Gitea\n\nWHY-WOULD-YOU-DO-THIS\n\nGitea/gitea'",
		}, {
			"Similarly we should nicely handle multiple single quotes if we have to single-quote",
			"'!''!'''!''!'!'",
			`\''!'\'\''!'\'\'\''!'\'\''!'\''!'\'`,
		}, {
			"Double quote < ...",
			"~/<gitea",
			"~/\"<gitea\"",
		}, {
			"Double quote > ...",
			"~/gitea>",
			"~/\"gitea>\"",
		}, {
			"Double quote and escape $ ...",
			"~/$gitea",
			"~/\"\\$gitea\"",
		}, {
			"Double quote {...",
			"~/{gitea",
			"~/\"{gitea\"",
		}, {
			"Double quote }...",
			"~/gitea}",
			"~/\"gitea}\"",
		}, {
			"Double quote ()...",
			"~/(gitea)",
			"~/\"(gitea)\"",
		}, {
			"Double quote and escape `...",
			"~/gitea`",
			"~/\"gitea\\`\"",
		}, {
			"Double quotes can handle a number of things without having to escape them but not everything ...",
			"~/<gitea> ${gitea} `gitea` [gitea] (gitea) \"gitea\" \\gitea\\ 'gitea'",
			"~/\"<gitea> \\${gitea} \\`gitea\\` [gitea] (gitea) \\\"gitea\\\" \\\\gitea\\\\ 'gitea'\"",
		}, {
			"Single quotes don't need to escape except for '...",
			"~/<gitea> ${gitea} `gitea` (gitea) !gitea! \"gitea\" \\gitea\\ 'gitea'",
			"~/'<gitea> ${gitea} `gitea` (gitea) !gitea! \"gitea\" \\gitea\\ '\\''gitea'\\'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShellEscape(tt.toEscape); got != tt.want {
				t.Errorf("ShellEscape(%q):\nGot:    %s\nWanted: %s", tt.toEscape, got, tt.want)
			}
		})
	}
}
