// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"container/list"
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

// GPGSettings represents the default GPG settings for this repository
type GPGSettings struct {
	Sign             bool
	KeyID            string
	Email            string
	Name             string
	PublicKeyContent string
}

const prettyLogFormat = `--pretty=format:%H`

// GetAllCommitsCount returns count of all commits in repository
func (repo *Repository) GetAllCommitsCount() (int64, error) {
	return AllCommitsCount(repo.Path, false)
}

func (repo *Repository) parsePrettyFormatLogToList(logs []byte) (*list.List, error) {
	l := list.New()
	if len(logs) == 0 {
		return l, nil
	}

	parts := bytes.Split(logs, []byte{'\n'})

	for _, commitID := range parts {
		commit, err := repo.GetCommit(string(commitID))
		if err != nil {
			return nil, err
		}
		l.PushBack(commit)
	}

	return l, nil
}

// IsRepoURLAccessible checks if given repository URL is accessible.
func IsRepoURLAccessible(url string) bool {
	_, err := NewCommand("ls-remote", "-q", "-h", url, "HEAD").Run()
	return err == nil
}

// InitRepository initializes a new Git repository.
func InitRepository(repoPath string, bare bool) error {
	err := os.MkdirAll(repoPath, os.ModePerm)
	if err != nil {
		return err
	}

	cmd := NewCommand("init")
	if bare {
		cmd.AddArguments("--bare")
	}
	_, err = cmd.RunInDir(repoPath)
	return err
}

// IsEmpty Check if repository is empty.
func (repo *Repository) IsEmpty() (bool, error) {
	var errbuf strings.Builder
	if err := NewCommand("log", "-1").RunInDirPipeline(repo.Path, nil, &errbuf); err != nil {
		if strings.Contains(errbuf.String(), "fatal: bad default revision 'HEAD'") ||
			strings.Contains(errbuf.String(), "fatal: your current branch 'master' does not have any commits yet") {
			return true, nil
		}
		return true, fmt.Errorf("check empty: %v - %s", err, errbuf.String())
	}

	return false, nil
}

// CloneRepoOptions options when clone a repository
type CloneRepoOptions struct {
	Timeout    time.Duration
	Mirror     bool
	Bare       bool
	Quiet      bool
	Branch     string
	Shared     bool
	NoCheckout bool
	Depth      int
}

// Clone clones original repository to target path.
func Clone(from, to string, opts CloneRepoOptions) (err error) {
	return CloneWithContext(DefaultContext, from, to, opts)
}

// CloneWithContext clones original repository to target path.
func CloneWithContext(ctx context.Context, from, to string, opts CloneRepoOptions) (err error) {
	cargs := make([]string, len(GlobalCommandArgs))
	copy(cargs, GlobalCommandArgs)
	return CloneWithArgs(ctx, from, to, cargs, opts)
}

// CloneWithArgs original repository to target path.
func CloneWithArgs(ctx context.Context, from, to string, args []string, opts CloneRepoOptions) (err error) {
	toDir := path.Dir(to)
	if err = os.MkdirAll(toDir, os.ModePerm); err != nil {
		return err
	}

	cmd := NewCommandContextNoGlobals(ctx, args...).AddArguments("clone")
	if opts.Mirror {
		cmd.AddArguments("--mirror")
	}
	if opts.Bare {
		cmd.AddArguments("--bare")
	}
	if opts.Quiet {
		cmd.AddArguments("--quiet")
	}
	if opts.Shared {
		cmd.AddArguments("-s")
	}
	if opts.NoCheckout {
		cmd.AddArguments("--no-checkout")
	}
	if opts.Depth > 0 {
		cmd.AddArguments("--depth", strconv.Itoa(opts.Depth))
	}

	if len(opts.Branch) > 0 {
		cmd.AddArguments("-b", opts.Branch)
	}
	cmd.AddArguments("--", from, to)

	if opts.Timeout <= 0 {
		opts.Timeout = -1
	}

	_, err = cmd.RunTimeout(opts.Timeout)
	return err
}

