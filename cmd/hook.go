// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/urfave/cli"
)

const (
	hookBatchSize = 30
)

var (
	// CmdHook represents the available hooks sub-command.
	CmdHook = cli.Command{
		Name:        "hook",
		Usage:       "Delegate commands to corresponding Git hooks",
		Description: "This should only be called by Git",
		Subcommands: []cli.Command{
			subcmdHookPreReceive,
			subcmdHookUpdate,
			subcmdHookPostReceive,
		},
	}

	subcmdHookPreReceive = cli.Command{
		Name:        "pre-receive",
		Usage:       "Delegate pre-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookPreReceive,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name: "debug",
			},
		},
	}
	subcmdHookUpdate = cli.Command{
		Name:        "update",
		Usage:       "Delegate update Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookUpdate,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name: "debug",
			},
		},
	}
	subcmdHookPostReceive = cli.Command{
		Name:        "post-receive",
		Usage:       "Delegate post-receive Git hook",
		Description: "This command should only be called by Git",
		Action:      runHookPostReceive,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name: "debug",
			},
		},
	}
)

type delayWriter struct {
	internal io.Writer
	buf      *bytes.Buffer
	timer    *time.Timer
}

func newDelayWriter(internal io.Writer, delay time.Duration) *delayWriter {
	timer := time.NewTimer(delay)
	return &delayWriter{
		internal: internal,
		buf:      &bytes.Buffer{},
		timer:    timer,
	}
}

func (d *delayWriter) Write(p []byte) (n int, err error) {
	if d.buf != nil {
		select {
		case <-d.timer.C:
			_, err := d.internal.Write(d.buf.Bytes())
			if err != nil {
				return 0, err
			}
			d.buf = nil
			return d.internal.Write(p)
		default:
			return d.buf.Write(p)
		}
	}
	return d.internal.Write(p)
}

func (d *delayWriter) WriteString(s string) (n int, err error) {
	if d.buf != nil {
		select {
		case <-d.timer.C:
			_, err := d.internal.Write(d.buf.Bytes())
			if err != nil {
				return 0, err
			}
			d.buf = nil
			return d.internal.Write([]byte(s))
		default:
			return d.buf.WriteString(s)
		}
	}
	return d.internal.Write([]byte(s))
}

func (d *delayWriter) Close() error {
	if d == nil {
		return nil
	}
	stopped := util.StopTimer(d.timer)
	if stopped || d.buf == nil {
		return nil
	}
	_, err := d.internal.Write(d.buf.Bytes())
	d.buf = nil
	return err
}

type nilWriter struct{}

