// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markdown

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/common"
	"code.gitea.io/gitea/modules/setting"
	giteautil "code.gitea.io/gitea/modules/util"

	chromahtml "github.com/alecthomas/chroma/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

var (
	converter goldmark.Markdown
	once      = sync.Once{}
)

var (
	urlPrefixKey   = parser.NewContextKey()
	isWikiKey      = parser.NewContextKey()
	renderMetasKey = parser.NewContextKey()
)

type closesWithError interface {
	io.WriteCloser
	CloseWithError(err error) error
}

type limitWriter struct {
	w     closesWithError
	sum   int64
	limit int64
}

// Write implements the standard Write interface:
func (l *limitWriter) Write(data []byte) (int, error) {
	leftToWrite := l.limit - l.sum
	if leftToWrite < int64(len(data)) {
		n, err := l.w.Write(data[:leftToWrite])
		l.sum += int64(n)
		if err != nil {
			return n, err
		}
		_ = l.w.Close()
		return n, fmt.Errorf("Rendered content too large - truncating render")
	}
	n, err := l.w.Write(data)
	l.sum += int64(n)
	return n, err
}

// Close closes the writer
func (l *limitWriter) Close() error {
	return l.w.Close()
}

// CloseWithError closes the writer
func (l *limitWriter) CloseWithError(err error) error {
	return l.w.CloseWithError(err)
}

// NewGiteaParseContext creates a parser.Context with the gitea context set
func NewGiteaParseContext(urlPrefix string, metas map[string]string, isWiki bool) parser.Context {
	pc := parser.NewContext(parser.WithIDs(newPrefixedIDs()))
	pc.Set(urlPrefixKey, urlPrefix)
	pc.Set(isWikiKey, isWiki)
	pc.Set(renderMetasKey, metas)
	return pc
}

// actualRender renders Markdown to HTML without handling special links.
func actualRender(body []byte, urlPrefix string, metas map[string]string, wikiMarkdown bool) []byte {
	once.Do(func() {
		converter = goldmark.New(
			goldmark.WithExtensions(extension.Table,
				extension.Strikethrough,
				extension.TaskList,
				extension.DefinitionList,
				common.FootnoteExtension,
				highlighting.NewHighlighting(
					highlighting.WithFormatOptions(
						chromahtml.WithClasses(true),
						chromahtml.PreventSurroundingPre(true),
					),
					highlighting.WithWrapperRenderer(func(w util.BufWriter, c highlighting.CodeBlockContext, entering bool) {
						if entering {
							language, _ := c.Language()
							if language == nil {
								language = []byte("text")
							}

							languageStr := string(language)

							preClasses := []string{}
							if languageStr == "mermaid" {
								preClasses = append(preClasses, "is-loading")
							}

							if len(preClasses) > 0 {
								_, err := w.WriteString(`<pre class="` + strings.Join(preClasses, " ") + `">`)
								if err != nil {
									return
								}
							} else {
								_, err := w.WriteString(`<pre>`)
								if err != nil {
									return
								}
							}

							// include language-x class as part of commonmark spec
							_, err := w.WriteString(`<code class="chroma language-` + string(language) + `">`)
							if err != nil {
								return
							}
						} else {
							_, err := w.WriteString("</code></pre>")
							if err != nil {
								return
							}
						}
					}),
				),
				meta.Meta,
			),
			goldmark.WithParserOptions(
				parser.WithAttribute(),
				parser.WithAutoHeadingID(),
				parser.WithASTTransformers(
					util.Prioritized(&ASTTransformer{}, 10000),
				),
			),
			goldmark.WithRendererOptions(
				html.WithUnsafe(),
			),
		)

		// Override the original Tasklist renderer!
		converter.Renderer().AddOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(NewHTMLRenderer(), 10),
			),
		)
	})

	rd, wr := io.Pipe()
	defer func() {
		_ = rd.Close()
		_ = wr.Close()
	}()

	lw := &limitWriter{
		w:     wr,
		limit: setting.UI.MaxDisplayFileSize * 3,
	}

	// FIXME: should we include a timeout that closes the pipe to abort the parser and sanitizer if it takes too long?
	go func() {
		defer func() {
			err := recover()
			if err == nil {
				return
			}

			log.Warn("Unable to render markdown due to panic in goldmark: %v", err)
			if log.IsDebug() {
				log.Debug("Panic in markdown: %v\n%s", err, string(log.Stack(2)))
			}
			_ = lw.CloseWithError(fmt.Errorf("%v", err))
		}()

		pc := NewGiteaParseContext(urlPrefix, metas, wikiMarkdown)
		if err := converter.Convert(giteautil.NormalizeEOL(body), lw, parser.WithContext(pc)); err != nil {
			log.Error("Unable to render: %v", err)
			_ = lw.CloseWithError(err)
			return
		}
		_ = lw.Close()
	}()
	return markup.SanitizeReader(rd).Bytes()
}

func render(body []byte, urlPrefix string, metas map[string]string, wikiMarkdown bool) (ret []byte) {
	defer func() {
		err := recover()
		if err == nil {
			return
		}

		log.Warn("Unable to render markdown due to panic in goldmark - will return sanitized raw bytes")
		if log.IsDebug() {
			log.Debug("Panic in markdown: %v\n%s", err, string(log.Stack(2)))
		}
		ret = markup.SanitizeBytes(body)
	}()
	return actualRender(body, urlPrefix, metas, wikiMarkdown)
}

// MarkupName describes markup's name
var MarkupName = "markdown"

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser
type Parser struct{}

// Name implements markup.Parser
func (Parser) Name() string {
	return MarkupName
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return setting.Markdown.FileExtensions
}

// Render implements markup.Parser
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	return render(rawBytes, urlPrefix, metas, isWiki)
}

// Render renders Markdown to HTML with all specific handling stuff.
func Render(rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	return markup.Render("a.md", rawBytes, urlPrefix, metas)
}

// RenderRaw renders Markdown to HTML without handling special links.
func RenderRaw(body []byte, urlPrefix string, wikiMarkdown bool) []byte {
	return render(body, urlPrefix, map[string]string{}, wikiMarkdown)
}

// RenderString renders Markdown to HTML with special links and returns string type.
func RenderString(raw, urlPrefix string, metas map[string]string) string {
	return markup.RenderString("a.md", raw, urlPrefix, metas)
}

// RenderWiki renders markdown wiki page to HTML and return HTML string
func RenderWiki(rawBytes []byte, urlPrefix string, metas map[string]string) string {
	return markup.RenderWiki("a.md", rawBytes, urlPrefix, metas)
}

// IsMarkdownFile reports whether name looks like a Markdown file
// based on its extension.
func IsMarkdownFile(name string) bool {
	return markup.IsMarkupFile(name, MarkupName)
}
