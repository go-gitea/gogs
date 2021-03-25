// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"bytes"
	"fmt"
	"html"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/util"

	"github.com/niklasfasching/go-org/org"
)

func init() {
	markup.RegisterParser(Parser{})
}

// Parser implements markup.Parser for orgmode
type Parser struct{}

// Name implements markup.Parser
func (Parser) Name() string {
	return "orgmode"
}

// Extensions implements markup.Parser
func (Parser) Extensions() []string {
	return []string{".org"}
}

// Render renders orgmode rawbytes to HTML
func Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	htmlWriter := org.NewHTMLWriter()

	renderer := &Renderer{
		HTMLWriter: htmlWriter,
		URLPrefix:  urlPrefix,
		IsWiki:     isWiki,
	}

	htmlWriter.ExtendingWriter = renderer

	res, err := org.New().Silent().Parse(bytes.NewReader(rawBytes), "").Write(renderer)
	if err != nil {
		log.Error("Panic in orgmode.Render: %v Just returning the rawBytes", err)
		return rawBytes
	}
	return []byte(res)
}

// RenderString reners orgmode string to HTML string
func RenderString(rawContent, urlPrefix string, metas map[string]string, isWiki bool) string {
	return string(Render([]byte(rawContent), urlPrefix, metas, isWiki))
}

// Render reners orgmode string to HTML string
func (Parser) Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	return Render(rawBytes, urlPrefix, metas, isWiki)
}

// Renderer implements org.Writer
type Renderer struct {
	*org.HTMLWriter
	URLPrefix string
	IsWiki    bool
}

var byteMailto = []byte("mailto:")

// WriteRegularLink renders images, links or videos
func (r *Renderer) WriteRegularLink(l org.RegularLink) {
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