func (n *nilWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (n *nilWriter) WriteString(s string) (int, error) {
	return len(s), nil
}

func runHookPreReceive(c *cli.Context) error {
	if os.Getenv(models.EnvIsInternal) == "true" {
		return nil
	}
	ctx, cancel := installSignals()
	defer cancel()

	setup("hooks/pre-receive.log", c.Bool("debug"))

	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		if setting.OnlyAllowPushIfGiteaEnvironmentSet {
			return fail(`Rejecting changes as Gitea environment not set.
If you are pushing over SSH you must push with a key managed by
Gitea or set your environment appropriately.`, "")
		}
		return nil
	}

	// the environment is set by serv command
	isWiki := os.Getenv(models.EnvRepoIsWiki) == "true"
	username := os.Getenv(models.EnvRepoUsername)
	reponame := os.Getenv(models.EnvRepoName)
	userID, _ := strconv.ParseInt(os.Getenv(models.EnvPusherID), 10, 64)
	prID, _ := strconv.ParseInt(os.Getenv(models.EnvPRID), 10, 64)
	isDeployKey, _ := strconv.ParseBool(os.Getenv(models.EnvIsDeployKey))

	hookOptions := private.HookOptions{
		UserID:                          userID,
		GitAlternativeObjectDirectories: os.Getenv(private.GitAlternativeObjectDirectories),
		GitObjectDirectory:              os.Getenv(private.GitObjectDirectory),
		GitQuarantinePath:               os.Getenv(private.GitQuarantinePath),
		GitPushOptions:                  pushOptions(),
		PullRequestID:                   prID,
		IsDeployKey:                     isDeployKey,
	}

	scanner := bufio.NewScanner(os.Stdin)

	oldCommitIDs := make([]string, hookBatchSize)
	newCommitIDs := make([]string, hookBatchSize)
	refFullNames := make([]string, hookBatchSize)
	count := 0
	total := 0
	lastline := 0

	var out io.Writer
	out = &nilWriter{}
	if setting.Git.VerbosePush {
		if setting.Git.VerbosePushDelay > 0 {
			dWriter := newDelayWriter(os.Stdout, setting.Git.VerbosePushDelay)
			defer dWriter.Close()
			out = dWriter
		} else {
			out = os.Stdout
		}
	}

	for scanner.Scan() {
		// TODO: support news feeds for wiki
		if isWiki {
			continue
		}

		fields := bytes.Fields(scanner.Bytes())
		if len(fields) != 3 {
			continue
		}

		oldCommitID := string(fields[0])
		newCommitID := string(fields[1])
		refFullName := string(fields[2])
		total++
		lastline++

		// If the ref is a branch or tag, check if it's protected
		if strings.HasPrefix(refFullName, git.BranchPrefix) || strings.HasPrefix(refFullName, git.TagPrefix) {
			oldCommitIDs[count] = oldCommitID
			newCommitIDs[count] = newCommitID
			refFullNames[count] = refFullName
			count++
			fmt.Fprintf(out, "*")

			if count >= hookBatchSize {
				fmt.Fprintf(out, " Checking %d references\n", count)

				hookOptions.OldCommitIDs = oldCommitIDs
				hookOptions.NewCommitIDs = newCommitIDs
				hookOptions.RefFullNames = refFullNames
				statusCode, msg := private.HookPreReceive(ctx, username, reponame, hookOptions)
				switch statusCode {
				case http.StatusOK:
					// no-op
				case http.StatusInternalServerError:
					return fail("Internal Server Error", msg)
				default:
					return fail(msg, "")
				}
				count = 0
				lastline = 0
			}
		} else {
			fmt.Fprintf(out, ".")
		}
		if lastline >= hookBatchSize {
			fmt.Fprintf(out, "\n")
			lastline = 0
		}
	}

	if count > 0 {
		hookOptions.OldCommitIDs = oldCommitIDs[:count]
		hookOptions.NewCommitIDs = newCommitIDs[:count]
		hookOptions.RefFullNames = refFullNames[:count]

		fmt.Fprintf(out, " Checking %d references\n", count)

		statusCode, msg := private.HookPreReceive(ctx, username, reponame, hookOptions)
		switch statusCode {
		case http.StatusInternalServerError:
			return fail("Internal Server Error", msg)
		case http.StatusForbidden:
			return fail(msg, "")
		}
	} else if lastline > 0 {
		fmt.Fprintf(out, "\n")
		lastline = 0
	}

	fmt.Fprintf(out, "Checked %d references in total\n", total)
	return nil
}

func runHookUpdate(c *cli.Context) error {
	// Update is empty and is kept only for backwards compatibility
	return nil
}