// PullRemoteOptions options when pull from remote
type PullRemoteOptions struct {
	Timeout time.Duration
	All     bool
	Rebase  bool
	Remote  string
	Branch  string
}

// Pull pulls changes from remotes.
func Pull(repoPath string, opts PullRemoteOptions) error {
	cmd := NewCommand("pull")
	if opts.Rebase {
		cmd.AddArguments("--rebase")
	}
	if opts.All {
		cmd.AddArguments("--all")
	} else {
		cmd.AddArguments("--", opts.Remote, opts.Branch)
	}

	if opts.Timeout <= 0 {
		opts.Timeout = -1
	}

	_, err := cmd.RunInDirTimeout(opts.Timeout, repoPath)
	return err
}

// PushOptions options when push to remote
type PushOptions struct {
	Remote  string
	Branch  string
	Force   bool
	Mirror  bool
	Env     []string
	Timeout time.Duration
}

// Push pushs local commits to given remote branch.
func Push(repoPath string, opts PushOptions) error {
	cmd := NewCommand("push")
	if opts.Force {
		cmd.AddArguments("-f")
	}
	if opts.Mirror {
		cmd.AddArguments("--mirror")
	}
	cmd.AddArguments("--", opts.Remote)
	if len(opts.Branch) > 0 {
		cmd.AddArguments(opts.Branch)
	}
	var outbuf, errbuf strings.Builder

	if opts.Timeout == 0 {
		opts.Timeout = -1
	}

	err := cmd.RunInDirTimeoutEnvPipeline(opts.Env, opts.Timeout, repoPath, &outbuf, &errbuf)
	if err != nil {
		if strings.Contains(errbuf.String(), "non-fast-forward") {
			return &ErrPushOutOfDate{
				StdOut: outbuf.String(),
				StdErr: errbuf.String(),
				Err:    err,
			}
		} else if strings.Contains(errbuf.String(), "! [remote rejected]") {
			err := &ErrPushRejected{
				StdOut: outbuf.String(),
				StdErr: errbuf.String(),
				Err:    err,
			}
			err.GenerateMessage()
			return err
		} else if strings.Contains(errbuf.String(), "matches more than one") {
			err := &ErrMoreThanOne{
				StdOut: outbuf.String(),
				StdErr: errbuf.String(),
				Err:    err,
			}
			return err
		}
	}

	if errbuf.Len() > 0 && err != nil {
		return fmt.Errorf("%v - %s", err, errbuf.String())
	}

	return err
}

// CheckoutOptions options when heck out some branch
type CheckoutOptions struct {
	Timeout   time.Duration
	Branch    string
	OldBranch string
}

// Checkout checkouts a branch
func Checkout(repoPath string, opts CheckoutOptions) error {
	cmd := NewCommand("checkout")
	if len(opts.OldBranch) > 0 {
		cmd.AddArguments("-b")
	}

	if opts.Timeout <= 0 {
		opts.Timeout = -1
	}

	cmd.AddArguments(opts.Branch)

	if len(opts.OldBranch) > 0 {
		cmd.AddArguments(opts.OldBranch)
	}

	_, err := cmd.RunInDirTimeout(opts.Timeout, repoPath)
	return err
}

// ResetHEAD resets HEAD to given revision or head of branch.
func ResetHEAD(repoPath string, hard bool, revision string) error {
	cmd := NewCommand("reset")
	if hard {
		cmd.AddArguments("--hard")
	}
	_, err := cmd.AddArguments(revision).RunInDir(repoPath)
	return err
}

// MoveFile moves a file to another file or directory.
func MoveFile(repoPath, oldTreeName, newTreeName string) error {
	_, err := NewCommand("mv").AddArguments(oldTreeName, newTreeName).RunInDir(repoPath)
	return err
}

