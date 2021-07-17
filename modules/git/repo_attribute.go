// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// CheckAttributeOpts represents the possible options to CheckAttribute
type CheckAttributeOpts struct {
	CachedOnly    bool
	AllAttributes bool
	Attributes    []string
	Filenames     []string
	IndexFile     string
}

// CheckAttribute return the Blame object of file
func (repo *Repository) CheckAttribute(opts CheckAttributeOpts) (map[string]map[string]string, error) {
	err := LoadGitVersion()
	if err != nil {
		return nil, fmt.Errorf("Git version missing: %v", err)
	}

	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)

	cmdArgs := []string{"check-attr", "-z"}

	if opts.AllAttributes {
		cmdArgs = append(cmdArgs, "-a")
	} else {
		for _, attribute := range opts.Attributes {
			if attribute != "" {
				cmdArgs = append(cmdArgs, attribute)
			}
		}
	}

	// git check-attr --cached first appears in git 1.7.8
	if opts.CachedOnly && CheckGitVersionAtLeast("1.7.8") == nil {
		cmdArgs = append(cmdArgs, "--cached")
	}

	cmdArgs = append(cmdArgs, "--")

	for _, arg := range opts.Filenames {
		if arg != "" {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	cmd := NewCommand(cmdArgs...)

	env := make([]string, 0, 1)
	if len(opts.IndexFile) > 0 {
		env = append(env, "GIT_INDEX_FILE="+opts.IndexFile)
	}

	if err := cmd.RunInDirTimeoutEnvFullPipeline(env, -1, repo.Path, stdOut, stdErr, nil); err != nil {
		return nil, fmt.Errorf("Failed to run check-attr: %v\n%s\n%s", err, stdOut.String(), stdErr.String())
	}

	fields := bytes.Split(stdOut.Bytes(), []byte{'\000'})

	if len(fields)%3 != 1 {
		return nil, fmt.Errorf("Wrong number of fields in return from check-attr")
	}

	var name2attribute2info = make(map[string]map[string]string)

	for i := 0; i < (len(fields) / 3); i++ {
		filename := string(fields[3*i])
		attribute := string(fields[3*i+1])
		info := string(fields[3*i+2])
		attribute2info := name2attribute2info[filename]
		if attribute2info == nil {
			attribute2info = make(map[string]string)
		}
		attribute2info[attribute] = info
		name2attribute2info[filename] = attribute2info
	}

	return name2attribute2info, nil
}

// AttrChecker attrs checker
type AttrChecker struct {
	// params
	RequestAttrs []string
	Repo         *Repository
	IndexFile    string

	stdinReader *io.PipeReader
	stdinWriter *io.PipeWriter
	stdOut      *lineWriter
	cmd         *Command
	env         []string
}

// Init init cmd
func (c *AttrChecker) Init() {
	if len(c.RequestAttrs) == 0 {
		panic("Should have RequestAttrs!")
	}

	cmdArgs := []string{"check-attr"}
	cmdArgs = append(cmdArgs, c.RequestAttrs...)
	if len(c.IndexFile) > 0 {
		cmdArgs = append(cmdArgs, "--cached")
		c.env = []string{"GIT_INDEX_FILE=" + c.IndexFile}
	}
	cmdArgs = append(cmdArgs, "--stdin")
	c.cmd = NewCommand(cmdArgs...)
	c.stdinReader, c.stdinWriter = io.Pipe()
	c.stdOut = new(lineWriter)
}

// Run run cmd
func (c *AttrChecker) Run() error {
	stdErr := new(bytes.Buffer)
	err := c.cmd.RunInDirTimeoutEnvFullPipeline(c.env, -1, c.Repo.Path, c.stdOut, stdErr, c.stdinReader)
	if err != nil {
		return fmt.Errorf("failed to run attr-check. Error: %w\nStderr: %s", err, stdErr.String())
	}

	return nil
}

// CheckAttrs check attr for given path
func (c *AttrChecker) CheckAttrs(path string) (map[string]string, error) {
	_, err := c.stdinWriter.Write([]byte(path + "\n"))
	if err != nil {
		return nil, err
	}

	rs := make(map[string]string)
	for range c.RequestAttrs {
		line, err := c.stdOut.ReadLine(DefaultCommandExecutionTimeout)
		if err != nil {
			return nil, err
		}
		splits := strings.SplitN(line, ": ", 3)
		if len(splits) != 3 {
			continue
		}
		rs[splits[1]] = splits[2]
	}
	return rs, nil
}

// Close close pip after use
func (c *AttrChecker) Close() {
	c.stdinWriter.Close()
}

type lineWriter struct {
	tmp   []byte
	lines chan string
}

func (wr *lineWriter) Write(p []byte) (n int, err error) {
	l := len(p)
	if wr.tmp != nil && len(wr.tmp) > 0 {
		p = append(wr.tmp, p...)
	}
	lastEndl := -1
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '\n' {
			lastEndl = i
			break
		}
	}
	if lastEndl != len(p)-1 {
		wr.tmp = p[lastEndl+1:]
	}

	if lastEndl == -1 {
		return l, nil
	}

	if wr.lines == nil {
		wr.lines = make(chan string, 5)
	}

	splits := bytes.Split(p[:lastEndl], []byte{'\n'})
	for _, line := range splits {
		wr.lines <- string(line)
	}

	return l, nil
}

func (wr *lineWriter) ReadLine(timeOut time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeOut)
	defer cancel()

	select {
	case rs := <-wr.lines:
		return rs, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
