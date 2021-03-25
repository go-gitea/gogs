// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

// Init initialize regexps for markdown parsing
func Init() {
	getIssueFullPattern()
	NewSanitizer()
	if len(setting.Markdown.CustomURLSchemes) > 0 {
		CustomLinkURLSchemes(setting.Markdown.CustomURLSchemes)
	}

	// since setting maybe changed extensions, this will reload all parser extensions mapping
	extParsers = make(map[string]Parser)
	for _, parser := range parsers {
		for _, ext := range parser.Extensions() {
			extParsers[strings.ToLower(ext)] = parser
		}
	}
}

// Parser defines an interface for parsering markup file to HTML
type Parser interface {
	Name() string // markup format name
	Extensions() []string
	Render(rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte
}

var (
	extParsers = make(map[string]Parser)
	parsers    = make(map[string]Parser)
)

// RegisterParser registers a new markup file parser
func RegisterParser(parser Parser) {
	parsers[parser.Name()] = parser
	for _, ext := range parser.Extensions() {
		extParsers[strings.ToLower(ext)] = parser
	}
}

// GetParserByFileName get parser by filename
func GetParserByFileName(filename string) Parser {
	extension := strings.ToLower(filepath.Ext(filename))
	return extParsers[extension]
}

// GetParserByType returns a parser according type
func GetParserByType(tp string) Parser {
	return parsers[tp]
}

// Render renders markup file to HTML with all specific handling stuff.
func Render(filename string, rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	return renderFile(filename, rawBytes, urlPrefix, metas, false)
}

// RenderByType renders markup to HTML with special links and returns string type.
func RenderByType(tp string, rawBytes []byte, urlPrefix string, metas map[string]string) []byte {
	return renderByType(tp, rawBytes, urlPrefix, metas, false)
}

// RenderString renders Markdown to HTML with special links and returns string type.
func RenderString(filename, raw, urlPrefix string, metas map[string]string) string {
	return string(renderFile(filename, []byte(raw), urlPrefix, metas, false))
}

// RenderWiki renders markdown wiki page to HTML and return HTML string
func RenderWiki(filename string, rawBytes []byte, urlPrefix string, metas map[string]string) string {
	return string(renderFile(filename, rawBytes, urlPrefix, metas, true))
}

func render(parser Parser, rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	result := parser.Render(rawBytes, urlPrefix, metas, isWiki)
	// TODO: one day the error should be returned.
	result, err := PostProcess(result, urlPrefix, metas, isWiki)
	if err != nil {
		log.Error("PostProcess: %v", err)
	}
	return SanitizeBytes(result)
}

func renderByType(tp string, rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	if parser, ok := parsers[tp]; ok {
		return render(parser, rawBytes, urlPrefix, metas, isWiki)
	}
	return nil
}

func renderFile(filename string, rawBytes []byte, urlPrefix string, metas map[string]string, isWiki bool) []byte {
	extension := strings.ToLower(filepath.Ext(filename))
	if parser, ok := extParsers[extension]; ok {
		return render(parser, rawBytes, urlPrefix, metas, isWiki)
	}
	return nil
}

// Type returns if markup format via the filename
func Type(filename string) string {
	if parser := GetParserByFileName(filename); parser != nil {
		return parser.Name()
	}
	return ""
}

// IsMarkupFile reports whether file is a markup type file
func IsMarkupFile(name, markup string) bool {
	if parser := GetParserByFileName(name); parser != nil {
		return parser.Name() == markup
	}
	return false
}

// IsReadmeFile reports whether name looks like a README file
// based on its name. If an extension is provided, it will strictly
// match that extension.
// Note that the '.' should be provided in ext, e.g ".md"
func IsReadmeFile(name string, ext ...string) bool {
	name = strings.ToLower(name)
	if len(ext) > 0 {
		return name == "readme"+ext[0]
	}
	if len(name) < 6 {
		return false
	} else if len(name) == 6 {
		return name == "readme"
	}
	return name[:7] == "readme."
}
