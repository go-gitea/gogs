// Copyright 2019 Yusuke Inuzuka
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Most of what follows is a subtly changed version of github.com/yuin/goldmark/extension/footnote.go

package common

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// CleanValue will clean a value to make it safe to be an id
// This function is quite different from the original goldmark function
// and more closely matches the output from the shurcooL sanitizer
// In particular Unicode letters and numbers are a lot more than a-zA-Z0-9...
func CleanValue(value []byte) []byte {
	value = bytes.TrimSpace(value)
	rs := bytes.Runes(value)
	result := make([]rune, 0, len(rs))
	needsDash := false
	for _, r := range rs {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_':
			if needsDash && len(result) > 0 {
				result = append(result, '-')
			}
			needsDash = false
			result = append(result, unicode.ToLower(r))
		default:
			needsDash = true
		}
	}
	return []byte(string(result))
}

// Most of what follows is a subtly changed version of github.com/yuin/goldmark/extension/footnote.go

// A FootnoteLink struct represents a link to a footnote of Markdown
// (PHP Markdown Extra) text.
type FootnoteLink struct {
	ast.BaseInline
	Index int
	Name  []byte
}

// Dump implements Node.Dump.
func (n *FootnoteLink) Dump(source []byte, level int) {
	m := map[string]string{}
	m["Index"] = fmt.Sprintf("%v", n.Index)
	m["Name"] = fmt.Sprintf("%v", n.Name)
	ast.DumpHelper(n, source, level, m, nil)
}

// KindFootnoteLink is a NodeKind of the FootnoteLink node.
var KindFootnoteLink = ast.NewNodeKind("GiteaFootnoteLink")

// Kind implements Node.Kind.
func (n *FootnoteLink) Kind() ast.NodeKind {
	return KindFootnoteLink
}

// NewFootnoteLink returns a new FootnoteLink node.
func NewFootnoteLink(index int, name []byte) *FootnoteLink {
	return &FootnoteLink{
		Index: index,
		Name:  name,
	}
}

// A FootnoteBackLink struct represents a link to a footnote of Markdown
// (PHP Markdown Extra) text.
type FootnoteBackLink struct {
	ast.BaseInline
	Index int
	Name  []byte
}

// Dump implements Node.Dump.
func (n *FootnoteBackLink) Dump(source []byte, level int) {
	m := map[string]string{}
	m["Index"] = fmt.Sprintf("%v", n.Index)
	m["Name"] = fmt.Sprintf("%v", n.Name)
	ast.DumpHelper(n, source, level, m, nil)
}

// KindFootnoteBackLink is a NodeKind of the FootnoteBackLink node.
var KindFootnoteBackLink = ast.NewNodeKind("GiteaFootnoteBackLink")

// Kind implements Node.Kind.
func (n *FootnoteBackLink) Kind() ast.NodeKind {
	return KindFootnoteBackLink
}

// NewFootnoteBackLink returns a new FootnoteBackLink node.
func NewFootnoteBackLink(index int, name []byte) *FootnoteBackLink {
	return &FootnoteBackLink{
		Index: index,
		Name:  name,
	}
}

// A Footnote struct represents a footnote of Markdown
// (PHP Markdown Extra) text.
type Footnote struct {
	ast.BaseBlock
	Ref   []byte
	Index int
	Name  []byte
}

// Dump implements Node.Dump.
func (n *Footnote) Dump(source []byte, level int) {
	m := map[string]string{}
	m["Index"] = fmt.Sprintf("%v", n.Index)
	m["Ref"] = fmt.Sprintf("%s", n.Ref)
	m["Name"] = fmt.Sprintf("%v", n.Name)
	ast.DumpHelper(n, source, level, m, nil)
}

// KindFootnote is a NodeKind of the Footnote node.
var KindFootnote = ast.NewNodeKind("GiteaFootnote")

// Kind implements Node.Kind.
func (n *Footnote) Kind() ast.NodeKind {
	return KindFootnote
}

// NewFootnote returns a new Footnote node.
func NewFootnote(ref []byte) *Footnote {
	return &Footnote{
		Ref:   ref,
		Index: -1,
		Name:  ref,
	}
}

// A FootnoteList struct represents footnotes of Markdown
// (PHP Markdown Extra) text.
type FootnoteList struct {
	ast.BaseBlock
	Count int
}

// Dump implements Node.Dump.
func (n *FootnoteList) Dump(source []byte, level int) {
	m := map[string]string{}
	m["Count"] = fmt.Sprintf("%v", n.Count)
	ast.DumpHelper(n, source, level, m, nil)
}

// KindFootnoteList is a NodeKind of the FootnoteList node.
var KindFootnoteList = ast.NewNodeKind("GiteaFootnoteList")

// Kind implements Node.Kind.
func (n *FootnoteList) Kind() ast.NodeKind {
	return KindFootnoteList
}

// NewFootnoteList returns a new FootnoteList node.
func NewFootnoteList() *FootnoteList {
	return &FootnoteList{
		Count: 0,
	}
}

