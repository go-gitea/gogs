// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/common"
	"code.gitea.io/gitea/modules/setting"
	giteautil "code.gitea.io/gitea/modules/util"

	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var byteMailto = []byte("mailto:")

// Header holds the data about a header.
type Header struct {
	Level int
	Text  string
	ID    string
}

// ASTTransformer is a default transformer of the goldmark tree.
type ASTTransformer struct{}

// Transform transforms the given AST tree.
func (g *ASTTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	metaData := meta.GetItems(pc)
	firstChild := node.FirstChild()
	createTOC := false
	toc := []Header{}
	rc := &RenderConfig{
		Meta: "table",
		Icon: "table",
		Lang: "",
	}
	if metaData != nil {
		rc.ToRenderConfig(metaData)

		metaNode := rc.toMetaNode(metaData)
		if metaNode != nil {
			node.InsertBefore(node, firstChild, metaNode)
		}
		createTOC = rc.TOC
		toc = make([]Header, 0, 100)
	}

	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch v := n.(type) {
		case *ast.Heading:
			if createTOC {
				text := n.Text(reader.Source())
				header := Header{
					Text:  util.BytesToReadOnlyString(text),
					Level: v.Level,
				}
				if id, found := v.AttributeString("id"); found {
					header.ID = util.BytesToReadOnlyString(id.([]byte))
				}
				toc = append(toc, header)
			} else {
				for _, attr := range v.Attributes() {
					if _, ok := attr.Value.([]byte); !ok {
						v.SetAttribute(attr.Name, []byte(fmt.Sprintf("%v", attr.Value)))
					}
				}
			}
		case *ast.Image:
			// Images need two things:
			//
			// 1. Their src needs to munged to be a real value
			// 2. If they're not wrapped with a link they need a link wrapper

			// Check if the destination is a real link
			link := v.Destination
			if len(link) > 0 && !markup.IsLink(link) {
				prefix := pc.Get(urlPrefixKey).(string)
				if pc.Get(isWikiKey).(bool) {
					prefix = giteautil.URLJoin(prefix, "wiki", "raw")
				}
				prefix = strings.Replace(prefix, "/src/", "/media/", 1)

				lnk := string(link)
				lnk = giteautil.URLJoin(prefix, lnk)
				link = []byte(lnk)
			}
			v.Destination = link

			parent := n.Parent()
			// Create a link around image only if parent is not already a link
			if _, ok := parent.(*ast.Link); !ok && parent != nil {
				next := n.NextSibling()

				// Create a link wrapper
				wrap := ast.NewLink()
				wrap.Destination = link
				wrap.Title = v.Title

				// Duplicate the current image node
				image := ast.NewImage(ast.NewLink())
				image.Destination = link
				image.Title = v.Title
				for _, attr := range v.Attributes() {
					image.SetAttribute(attr.Name, attr.Value)
				}
				for child := v.FirstChild(); child != nil; {
					next := child.NextSibling()
					image.AppendChild(image, child)
					child = next
				}

				// Append our duplicate image to the wrapper link
				wrap.AppendChild(wrap, image)

				// Wire in the next sibling
				wrap.SetNextSibling(next)

				// Replace the current node with the wrapper link
				parent.ReplaceChild(parent, n, wrap)

				// But most importantly ensure the next sibling is still on the old image too
				v.SetNextSibling(next)
			}
		case *ast.Link:
			// Links need their href to munged to be a real value
			link := v.Destination
			if len(link) > 0 && !markup.IsLink(link) &&
				link[0] != '#' && !bytes.HasPrefix(link, byteMailto) {
				// special case: this is not a link, a hash link or a mailto:, so it's a
				// relative URL
				lnk := string(link)
				if pc.Get(isWikiKey).(bool) {
					lnk = giteautil.URLJoin("wiki", lnk)
				}
				link = []byte(giteautil.URLJoin(pc.Get(urlPrefixKey).(string), lnk))
			}
			if len(link) > 0 && link[0] == '#' {
				link = []byte("#user-content-" + string(link)[1:])
			}
			v.Destination = link
		case *ast.List:
			if v.HasChildren() {
				children := make([]ast.Node, 0, v.ChildCount())
				child := v.FirstChild()
				for child != nil {
					children = append(children, child)
					child = child.NextSibling()
				}
				v.RemoveChildren(v)

				for _, child := range children {
					listItem := child.(*ast.ListItem)
					if !child.HasChildren() || !child.FirstChild().HasChildren() {
						v.AppendChild(v, child)
						continue
					}
					taskCheckBox, ok := child.FirstChild().FirstChild().(*east.TaskCheckBox)
					if !ok {
						v.AppendChild(v, child)
						continue
					}
					newChild := NewTaskCheckBoxListItem(listItem)
					newChild.IsChecked = taskCheckBox.IsChecked
					newChild.SetAttributeString("class", []byte("task-list-item"))
					v.AppendChild(v, newChild)
				}
			}
		case *ast.Text:
			if v.SoftLineBreak() && !v.HardLineBreak() {
				renderMetas := pc.Get(renderMetasKey).(map[string]string)
				mode := renderMetas["mode"]
				if mode != "document" {
					v.SetHardLineBreak(setting.Markdown.EnableHardLineBreakInComments)
				} else {
					v.SetHardLineBreak(setting.Markdown.EnableHardLineBreakInDocuments)
				}
			}
		}
		return ast.WalkContinue, nil
	})

	if createTOC && len(toc) > 0 {
		lang := rc.Lang
		if len(lang) == 0 {
			lang = setting.Langs[0]
		}
		tocNode := createTOCNode(toc, lang)
		if tocNode != nil {
			node.InsertBefore(node, firstChild, tocNode)
		}
	}

	if len(rc.Lang) > 0 {
		node.SetAttributeString("lang", []byte(rc.Lang))
	}
}