// CountObject represents repository count objects report
type CountObject struct {
	Count       int64
	Size        int64
	InPack      int64
	Packs       int64
	SizePack    int64
	PrunePack   int64
	Garbage     int64
	SizeGarbage int64
}

const (
	statCount        = "count: "
	statSize         = "size: "
	statInpack       = "in-pack: "
	statPacks        = "packs: "
	statSizePack     = "size-pack: "
	statPrunePackage = "prune-package: "
	statGarbage      = "garbage: "
	statSizeGarbage  = "size-garbage: "
)

// CountObjects returns the results of git count-objects on the repoPath
func CountObjects(repoPath string) (*CountObject, error) {
	cmd := NewCommand("count-objects", "-v")
	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return nil, err
	}

	return parseSize(stdout), nil
}

// parseSize parses the output from count-objects and return a CountObject
func parseSize(objects string) *CountObject {
	repoSize := new(CountObject)
	for _, line := range strings.Split(objects, "\n") {
		switch {
		case strings.HasPrefix(line, statCount):
			repoSize.Count, _ = strconv.ParseInt(line[7:], 10, 64)
		case strings.HasPrefix(line, statSize):
			repoSize.Size, _ = strconv.ParseInt(line[6:], 10, 64)
			repoSize.Size *= 1024
		case strings.HasPrefix(line, statInpack):
			repoSize.InPack, _ = strconv.ParseInt(line[9:], 10, 64)
		case strings.HasPrefix(line, statPacks):
			repoSize.Packs, _ = strconv.ParseInt(line[7:], 10, 64)
		case strings.HasPrefix(line, statSizePack):
			repoSize.Count, _ = strconv.ParseInt(line[11:], 10, 64)
			repoSize.Count *= 1024
		case strings.HasPrefix(line, statPrunePackage):
			repoSize.PrunePack, _ = strconv.ParseInt(line[16:], 10, 64)
		case strings.HasPrefix(line, statGarbage):
			repoSize.Garbage, _ = strconv.ParseInt(line[9:], 10, 64)
		case strings.HasPrefix(line, statSizeGarbage):
			repoSize.SizeGarbage, _ = strconv.ParseInt(line[14:], 10, 64)
			repoSize.SizeGarbage *= 1024
		}
	}
	return repoSize
}

// GetLatestCommitTime returns time for latest commit in repository (across all branches)
func GetLatestCommitTime(repoPath string) (time.Time, error) {
	cmd := NewCommand("for-each-ref", "--sort=-committerdate", "refs/heads/", "--count", "1", "--format=%(committerdate)")
	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return time.Time{}, err
	}
	commitTime := strings.TrimSpace(stdout)
	return time.Parse(GitTimeLayout, commitTime)
}

// DivergeObject represents commit count diverging commits
type DivergeObject struct {
	Ahead  int
	Behind int
}

func checkDivergence(repoPath string, baseBranch string, targetBranch string) (int, error) {
	branches := fmt.Sprintf("%s..%s", baseBranch, targetBranch)
	cmd := NewCommand("rev-list", "--count", branches)
	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return -1, err
	}
	outInteger, errInteger := strconv.Atoi(strings.Trim(stdout, "\n"))
	if errInteger != nil {
		return -1, errInteger
	}
	return outInteger, nil
}

// GetDivergingCommits returns the number of commits a targetBranch is ahead or behind a baseBranch
func GetDivergingCommits(repoPath string, baseBranch string, targetBranch string) (DivergeObject, error) {
	// $(git rev-list --count master..feature) commits ahead of master
	ahead, errorAhead := checkDivergence(repoPath, baseBranch, targetBranch)
	if errorAhead != nil {
		return DivergeObject{}, errorAhead
	}

	// $(git rev-list --count feature..master) commits behind master
	behind, errorBehind := checkDivergence(repoPath, targetBranch, baseBranch)
	if errorBehind != nil {
		return DivergeObject{}, errorBehind
	}

	return DivergeObject{ahead, behind}, nil
}