var footnoteListKey = parser.NewContextKey()

type footnoteBlockParser struct{}

var defaultFootnoteBlockParser = &footnoteBlockParser{}

// NewFootnoteBlockParser returns a new parser.BlockParser that can parse
// footnotes of the Markdown(PHP Markdown Extra) text.
func NewFootnoteBlockParser() parser.BlockParser {
	return defaultFootnoteBlockParser
}

func (b *footnoteBlockParser) Trigger() []byte {
	return []byte{'['}
}

func (b *footnoteBlockParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || line[pos] != '[' {
		return nil, parser.NoChildren
	}
	pos++
	if pos > len(line)-1 || line[pos] != '^' {
		return nil, parser.NoChildren
	}
	open := pos + 1
	closes := 0
	closure := util.FindClosure(line[pos+1:], '[', ']', false, false)
	closes = pos + 1 + closure
	next := closes + 1
	if closure > -1 {
		if next >= len(line) || line[next] != ':' {
			return nil, parser.NoChildren
		}
	} else {
		return nil, parser.NoChildren
	}
	padding := segment.Padding
	label := reader.Value(text.NewSegment(segment.Start+open-padding, segment.Start+closes-padding))
	if util.IsBlank(label) {
		return nil, parser.NoChildren
	}
	item := NewFootnote(label)

	pos = next + 1 - padding
	if pos >= len(line) {
		reader.Advance(pos)
		return item, parser.NoChildren
	}
	reader.AdvanceAndSetPadding(pos, padding)
	return item, parser.HasChildren
}

func (b *footnoteBlockParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	line, _ := reader.PeekLine()
	if util.IsBlank(line) {
		return parser.Continue | parser.HasChildren
	}
	childpos, padding := util.IndentPosition(line, reader.LineOffset(), 4)
	if childpos < 0 {
		return parser.Close
	}
	reader.AdvanceAndSetPadding(childpos, padding)
	return parser.Continue | parser.HasChildren
}

func (b *footnoteBlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	var list *FootnoteList
	if tlist := pc.Get(footnoteListKey); tlist != nil {
		list = tlist.(*FootnoteList)
	} else {
		list = NewFootnoteList()
		pc.Set(footnoteListKey, list)
		node.Parent().InsertBefore(node.Parent(), node, list)
	}
	node.Parent().RemoveChild(node.Parent(), node)
	list.AppendChild(list, node)
}

func (b *footnoteBlockParser) CanInterruptParagraph() bool {
	return true
}

func (b *footnoteBlockParser) CanAcceptIndentedLine() bool {
	return false
}

type footnoteParser struct{}

var defaultFootnoteParser = &footnoteParser{}

// NewFootnoteParser returns a new parser.InlineParser that can parse
// footnote links of the Markdown(PHP Markdown Extra) text.
func NewFootnoteParser() parser.InlineParser {
	return defaultFootnoteParser
}

func (s *footnoteParser) Trigger() []byte {
	// footnote syntax probably conflict with the image syntax.
	// So we need trigger this parser with '!'.
	return []byte{'!', '['}
}

func (s *footnoteParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, segment := block.PeekLine()
	pos := 1
	if len(line) > 0 && line[0] == '!' {
		pos++
	}
	if pos >= len(line) || line[pos] != '^' {
		return nil
	}
	pos++
	if pos >= len(line) {
		return nil
	}
	open := pos
	closure := util.FindClosure(line[pos:], '[', ']', false, false)
	if closure < 0 {
		return nil
	}
	closes := pos + closure
	value := block.Value(text.NewSegment(segment.Start+open, segment.Start+closes))
	block.Advance(closes + 1)

	var list *FootnoteList
	if tlist := pc.Get(footnoteListKey); tlist != nil {
		list = tlist.(*FootnoteList)
	}
	if list == nil {
		return nil
	}
	index := 0
	name := []byte{}
	for def := list.FirstChild(); def != nil; def = def.NextSibling() {
		d := def.(*Footnote)
		if bytes.Equal(d.Ref, value) {
			if d.Index < 0 {
				list.Count++
				d.Index = list.Count
				val := CleanValue(d.Name)
				if len(val) == 0 {
					val = []byte(strconv.Itoa(d.Index))
				}
				d.Name = pc.IDs().Generate(val, KindFootnote)
			}
			index = d.Index
			name = d.Name
			break
		}
	}
	if index == 0 {
		return nil
	}

	return NewFootnoteLink(index, name)
}

type footnoteASTTransformer struct{}

var defaultFootnoteASTTransformer = &footnoteASTTransformer{}

// NewFootnoteASTTransformer returns a new parser.ASTTransformer that
// insert a footnote list to the last of the document.
func NewFootnoteASTTransformer() parser.ASTTransformer {
	return defaultFootnoteASTTransformer
}