func runHookPostReceive(c *cli.Context) error {
	ctx, cancel := installSignals()
	defer cancel()

	// First of all run update-server-info no matter what
	if _, err := git.NewCommand("update-server-info").SetParentContext(ctx).Run(); err != nil {
		return fmt.Errorf("Failed to call 'git update-server-info': %v", err)
	}

	// Now if we're an internal don't do anything else
	if os.Getenv(models.EnvIsInternal) == "true" {
		return nil
	}

	setup("hooks/post-receive.log", c.Bool("debug"))

	if len(os.Getenv("SSH_ORIGINAL_COMMAND")) == 0 {
		if setting.OnlyAllowPushIfGiteaEnvironmentSet {
			return fail(`Rejecting changes as Gitea environment not set.
If you are pushing over SSH you must push with a key managed by
Gitea or set your environment appropriately.`, "")
		}
		return nil
	}

	var out io.Writer
	var dWriter *delayWriter
	out = &nilWriter{}
	if setting.Git.VerbosePush {
		if setting.Git.VerbosePushDelay > 0 {
			dWriter = newDelayWriter(os.Stdout, setting.Git.VerbosePushDelay)
			defer dWriter.Close()
			out = dWriter
		} else {
			out = os.Stdout
		}
	}

	// the environment is set by serv command
	repoUser := os.Getenv(models.EnvRepoUsername)
	isWiki := os.Getenv(models.EnvRepoIsWiki) == "true"
	repoName := os.Getenv(models.EnvRepoName)
	pusherID, _ := strconv.ParseInt(os.Getenv(models.EnvPusherID), 10, 64)
	pusherName := os.Getenv(models.EnvPusherName)

	hookOptions := private.HookOptions{
		UserName:                        pusherName,
		UserID:                          pusherID,
		GitAlternativeObjectDirectories: os.Getenv(private.GitAlternativeObjectDirectories),
		GitObjectDirectory:              os.Getenv(private.GitObjectDirectory),
		GitQuarantinePath:               os.Getenv(private.GitQuarantinePath),
		GitPushOptions:                  pushOptions(),
	}
	oldCommitIDs := make([]string, hookBatchSize)
	newCommitIDs := make([]string, hookBatchSize)
	refFullNames := make([]string, hookBatchSize)
	count := 0
	total := 0
	wasEmpty := false
	masterPushed := false
	results := make([]private.HookPostReceiveBranchResult, 0)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		// TODO: support news feeds for wiki
		if isWiki {
			continue
		}

		fields := bytes.Fields(scanner.Bytes())
		if len(fields) != 3 {
			continue
		}

		fmt.Fprintf(out, ".")
		oldCommitIDs[count] = string(fields[0])
		newCommitIDs[count] = string(fields[1])
		refFullNames[count] = string(fields[2])
		if refFullNames[count] == git.BranchPrefix+"master" && newCommitIDs[count] != git.EmptySHA && count == total {
			masterPushed = true
		}
		count++
		total++

		if count >= hookBatchSize {
			fmt.Fprintf(out, " Processing %d references\n", count)
			hookOptions.OldCommitIDs = oldCommitIDs
			hookOptions.NewCommitIDs = newCommitIDs
			hookOptions.RefFullNames = refFullNames
			resp, err := private.HookPostReceive(ctx, repoUser, repoName, hookOptions)
			if resp == nil {
				_ = dWriter.Close()
				hookPrintResults(results)
				return fail("Internal Server Error", err)
			}
			wasEmpty = wasEmpty || resp.RepoWasEmpty
			results = append(results, resp.Results...)
			count = 0
		}
	}

	if count == 0 {
		if wasEmpty && masterPushed {
			// We need to tell the repo to reset the default branch to master
			err := private.SetDefaultBranch(ctx, repoUser, repoName, "master")
			if err != nil {
				return fail("Internal Server Error", "SetDefaultBranch failed with Error: %v", err)
			}
		}
		fmt.Fprintf(out, "Processed %d references in total\n", total)

		_ = dWriter.Close()
		hookPrintResults(results)
		return nil
	}

	hookOptions.OldCommitIDs = oldCommitIDs[:count]
	hookOptions.NewCommitIDs = newCommitIDs[:count]
	hookOptions.RefFullNames = refFullNames[:count]

	fmt.Fprintf(out, " Processing %d references\n", count)

	resp, err := private.HookPostReceive(ctx, repoUser, repoName, hookOptions)
	if resp == nil {
		_ = dWriter.Close()
		hookPrintResults(results)
		return fail("Internal Server Error", err)
	}
	wasEmpty = wasEmpty || resp.RepoWasEmpty
	results = append(results, resp.Results...)

	fmt.Fprintf(out, "Processed %d references in total\n", total)

	if wasEmpty && masterPushed {
		// We need to tell the repo to reset the default branch to master
		err := private.SetDefaultBranch(ctx, repoUser, repoName, "master")
		if err != nil {
			return fail("Internal Server Error", "SetDefaultBranch failed with Error: %v", err)
		}
	}
	_ = dWriter.Close()
	hookPrintResults(results)

	return nil
}

func hookPrintResults(results []private.HookPostReceiveBranchResult) {
	for _, res := range results {
		if !res.Message {
			continue
		}

		fmt.Fprintln(os.Stderr, "")
		if res.Create {
			fmt.Fprintf(os.Stderr, "Create a new pull request for '%s':\n", res.Branch)
			fmt.Fprintf(os.Stderr, "  %s\n", res.URL)
		} else {
			fmt.Fprint(os.Stderr, "Visit the existing pull request:\n")
			fmt.Fprintf(os.Stderr, "  %s\n", res.URL)
		}
		fmt.Fprintln(os.Stderr, "")
		os.Stderr.Sync()
	}
}

func pushOptions() map[string]string {
	opts := make(map[string]string)
	if pushCount, err := strconv.Atoi(os.Getenv(private.GitPushOptionCount)); err == nil {
		for idx := 0; idx < pushCount; idx++ {
			opt := os.Getenv(fmt.Sprintf("GIT_PUSH_OPTION_%d", idx))
			kv := strings.SplitN(opt, "=", 2)
			if len(kv) == 2 {
				opts[kv[0]] = kv[1]
			}
		}
	}
	return opts
}
