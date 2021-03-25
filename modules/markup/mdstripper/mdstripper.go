// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mdstripper

import (
	"bytes"
	"io"
	"net/url"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup/common"
	"code.gitea.io/gitea/modules/setting"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

var (
	giteaHostInit sync.Once
	giteaHost     *url.URL
)

type stripRenderer struct {
	localhost *url.URL
	links     []string
	empty     bool
}

func (r *stripRenderer) Render(w io.Writer, source []byte, doc ast.Node) error {
	return ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Text:
			if !v.IsRaw() {
				_, prevSibIsText := n.PreviousSibling().(*ast.Text)
				coalesce := prevSibIsText
				r.processString(
					w,
					v.Text(source),
					coalesce)
				if v.SoftLineBreak() {
					r.doubleSpace(w)
				}
			}
			return ast.WalkContinue, nil
		case *ast.Link:
			r.processLink(w, v.Destination)
			return ast.WalkSkipChildren, nil
		case *ast.AutoLink:
			// This could be a reference to an issue or pull - if so convert it
			r.processAutoLink(w, v.URL(source))
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
}

func (r *stripRenderer) doubleSpace(w io.Writer) {
	if !r.empty {
		_, _ = w.Write([]byte{'\n'})
	}
}

func (r *stripRenderer) processString(w io.Writer, text []byte, coalesce bool) {
	// Always break-up words
	if !coalesce {
		r.doubleSpace(w)
	}
	_, _ = w.Write(text)
	r.empty = false
}

// ProcessAutoLinks to detect and handle links to issues and pulls
func (r *stripRenderer) processAutoLink(w io.Writer, link []byte) {
	linkStr := string(link)
	u, err := url.Parse(linkStr)
	if err != nil {
		// Process out of band
		r.links = append(r.links, linkStr)
		return
	}

	// Note: we're not attempting to match the URL scheme (http/https)
	host := strings.ToLower(u.Host)
	if host != "" && host != strings.ToLower(r.localhost.Host) {
		// Process out of band
		r.links = append(r.links, linkStr)
		return
	}

	// We want: /user/repo/issues/3
	parts := strings.Split(strings.TrimPrefix(u.EscapedPath(), r.localhost.EscapedPath()), "/")
	if len(parts) != 5 || parts[0] != "" {
		// Process out of band
		r.links = append(r.links, linkStr)
		return
	}

	var sep string
	if parts[3] == "issues" {
		sep = "#"
	} else if parts[3] == "pulls" {
		sep = "!"
	} else {
		// Process out of band
		r.links = append(r.links, linkStr)
		return
	}

	_, _ = w.Write([]byte(parts[1]))
	_, _ = w.Write([]byte("/"))
	_, _ = w.Write([]byte(parts[2]))
	_, _ = w.Write([]byte(sep))
	_, _ = w.Write([]byte(parts[4]))
}

func (r *stripRenderer) processLink(w io.Writer, link []byte) {
	// Links are processed out of band
	r.links = append(r.links, string(link))
}

// GetLinks returns the list of link data collected while parsing
func (r *stripRenderer) GetLinks() []string {
	return r.links
}

// AddOptions adds given option to this renderer.
func (r *stripRenderer) AddOptions(...renderer.Option) {
	// no-op
}

// StripMarkdown parses markdown content by removing all markup and code blocks
//	in order to extract links and other references
func StripMarkdown(rawBytes []byte) (string, []string) {
	buf, links := StripMarkdownBytes(rawBytes)
	return string(buf), links
}

var (
	stripParser parser.Parser
	once        = sync.Once{}
)

// StripMarkdownBytes parses markdown content by removing all markup and code blocks
//	in order to extract links and other references
func StripMarkdownBytes(rawBytes []byte) ([]byte, []string) {
	once.Do(func() {
		gdMarkdown := goldmark.New(
			goldmark.WithExtensions(extension.Table,
				extension.Strikethrough,
				extension.TaskList,
				extension.DefinitionList,
				common.FootnoteExtension,
				common.Linkify,
			),
			goldmark.WithParserOptions(
				parser.WithAttribute(),
				parser.WithAutoHeadingID(),
			),
			goldmark.WithRendererOptions(
				html.WithUnsafe(),
			),
		)
		stripParser = gdMarkdown.Parser()
	})
	stripper := &stripRenderer{
		localhost: getGiteaHost(),
		links:     make([]string, 0, 10),
		empty:     true,
	}
	reader := text.NewReader(rawBytes)
	doc := stripParser.Parse(reader)
	var buf bytes.Buffer
	if err := stripper.Render(&buf, rawBytes, doc); err != nil {
		log.Error("Unable to strip: %v", err)
	}
	return buf.Bytes(), stripper.GetLinks()
}

// getGiteaHostName returns a normalized string with the local host name, with no scheme or port information
func getGiteaHost() *url.URL {
	giteaHostInit.Do(func() {
		var err error
		if giteaHost, err = url.Parse(setting.AppURL); err != nil {
			giteaHost = &url.URL{}
		}
	})
	return giteaHost
}