func (a *footnoteASTTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	var list *FootnoteList
	if tlist := pc.Get(footnoteListKey); tlist != nil {
		list = tlist.(*FootnoteList)
	} else {
		return
	}
	pc.Set(footnoteListKey, nil)
	for footnote := list.FirstChild(); footnote != nil; {
		var container ast.Node = footnote
		next := footnote.NextSibling()
		if fc := container.LastChild(); fc != nil && ast.IsParagraph(fc) {
			container = fc
		}
		footnoteNode := footnote.(*Footnote)
		index := footnoteNode.Index
		name := footnoteNode.Name
		if index < 0 {
			list.RemoveChild(list, footnote)
		} else {
			container.AppendChild(container, NewFootnoteBackLink(index, name))
		}
		footnote = next
	}
	list.SortChildren(func(n1, n2 ast.Node) int {
		if n1.(*Footnote).Index < n2.(*Footnote).Index {
			return -1
		}
		return 1
	})
	if list.Count <= 0 {
		list.Parent().RemoveChild(list.Parent(), list)
		return
	}

	node.AppendChild(node, list)
}

// FootnoteHTMLRenderer is a renderer.NodeRenderer implementation that
// renders FootnoteLink nodes.
type FootnoteHTMLRenderer struct {
	html.Config
}

// NewFootnoteHTMLRenderer returns a new FootnoteHTMLRenderer.
func NewFootnoteHTMLRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &FootnoteHTMLRenderer{
		Config: html.NewConfig(),
	}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs implements renderer.NodeRenderer.RegisterFuncs.
func (r *FootnoteHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindFootnoteLink, r.renderFootnoteLink)
	reg.Register(KindFootnoteBackLink, r.renderFootnoteBackLink)
	reg.Register(KindFootnote, r.renderFootnote)
	reg.Register(KindFootnoteList, r.renderFootnoteList)
}

func (r *FootnoteHTMLRenderer) renderFootnoteLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*FootnoteLink)
		n.Dump(source, 0)
		is := strconv.Itoa(n.Index)
		_, _ = w.WriteString(`<sup id="fnref:`)
		_, _ = w.Write(n.Name)
		_, _ = w.WriteString(`"><a href="#fn:`)
		_, _ = w.Write(n.Name)
		_, _ = w.WriteString(`" class="footnote-ref" role="doc-noteref">`)
		_, _ = w.WriteString(is)
		_, _ = w.WriteString(`</a></sup>`)
	}
	return ast.WalkContinue, nil
}

func (r *FootnoteHTMLRenderer) renderFootnoteBackLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		n := node.(*FootnoteBackLink)
		fmt.Fprintf(os.Stdout, "source:\n%s\n", string(n.Text(source)))
		_, _ = w.WriteString(` <a href="#fnref:`)
		_, _ = w.Write(n.Name)
		_, _ = w.WriteString(`" class="footnote-backref" role="doc-backlink">`)
		_, _ = w.WriteString("&#x21a9;&#xfe0e;")
		_, _ = w.WriteString(`</a>`)
	}
	return ast.WalkContinue, nil
}

func (r *FootnoteHTMLRenderer) renderFootnote(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*Footnote)
	if entering {
		fmt.Fprintf(os.Stdout, "source:\n%s\n", string(n.Text(source)))
		_, _ = w.WriteString(`<li id="fn:`)
		_, _ = w.Write(n.Name)
		_, _ = w.WriteString(`" role="doc-endnote"`)
		if node.Attributes() != nil {
			html.RenderAttributes(w, node, html.ListItemAttributeFilter)
		}
		_, _ = w.WriteString(">\n")
	} else {
		_, _ = w.WriteString("</li>\n")
	}
	return ast.WalkContinue, nil
}

func (r *FootnoteHTMLRenderer) renderFootnoteList(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	tag := "div"
	if entering {
		_, _ = w.WriteString("<")
		_, _ = w.WriteString(tag)
		_, _ = w.WriteString(` class="footnotes" role="doc-endnotes"`)
		if node.Attributes() != nil {
			html.RenderAttributes(w, node, html.GlobalAttributeFilter)
		}
		_ = w.WriteByte('>')
		if r.Config.XHTML {
			_, _ = w.WriteString("\n<hr />\n")
		} else {
			_, _ = w.WriteString("\n<hr>\n")
		}
		_, _ = w.WriteString("<ol>\n")
	} else {
		_, _ = w.WriteString("</ol>\n")
		_, _ = w.WriteString("</")
		_, _ = w.WriteString(tag)
		_, _ = w.WriteString(">\n")
	}
	return ast.WalkContinue, nil
}

type footnoteExtension struct{}

// FootnoteExtension represents the Gitea Footnote
var FootnoteExtension = &footnoteExtension{}

// Extend extends the markdown converter with the Gitea Footnote parser
func (e *footnoteExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithBlockParsers(
			util.Prioritized(NewFootnoteBlockParser(), 999),
		),
		parser.WithInlineParsers(
			util.Prioritized(NewFootnoteParser(), 101),
		),
		parser.WithASTTransformers(
			util.Prioritized(NewFootnoteASTTransformer(), 999),
		),
	)
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(NewFootnoteHTMLRenderer(), 500),
	))
}