type prefixedIDs struct {
	values map[string]bool
}

// Generate generates a new element id.
func (p *prefixedIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	dft := []byte("id")
	if kind == ast.KindHeading {
		dft = []byte("heading")
	}
	return p.GenerateWithDefault(value, dft)
}

// Generate generates a new element id.
func (p *prefixedIDs) GenerateWithDefault(value, dft []byte) []byte {
	result := common.CleanValue(value)
	if len(result) == 0 {
		result = dft
	}
	if !bytes.HasPrefix(result, []byte("user-content-")) {
		result = append([]byte("user-content-"), result...)
	}
	if _, ok := p.values[util.BytesToReadOnlyString(result)]; !ok {
		p.values[util.BytesToReadOnlyString(result)] = true
		return result
	}
	for i := 1; ; i++ {
		newResult := fmt.Sprintf("%s-%d", result, i)
		if _, ok := p.values[newResult]; !ok {
			p.values[newResult] = true
			return []byte(newResult)
		}
	}
}

// Put puts a given element id to the used ids table.
func (p *prefixedIDs) Put(value []byte) {
	p.values[util.BytesToReadOnlyString(value)] = true
}

func newPrefixedIDs() *prefixedIDs {
	return &prefixedIDs{
		values: map[string]bool{},
	}
}

// NewHTMLRenderer creates a HTMLRenderer to render
// in the gitea form.
func NewHTMLRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &HTMLRenderer{
		Config: html.NewConfig(),
	}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// HTMLRenderer is a renderer.NodeRenderer implementation that
// renders gitea specific features.
type HTMLRenderer struct {
	html.Config
}

// RegisterFuncs implements renderer.NodeRenderer.RegisterFuncs.
func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindDocument, r.renderDocument)
	reg.Register(KindDetails, r.renderDetails)
	reg.Register(KindSummary, r.renderSummary)
	reg.Register(KindIcon, r.renderIcon)
	reg.Register(KindTaskCheckBoxListItem, r.renderTaskCheckBoxListItem)
	reg.Register(east.KindTaskCheckBox, r.renderTaskCheckBox)
}

func (r *HTMLRenderer) renderDocument(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Document)

	if val, has := n.AttributeString("lang"); has {
		var err error
		if entering {
			_, err = w.WriteString("<div")
			if err == nil {
				_, err = w.WriteString(fmt.Sprintf(` lang=%q`, val))
			}
			if err == nil {
				_, err = w.WriteRune('>')
			}
		} else {
			_, err = w.WriteString("</div>")
		}

		if err != nil {
			return ast.WalkStop, err
		}
	}

	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderDetails(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	var err error
	if entering {
		_, err = w.WriteString("<details>")
	} else {
		_, err = w.WriteString("</details>")
	}

	if err != nil {
		return ast.WalkStop, err
	}

	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderSummary(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	var err error
	if entering {
		_, err = w.WriteString("<summary>")
	} else {
		_, err = w.WriteString("</summary>")
	}

	if err != nil {
		return ast.WalkStop, err
	}

	return ast.WalkContinue, nil
}

var validNameRE = regexp.MustCompile("^[a-z ]+$")

func (r *HTMLRenderer) renderIcon(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n := node.(*Icon)

	name := strings.TrimSpace(strings.ToLower(string(n.Name)))

	if len(name) == 0 {
		// skip this
		return ast.WalkContinue, nil
	}

	if !validNameRE.MatchString(name) {
		// skip this
		return ast.WalkContinue, nil
	}

	var err error
	_, err = w.WriteString(fmt.Sprintf(`<i class="icon %s"></i>`, name))

	if err != nil {
		return ast.WalkStop, err
	}

	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderTaskCheckBoxListItem(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*TaskCheckBoxListItem)
	if entering {
		if n.Attributes() != nil {
			_, _ = w.WriteString("<li")
			html.RenderAttributes(w, n, html.ListItemAttributeFilter)
			_ = w.WriteByte('>')
		} else {
			_, _ = w.WriteString("<li>")
		}
		end := ">"
		if r.XHTML {
			end = " />"
		}
		var err error
		if n.IsChecked {
			_, err = w.WriteString(`<input type="checkbox" disabled="" checked=""` + end)
		} else {
			_, err = w.WriteString(`<input type="checkbox" disabled=""` + end)
		}
		if err != nil {
			return ast.WalkStop, err
		}
		fc := n.FirstChild()
		if fc != nil {
			if _, ok := fc.(*ast.TextBlock); !ok {
				_ = w.WriteByte('\n')
			}
		}
	} else {
		_, _ = w.WriteString("</li>\n")
	}
	return ast.WalkContinue, nil
}

func (r *HTMLRenderer) renderTaskCheckBox(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}
