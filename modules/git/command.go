// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
)

var (
	// GlobalCommandArgs global command args for external package setting
	GlobalCommandArgs []string

	// defaultCommandExecutionTimeout default command execution timeout duration
	defaultCommandExecutionTimeout = 360 * time.Second
)

// DefaultLocale is the default LC_ALL to run git commands in.
const DefaultLocale = "C"

// Command represents a command with its subcommands or arguments.
type Command struct {
	name          string
	args          []string
	parentContext context.Context
	desc          string
}

func (c *Command) String() string {
	if len(c.args) == 0 {
		return c.name
	}
	return fmt.Sprintf("%s %s", c.name, strings.Join(c.args, " "))
}

// NewCommand creates and returns a new Git Command based on given command and arguments.
func NewCommand(args ...string) *Command {
	return NewCommandContext(DefaultContext, args...)
}

// NewCommandContext creates and returns a new Git Command based on given command and arguments.
func NewCommandContext(ctx context.Context, args ...string) *Command {
	// Make an explicit copy of GlobalCommandArgs, otherwise append might overwrite it
	cargs := make([]string, len(GlobalCommandArgs))
	copy(cargs, GlobalCommandArgs)
	return &Command{
		name:          GitExecutable,
		args:          append(cargs, args...),
		parentContext: ctx,
	}
}

// NewCommandNoGlobals creates and returns a new Git Command based on given command and arguments only with the specify args and don't care global command args
func NewCommandNoGlobals(args ...string) *Command {
	return NewCommandContextNoGlobals(DefaultContext, args...)
}

// NewCommandContextNoGlobals creates and returns a new Git Command based on given command and arguments only with the specify args and don't care global command args
func NewCommandContextNoGlobals(ctx context.Context, args ...string) *Command {
	return &Command{
		name:          GitExecutable,
		args:          args,
		parentContext: ctx,
	}
}

// SetParentContext sets the parent context for this command
func (c *Command) SetParentContext(ctx context.Context) *Command {
	c.parentContext = ctx
	return c
}

// SetDescription sets the description for this command which be returned on
// c.String()
func (c *Command) SetDescription(desc string) *Command {
	c.desc = desc
	return c
}

// AddArguments adds new argument(s) to the command.
func (c *Command) AddArguments(args ...string) *Command {
	c.args = append(c.args, args...)
	return c
}

// RunInDirTimeoutEnvPipeline executes the command in given directory with given timeout,
// it pipes stdout and stderr to given io.Writer.
func (c *Command) RunInDirTimeoutEnvPipeline(env []string, timeout time.Duration, dir string, stdout, stderr io.Writer) error {
	return c.RunInDirTimeoutEnvFullPipeline(env, timeout, dir, stdout, stderr, nil)
}

// RunInDirTimeoutEnvFullPipeline executes the command in given directory with given timeout,
// it pipes stdout and stderr to given io.Writer and passes in an io.Reader as stdin.
func (c *Command) RunInDirTimeoutEnvFullPipeline(env []string, timeout time.Duration, dir string, stdout, stderr io.Writer, stdin io.Reader) error {
	return c.RunInDirTimeoutEnvFullPipelineFunc(env, timeout, dir, stdout, stderr, stdin, nil)
}

// RunInDirTimeoutEnvFullPipelineFunc executes the command in given directory with given timeout,
// it pipes stdout and stderr to given io.Writer and passes in an io.Reader as stdin. Between cmd.Start and cmd.Wait the passed in function is run.
func (c *Command) RunInDirTimeoutEnvFullPipelineFunc(env []string, timeout time.Duration, dir string, stdout, stderr io.Writer, stdin io.Reader, fn func(context.Context, context.CancelFunc) error) error {
	if timeout == -1 {
		timeout = defaultCommandExecutionTimeout
	}

	if len(dir) == 0 {
		log.Debug("%s", c)
	} else {
		log.Debug("%s: %v", dir, c)
	}

	ctx, cancel := context.WithTimeout(c.parentContext, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.name, c.args...)
	if env == nil {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = env
	}

	cmd.Env = append(
		cmd.Env,
		fmt.Sprintf("LC_ALL=%s", DefaultLocale),
		// avoid prompting for credentials interactively, supported since git v2.3
		"GIT_TERMINAL_PROMPT=0",
	)

	// TODO: verify if this is still needed in golang 1.15
	if goVersionLessThan115 {
		cmd.Env = append(cmd.Env, "GODEBUG=asyncpreemptoff=1")
	}
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = stdin
	if err := cmd.Start(); err != nil {
		return err
	}

	desc := c.desc
	if desc == "" {
		desc = fmt.Sprintf("%s %s %s [repo_path: %s]", GitExecutable, c.name, strings.Join(c.args, " "), dir)
	}
	pid := process.GetManager().Add(desc, cancel)
	defer process.GetManager().Remove(pid)

	if fn != nil {
		err := fn(ctx, cancel)
		if err != nil {
			cancel()
			_ = cmd.Wait()
			return err
		}
	}

	if err := cmd.Wait(); err != nil && ctx.Err() != context.DeadlineExceeded {
		return err
	}

	return ctx.Err()
}

