// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package external

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// RegisterRenderers registers all supported third part renderers according settings
func RegisterRenderers() {
	for _, renderer := range setting.ExternalMarkupRenderers {
		if renderer.Enabled && renderer.Command != "" && len(renderer.FileExtensions) > 0 {
			markup.RegisterRenderer(&Renderer{renderer})
		}
	}
}

// Renderer implements markup.Renderer for external tools
type Renderer struct {
	*setting.MarkupRenderer
}

// Name returns the external tool name
func (p *Renderer) Name() string {
	return p.MarkupName
}

// NeedPostProcess implements markup.Renderer
func (p *Renderer) NeedPostProcess() bool {
	return p.MarkupRenderer.NeedPostProcess
}

// Extensions returns the supported extensions of the tool
func (p *Renderer) Extensions() []string {
	return p.FileExtensions
}

// SanitizerRules implements markup.Renderer
func (p *Renderer) SanitizerRules() []setting.MarkupSanitizerRule {
	return p.MarkupSanitizerRules
}

func envMark(envName string) string {
	if runtime.GOOS == "windows" {
		return "%" + envName + "%"
	}
	return "$" + envName
}

// Render renders the data of the document to HTML via the external tool.
func (p *Renderer) Render(ctx *markup.RenderContext, input io.Reader, output io.Writer) error {
	var (
		urlRawPrefix = strings.Replace(ctx.URLPrefix, "/src/", "/raw/", 1)
		command      = strings.NewReplacer(envMark("GITEA_PREFIX_SRC"), ctx.URLPrefix,
			envMark("GITEA_PREFIX_RAW"), urlRawPrefix).Replace(p.Command)
		commands = strings.Fields(command)
		args     = commands[1:]
	)

	if p.IsInputFile {
		// write to temp file
		f, err := ioutil.TempFile("", "gitea_input")
		if err != nil {
			return fmt.Errorf("%s create temp file when rendering %s failed: %v", p.Name(), p.Command, err)
		}
		tmpPath := f.Name()
		defer func() {
			if err := util.Remove(tmpPath); err != nil {
				log.Warn("Unable to remove temporary file: %s: Error: %v", tmpPath, err)
			}
		}()

		_, err = io.Copy(f, input)
		if err != nil {
			f.Close()
			return fmt.Errorf("%s write data to temp file when rendering %s failed: %v", p.Name(), p.Command, err)
		}

		err = f.Close()
		if err != nil {
			return fmt.Errorf("%s close temp file when rendering %s failed: %v", p.Name(), p.Command, err)
		}
		args = append(args, f.Name())
	}

	if ctx == nil || ctx.Ctx == nil {
		return fmt.Errorf("RenderContext did not provide context")
	}

	processCtx, cancel := context.WithCancel(ctx.Ctx)
	defer cancel()

	pid := process.GetManager().Add(fmt.Sprintf("Render [%s] for %s", commands[0], ctx.URLPrefix), cancel)
	defer process.GetManager().Remove(pid)

	cmd := exec.CommandContext(processCtx, commands[0], args...)
	cmd.Env = append(
		os.Environ(),
		"GITEA_PREFIX_SRC="+ctx.URLPrefix,
		"GITEA_PREFIX_RAW="+urlRawPrefix,
	)
	if !p.IsInputFile {
		cmd.Stdin = input
	}
	cmd.Stdout = output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s render run command %s %v failed: %v", p.Name(), commands[0], args, err)
	}
	return nil
}
