// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/setting"
)

// Init initialize regexps for markdown parsing
func Init() {
	getIssueFullPattern()
	NewSanitizer()
	if len(setting.Markdown.CustomURLSchemes) > 0 {
		CustomLinkURLSchemes(setting.Markdown.CustomURLSchemes)
	}

	// since setting maybe changed extensions, this will reload all renderer extensions mapping
	extRenderers = make(map[string]Renderer)
	for _, renderer := range renderers {
		for _, ext := range renderer.Extensions() {
			extRenderers[strings.ToLower(ext)] = renderer
		}
	}
}

// RenderContext represents a render context
type RenderContext struct {
	Ctx           context.Context
	Filename      string
	Type          string
	IsWiki        bool
	URLPrefix     string
	Metas         map[string]string
	DefaultLink   string
	GitRepo       *git.Repository
	ShaExistCache map[string]bool
	cancelFn      func()
}

// Cancel runs any cleanup functions that have been registered for this Ctx
func (ctx *RenderContext) Cancel() {
	if ctx == nil {
		return
	}
	ctx.ShaExistCache = map[string]bool{}
	if ctx.cancelFn == nil {
		return
	}
	ctx.cancelFn()
}

// AddCancel adds the provided fn as a Cleanup for this Ctx
func (ctx *RenderContext) AddCancel(fn func()) {
	if ctx == nil {
		return
	}
	oldCancelFn := ctx.cancelFn
	if oldCancelFn == nil {
		ctx.cancelFn = fn
		return
	}
	ctx.cancelFn = func() {
		defer oldCancelFn()
		fn()
	}
}

// Renderer defines an interface for rendering markup file to HTML
type Renderer interface {
	Name() string // markup format name
	Extensions() []string
	NeedPostProcess() bool
	SanitizerRules() []setting.MarkupSanitizerRule
	Render(ctx *RenderContext, input io.Reader, output io.Writer) error
}

var (
	extRenderers = make(map[string]Renderer)
	renderers    = make(map[string]Renderer)
)

// RegisterRenderer registers a new markup file renderer
func RegisterRenderer(renderer Renderer) {
	renderers[renderer.Name()] = renderer
	for _, ext := range renderer.Extensions() {
		extRenderers[strings.ToLower(ext)] = renderer
	}
}

// GetRendererByFileName get renderer by filename
func GetRendererByFileName(filename string) Renderer {
	extension := strings.ToLower(filepath.Ext(filename))
	return extRenderers[extension]
}

// GetRendererByType returns a renderer according type
func GetRendererByType(tp string) Renderer {
	return renderers[tp]
}

// Render renders markup file to HTML with all specific handling stuff.
func Render(ctx *RenderContext, input io.Reader, output io.Writer) error {
	if ctx.Type != "" {
		return renderByType(ctx, input, output)
	} else if ctx.Filename != "" {
		return renderFile(ctx, input, output)
	}
	return errors.New("Render options both filename and type missing")
}

// RenderString renders Markup string to HTML with all specific handling stuff and return string
func RenderString(ctx *RenderContext, content string) (string, error) {
	var buf strings.Builder
	if err := Render(ctx, strings.NewReader(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func render(ctx *RenderContext, renderer Renderer, input io.Reader, output io.Writer) error {
	var wg sync.WaitGroup
	var err error
	pr, pw := io.Pipe()
	defer func() {
		_ = pr.Close()
		_ = pw.Close()
	}()

	pr2, pw2 := io.Pipe()
	defer func() {
		_ = pr2.Close()
		_ = pw2.Close()
	}()

	wg.Add(1)
	go func() {
		buf := SanitizeReader(pr2, renderer.Name())
		_, err = io.Copy(output, buf)
		_ = pr2.Close()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		if renderer.NeedPostProcess() {
			err = PostProcess(ctx, pr, pw2)
		} else {
			_, err = io.Copy(pw2, pr)
		}
		_ = pr.Close()
		_ = pw2.Close()
		wg.Done()
	}()

	if err1 := renderer.Render(ctx, input, pw); err1 != nil {
		return err1
	}
	_ = pw.Close()

	wg.Wait()
	return err
}

// ErrUnsupportedRenderType represents
type ErrUnsupportedRenderType struct {
	Type string
}

func (err ErrUnsupportedRenderType) Error() string {
	return fmt.Sprintf("Unsupported render type: %s", err.Type)
}

func renderByType(ctx *RenderContext, input io.Reader, output io.Writer) error {
	if renderer, ok := renderers[ctx.Type]; ok {
		return render(ctx, renderer, input, output)
	}
	return ErrUnsupportedRenderType{ctx.Type}
}

// ErrUnsupportedRenderExtension represents the error when extension doesn't supported to render
type ErrUnsupportedRenderExtension struct {
	Extension string
}

func (err ErrUnsupportedRenderExtension) Error() string {
	return fmt.Sprintf("Unsupported render extension: %s", err.Extension)
}

func renderFile(ctx *RenderContext, input io.Reader, output io.Writer) error {
	extension := strings.ToLower(filepath.Ext(ctx.Filename))
	if renderer, ok := extRenderers[extension]; ok {
		return render(ctx, renderer, input, output)
	}
	return ErrUnsupportedRenderExtension{extension}
}

// Type returns if markup format via the filename
func Type(filename string) string {
	if parser := GetRendererByFileName(filename); parser != nil {
		return parser.Name()
	}
	return ""
}

// IsMarkupFile reports whether file is a markup type file
func IsMarkupFile(name, markup string) bool {
	if parser := GetRendererByFileName(name); parser != nil {
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