// RunInDirTimeoutPipeline executes the command in given directory with given timeout,
// it pipes stdout and stderr to given io.Writer.
func (c *Command) RunInDirTimeoutPipeline(timeout time.Duration, dir string, stdout, stderr io.Writer) error {
	return c.RunInDirTimeoutEnvPipeline(nil, timeout, dir, stdout, stderr)
}

// RunInDirTimeoutFullPipeline executes the command in given directory with given timeout,
// it pipes stdout and stderr to given io.Writer, and stdin from the given io.Reader
func (c *Command) RunInDirTimeoutFullPipeline(timeout time.Duration, dir string, stdout, stderr io.Writer, stdin io.Reader) error {
	return c.RunInDirTimeoutEnvFullPipeline(nil, timeout, dir, stdout, stderr, stdin)
}

// RunInDirTimeout executes the command in given directory with given timeout,
// and returns stdout in []byte and error (combined with stderr).
func (c *Command) RunInDirTimeout(timeout time.Duration, dir string) ([]byte, error) {
	return c.RunInDirTimeoutEnv(nil, timeout, dir)
}

// RunInDirTimeoutEnv executes the command in given directory with given timeout,
// and returns stdout in []byte and error (combined with stderr).
func (c *Command) RunInDirTimeoutEnv(env []string, timeout time.Duration, dir string) ([]byte, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	if err := c.RunInDirTimeoutEnvPipeline(env, timeout, dir, stdout, stderr); err != nil {
		return nil, ConcatenateError(err, stderr.String())
	}
	if stdout.Len() > 0 && log.IsTrace() {
		tracelen := stdout.Len()
		if tracelen > 1024 {
			tracelen = 1024
		}
		log.Trace("Stdout:\n %s", stdout.Bytes()[:tracelen])
	}
	return stdout.Bytes(), nil
}

// RunInDirPipeline executes the command in given directory,
// it pipes stdout and stderr to given io.Writer.
func (c *Command) RunInDirPipeline(dir string, stdout, stderr io.Writer) error {
	return c.RunInDirFullPipeline(dir, stdout, stderr, nil)
}

// RunInDirFullPipeline executes the command in given directory,
// it pipes stdout and stderr to given io.Writer.
func (c *Command) RunInDirFullPipeline(dir string, stdout, stderr io.Writer, stdin io.Reader) error {
	return c.RunInDirTimeoutFullPipeline(-1, dir, stdout, stderr, stdin)
}

// RunInDirBytes executes the command in given directory
// and returns stdout in []byte and error (combined with stderr).
func (c *Command) RunInDirBytes(dir string) ([]byte, error) {
	return c.RunInDirTimeout(-1, dir)
}

// RunInDir executes the command in given directory
// and returns stdout in string and error (combined with stderr).
func (c *Command) RunInDir(dir string) (string, error) {
	return c.RunInDirWithEnv(dir, nil)
}

// RunInDirWithEnv executes the command in given directory
// and returns stdout in string and error (combined with stderr).
func (c *Command) RunInDirWithEnv(dir string, env []string) (string, error) {
	stdout, err := c.RunInDirTimeoutEnv(env, -1, dir)
	if err != nil {
		return "", err
	}
	return string(stdout), nil
}

// RunTimeout executes the command in default working directory with given timeout,
// and returns stdout in string and error (combined with stderr).
func (c *Command) RunTimeout(timeout time.Duration) (string, error) {
	stdout, err := c.RunInDirTimeout(timeout, "")
	if err != nil {
		return "", err
	}
	return string(stdout), nil
}

// Run executes the command in default working directory
// and returns stdout in string and error (combined with stderr).
func (c *Command) Run() (string, error) {
	return c.RunTimeout(-1)
}
