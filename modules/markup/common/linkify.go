// Copyright 2019 Yusuke Inuzuka
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Most of this file is a subtly changed version of github.com/yuin/goldmark/extension/linkify.go

package common

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var wwwURLRegxp = regexp.MustCompile(`^www\.[-a-zA-Z0-9@:%._\+~#=]{2,256}\.[a-z]{2,6}((?:/|[#?])[-a-zA-Z0-9@:%_\+.~#!?&//=\(\);,'">\^{}\[\]` + "`" + `]*)?`)

type linkifyParser struct{}

var defaultLinkifyParser = &linkifyParser{}

// NewLinkifyParser return a new InlineParser can parse
// text that seems like a URL.
func NewLinkifyParser() parser.InlineParser {
	return defaultLinkifyParser
}

func (s *linkifyParser) Trigger() []byte {
	// ' ' indicates any white spaces and a line head
	return []byte{' ', '*', '_', '~', '('}
}

var (
	protoHTTP  = []byte("http:")
	protoHTTPS = []byte("https:")
	protoFTP   = []byte("ftp:")
	domainWWW  = []byte("www.")
)

func (s *linkifyParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	if pc.IsInLinkLabel() {
		return nil
	}
	line, segment := block.PeekLine()
	consumes := 0
	start := segment.Start
	c := line[0]
	// advance if current position is not a line head.
	if c == ' ' || c == '*' || c == '_' || c == '~' || c == '(' {
		consumes++
		start++
		line = line[1:]
	}

	var m []int
	var protocol []byte
	var typ ast.AutoLinkType = ast.AutoLinkURL
	if bytes.HasPrefix(line, protoHTTP) || bytes.HasPrefix(line, protoHTTPS) || bytes.HasPrefix(line, protoFTP) {
		m = LinkRegex.FindSubmatchIndex(line)
	}
	if m == nil && bytes.HasPrefix(line, domainWWW) {
		m = wwwURLRegxp.FindSubmatchIndex(line)
		protocol = []byte("http")
	}
	if m != nil {
		lastChar := line[m[1]-1]
		if lastChar == '.' {
			m[1]--
		} else if lastChar == ')' {
			closing := 0
			for i := m[1] - 1; i >= m[0]; i-- {
				if line[i] == ')' {
					closing++
				} else if line[i] == '(' {
					closing--
				}
			}
			if closing > 0 {
				m[1] -= closing
			}
		} else if lastChar == ';' {
			i := m[1] - 2
			for ; i >= m[0]; i-- {
				if util.IsAlphaNumeric(line[i]) {
					continue
				}
				break
			}
			if i != m[1]-2 {
				if line[i] == '&' {
					m[1] -= m[1] - i
				}
			}
		}
	}
	if m == nil {
		if len(line) > 0 && util.IsPunct(line[0]) {
			return nil
		}
		typ = ast.AutoLinkEmail
		stop := util.FindEmailIndex(line)
		if stop < 0 {
			return nil
		}
		at := bytes.IndexByte(line, '@')
		m = []int{0, stop, at, stop - 1}
		if bytes.IndexByte(line[m[2]:m[3]], '.') < 0 {
			return nil
		}
		lastChar := line[m[1]-1]
		if lastChar == '.' {
			m[1]--
		}
		if m[1] < len(line) {
			nextChar := line[m[1]]
			if nextChar == '-' || nextChar == '_' {
				return nil
			}
		}
	}
	if m == nil {
		return nil
	}
	if consumes != 0 {
		s := segment.WithStop(segment.Start + 1)
		ast.MergeOrAppendTextSegment(parent, s)
	}
	consumes += m[1]
	block.Advance(consumes)
	n := ast.NewTextSegment(text.NewSegment(start, start+m[1]))
	link := ast.NewAutoLink(typ, n)
	link.Protocol = protocol
	return link
}

func (s *linkifyParser) CloseBlock(parent ast.Node, pc parser.Context) {
	// nothing to do
}

type linkify struct{}

// Linkify is an extension that allow you to parse text that seems like a URL.
var Linkify = &linkify{}

func (e *linkify) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(NewLinkifyParser(), 999),
		),
	)
}
