// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import "strings"

// Bash has the definition of a metacharacter:
// * A character that, when unquoted, separates words.
//   A metacharacter is one of: " \t\n|&;()<>"
//
// The following characters also have addition special meaning when unescaped:
// * ‘${[*?!"'`\’
//
// Double Quotes preserve the literal value of all characters with then quotes
// excepting: ‘$’, ‘`’, ‘\’, and, when history expansion is enabled, ‘!’.
// The backslash retains its special meaning only when followed by one of the
// following characters: ‘$’, ‘`’, ‘"’, ‘\’, or newline.
// Backslashes preceding characters without a special meaning are left
// unmodified. A double quote may be quoted within double quotes by preceding
// it with a backslash. If enabled, history expansion will be performed unless
// an ‘!’ appearing in double quotes is escaped using a backslash. The
// backslash preceding the ‘!’ is not removed.
//
// -> This means that `!\n` cannot be safely expressed in `"`.
//
// Looking at the man page for Dash and ash the situation is similar.
//
// Now zsh requires that ‘}’, and ‘]’ are also enclosed in doublequotes or escaped
//
// Single quotes escape everything except a ‘'’
//
// There's one other gotcha - ‘~’ at the start of a string needs to be expanded
// because people always expect that - of course if there is a special character before '/'
// this is not going to work

const (
	tildePrefix      = '~'
	needsEscape      = " \t\n|&;()<>${}[]*?!\"'`\\"
	needsSingleQuote = "!\n"
)

var (
	doubleQuoteEscaper   = strings.NewReplacer(`$`, `\$`, "`", "\\`", `"`, `\"`, `\`, `\\`)
	singleQuoteEscaper   = strings.NewReplacer(`'`, `'\''`)
	singleQuoteCoalescer = strings.NewReplacer(`''\'`, `\'`, `\'''`, `\'`)
)

// ShellEscape will escape the provided string.
// We can't just use go-shellquote here because our preferences for escaping differ from those in that we want:
//
// * If the string doesn't require any escaping just leave it as it is.
// * If the string requires any escaping prefer double quote escaping
// * If we have ! or newlines then we need to use single quote escaping
func ShellEscape(toEscape string) string {
	if len(toEscape) == 0 {
		return toEscape
	}

	start := 0

	if toEscape[0] == tildePrefix {
		// We're in the forcibly non-escaped section...
		idx := strings.IndexRune(toEscape, '/')
		if idx < 0 {
			idx = len(toEscape)
		} else {
			idx++
		}
		if !strings.ContainsAny(toEscape[:idx], needsEscape) {
			// We'll assume that they intend ~ expansion to occur
			start = idx
		}
	}

	// Now for simplicity we'll look at the rest of the string
	if !strings.ContainsAny(toEscape[start:], needsEscape) {
		return toEscape
	}

	// OK we have to do some escaping
	sb := &strings.Builder{}
	_, _ = sb.WriteString(toEscape[:start])

	// Do we have any characters which absolutely need to be within single quotes - that is simply ! or \n?
	if strings.ContainsAny(toEscape[start:], needsSingleQuote) {
		// We need to single quote escape.
		sb2 := &strings.Builder{}
		_, _ = sb2.WriteRune('\'')
		_, _ = singleQuoteEscaper.WriteString(sb2, toEscape[start:])
		_, _ = sb2.WriteRune('\'')
		_, _ = singleQuoteCoalescer.WriteString(sb, sb2.String())
		return sb.String()
	}

	// OK we can just use " just escape the things that need escaping
	_, _ = sb.WriteRune('"')
	_, _ = doubleQuoteEscaper.WriteString(sb, toEscape[start:])
	_, _ = sb.WriteRune('"')
	return sb.String()
}
