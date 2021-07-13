// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"strings"

	"code.gitea.io/gitea/modules/highlight"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/niklasfasching/go-org/org"
)

func init() {
	markup.RegisterRenderer(Renderer{})
}

// Renderer implements markup.Renderer for orgmode
type Renderer struct {
}

// Name implements markup.Renderer
func (Renderer) Name() string {
	return "orgmode"
}

// NeedPostProcess implements markup.Renderer
func (Renderer) NeedPostProcess() bool { return true }

// Extensions implements markup.Renderer
func (Renderer) Extensions() []string {
	return []string{".org"}
}

// SanitizerRules implements markup.Renderer
func (Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return []setting.MarkupSanitizerRule{}
}

// Render renders orgmode rawbytes to HTML
func Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	htmlWriter := org.NewHTMLWriter()
	htmlWriter.HighlightCodeBlock = func(source, lang string, inline bool) string {
		var w strings.Builder
		if _, err := w.WriteString(`<pre>`); err != nil {
			return ""
		}

		lexer := lexers.Get(lang)
		if lexer == nil && lang == "" {
			lexer = lexers.Analyse(source)
			if lexer == nil {
				lexer = lexers.Fallback
			}
			lang = strings.ToLower(lexer.Config().Name)
		}

		if lexer == nil {
			// include language-x class as part of commonmark spec
			if _, err := w.WriteString(`<code class="chroma language-` + string(lang) + `">`); err != nil {
				return ""
			}
			if _, err := w.WriteString(html.EscapeString(source)); err != nil {
				return ""
			}
		} else {
			// include language-x class as part of commonmark spec
			if _, err := w.WriteString(`<code class="chroma language-` + string(lang) + `">`); err != nil {
				return ""
			}
			lexer = chroma.Coalesce(lexer)

			if _, err := w.WriteString(highlight.Code(lexer.Config().Filenames[0], source)); err != nil {
				return ""
			}
		}

		if _, err := w.WriteString("</code></pre>"); err != nil {
			return ""
		}

		return w.String()
	}

	w := &Writer{
		HTMLWriter: htmlWriter,
		URLPrefix:  ctx.URLPrefix,
		IsWiki:     ctx.IsWiki,
	}

	htmlWriter.ExtendingWriter = w

	res, err := org.New().Silent().Parse(input, "").Write(w)
	if err != nil {
		return fmt.Errorf("orgmode.Render failed: %v", err)
	}
	_, err = io.Copy(output, strings.NewReader(res))
	return err
}

// RenderString renders orgmode string to HTML string
func RenderString(ctx *markup.RenderContext, content string) (string, error) {
	var buf strings.Builder
	if err := Render(ctx, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Render renders orgmode string to HTML string
func (Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	return Render(ctx, input, output)
}

// Writer implements org.Writer
type Writer struct {
	*org.HTMLWriter
	URLPrefix string
	IsWiki    bool
}

var byteMailto = []byte("mailto:")

// WriteRegularLink renders images, links or videos
func (r *Writer) WriteRegularLink(l org.RegularLink) {
	link := []byte(html.EscapeString(l.URL))
	if l.Protocol == "file" {
		link = link[len("file:"):]
	}
	if len(link) > 0 && !markup.IsLink(link) &&
		link[0] != '#' && !bytes.HasPrefix(link, byteMailto) {
		lnk := string(link)
		if r.IsWiki {
			lnk = util.URLJoin("wiki", lnk)
		}
		link = []byte(util.URLJoin(r.URLPrefix, lnk))
	}

	description := string(link)
	if l.Description != nil {
		description = r.WriteNodesAsString(l.Description...)
	}
	switch l.Kind() {
	case "image":
		imageSrc := getMediaURL(link)
		fmt.Fprintf(r, `<img src="%s" alt="%s" title="%s" />`, imageSrc, description, description)
	case "video":
		videoSrc := getMediaURL(link)
		fmt.Fprintf(r, `<video src="%s" title="%s">%s</video>`, videoSrc, description, description)
	default:
		fmt.Fprintf(r, `<a href="%s" title="%s">%s</a>`, link, description, description)
	}
}

func getMediaURL(l []byte) string {
	srcURL := string(l)

	// Check if link is valid
	if len(srcURL) > 0 && !markup.IsLink(l) {
		srcURL = strings.Replace(srcURL, "/src/", "/media/", 1)
	}

	return srcURL
}
