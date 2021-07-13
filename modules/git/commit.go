// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"bytes"
	"container/list"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// Commit represents a git commit.
type Commit struct {
	Branch string // Branch this commit belongs to
	Tree
	ID            SHA1 // The ID of this commit object
	Author        *Signature
	Committer     *Signature
	CommitMessage string
	Signature     *CommitGPGSignature

	Parents        []SHA1 // SHA1 strings
	submoduleCache *ObjectCache
}

// CommitGPGSignature represents a git commit signature part.
type CommitGPGSignature struct {
	Signature string
	Payload   string //TODO check if can be reconstruct from the rest of commit information to not have duplicate data
}

// Message returns the commit message. Same as retrieving CommitMessage directly.
func (c *Commit) Message() string {
	return c.CommitMessage
}

// Summary returns first line of commit message.
func (c *Commit) Summary() string {
	return strings.Split(strings.TrimSpace(c.CommitMessage), "\n")[0]
}

// ParentID returns oid of n-th parent (0-based index).
// It returns nil if no such parent exists.
func (c *Commit) ParentID(n int) (SHA1, error) {
	if n >= len(c.Parents) {
		return SHA1{}, ErrNotExist{"", ""}
	}
	return c.Parents[n], nil
}

// Parent returns n-th parent (0-based index) of the commit.
func (c *Commit) Parent(n int) (*Commit, error) {
	id, err := c.ParentID(n)
	if err != nil {
		return nil, err
	}
	parent, err := c.repo.getCommit(id)
	if err != nil {
		return nil, err
	}
	return parent, nil
}

// ParentCount returns number of parents of the commit.
// 0 if this is the root commit,  otherwise 1,2, etc.
func (c *Commit) ParentCount() int {
	return len(c.Parents)
}

// GetCommitByPath return the commit of relative path object.
func (c *Commit) GetCommitByPath(relpath string) (*Commit, error) {
	return c.repo.getCommitByPathWithID(c.ID, relpath)
}

// AddChanges marks local changes to be ready for commit.
func AddChanges(repoPath string, all bool, files ...string) error {
	return AddChangesWithArgs(repoPath, GlobalCommandArgs, all, files...)
}

// AddChangesWithArgs marks local changes to be ready for commit.
func AddChangesWithArgs(repoPath string, gloablArgs []string, all bool, files ...string) error {
	cmd := NewCommandNoGlobals(append(gloablArgs, "add")...)
	if all {
		cmd.AddArguments("--all")
	}
	cmd.AddArguments("--")
	_, err := cmd.AddArguments(files...).RunInDir(repoPath)
	return err
}

// CommitChangesOptions the options when a commit created
type CommitChangesOptions struct {
	Committer *Signature
	Author    *Signature
	Message   string
}

// CommitChanges commits local changes with given committer, author and message.
// If author is nil, it will be the same as committer.
func CommitChanges(repoPath string, opts CommitChangesOptions) error {
	cargs := make([]string, len(GlobalCommandArgs))
	copy(cargs, GlobalCommandArgs)
	return CommitChangesWithArgs(repoPath, cargs, opts)
}

// CommitChangesWithArgs commits local changes with given committer, author and message.
// If author is nil, it will be the same as committer.
func CommitChangesWithArgs(repoPath string, args []string, opts CommitChangesOptions) error {
	cmd := NewCommandNoGlobals(args...)
	if opts.Committer != nil {
		cmd.AddArguments("-c", "user.name="+opts.Committer.Name, "-c", "user.email="+opts.Committer.Email)
	}
	cmd.AddArguments("commit")

	if opts.Author == nil {
		opts.Author = opts.Committer
	}
	if opts.Author != nil {
		cmd.AddArguments(fmt.Sprintf("--author='%s <%s>'", opts.Author.Name, opts.Author.Email))
	}
	cmd.AddArguments("-m", opts.Message)

	_, err := cmd.RunInDir(repoPath)
	// No stderr but exit status 1 means nothing to commit.
	if err != nil && err.Error() == "exit status 1" {
		return nil
	}
	return err
}

// AllCommitsCount returns count of all commits in repository
func AllCommitsCount(repoPath string, hidePRRefs bool, files ...string) (int64, error) {
	args := []string{"--all", "--count"}
	if hidePRRefs {
		args = append([]string{"--exclude=refs/pull/*"}, args...)
	}
	cmd := NewCommand("rev-list")
	cmd.AddArguments(args...)
	if len(files) > 0 {
		cmd.AddArguments("--")
		cmd.AddArguments(files...)
	}

	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
}

// CommitsCountFiles returns number of total commits of until given revision.
func CommitsCountFiles(repoPath string, revision, relpath []string) (int64, error) {
	cmd := NewCommand("rev-list", "--count")
	cmd.AddArguments(revision...)
	if len(relpath) > 0 {
		cmd.AddArguments("--")
		cmd.AddArguments(relpath...)
	}

	stdout, err := cmd.RunInDir(repoPath)
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSpace(stdout), 10, 64)
}

// CommitsCount returns number of total commits of until given revision.
func CommitsCount(repoPath string, revision ...string) (int64, error) {
	return CommitsCountFiles(repoPath, revision, []string{})
}

// CommitsCount returns number of total commits of until current revision.
func (c *Commit) CommitsCount() (int64, error) {
	return CommitsCount(c.repo.Path, c.ID.String())
}

// CommitsByRange returns the specific page commits before current revision, every page's number default by CommitsRangeSize
func (c *Commit) CommitsByRange(page, pageSize int) (*list.List, error) {
	return c.repo.commitsByRange(c.ID, page, pageSize)
}

// CommitsBefore returns all the commits before current revision
func (c *Commit) CommitsBefore() (*list.List, error) {
	return c.repo.getCommitsBefore(c.ID)
}

// HasPreviousCommit returns true if a given commitHash is contained in commit's parents
func (c *Commit) HasPreviousCommit(commitHash SHA1) (bool, error) {
	this := c.ID.String()
	that := commitHash.String()

	if this == that {
		return false, nil
	}

	if err := CheckGitVersionAtLeast("1.8"); err == nil {
		_, err := NewCommand("merge-base", "--is-ancestor", that, this).RunInDir(c.repo.Path)
		if err == nil {
			return true, nil
		}
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			if exitError.ProcessState.ExitCode() == 1 && len(exitError.Stderr) == 0 {
				return false, nil
			}
		}
		return false, err
	}

	result, err := NewCommand("rev-list", "--ancestry-path", "-n1", that+".."+this, "--").RunInDir(c.repo.Path)
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(result)) > 0, nil
}

// CommitsBeforeLimit returns num commits before current revision
func (c *Commit) CommitsBeforeLimit(num int) (*list.List, error) {
	return c.repo.getCommitsBeforeLimit(c.ID, num)
}

// CommitsBeforeUntil returns the commits between commitID to current revision
func (c *Commit) CommitsBeforeUntil(commitID string) (*list.List, error) {
	endCommit, err := c.repo.GetCommit(commitID)
	if err != nil {
		return nil, err
	}
	return c.repo.CommitsBetween(c, endCommit)
}

// SearchCommitsOptions specify the parameters for SearchCommits
type SearchCommitsOptions struct {
	Keywords            []string
	Authors, Committers []string
	After, Before       string
	All                 bool
}

// NewSearchCommitsOptions construct a SearchCommitsOption from a space-delimited search string
func NewSearchCommitsOptions(searchString string, forAllRefs bool) SearchCommitsOptions {
	var keywords, authors, committers []string
	var after, before string

	fields := strings.Fields(searchString)
	for _, k := range fields {
		switch {
		case strings.HasPrefix(k, "author:"):
			authors = append(authors, strings.TrimPrefix(k, "author:"))
		case strings.HasPrefix(k, "committer:"):
			committers = append(committers, strings.TrimPrefix(k, "committer:"))
		case strings.HasPrefix(k, "after:"):
			after = strings.TrimPrefix(k, "after:")
		case strings.HasPrefix(k, "before:"):
			before = strings.TrimPrefix(k, "before:")
		default:
			keywords = append(keywords, k)
		}
	}

	return SearchCommitsOptions{
		Keywords:   keywords,
		Authors:    authors,
		Committers: committers,
		After:      after,
		Before:     before,
		All:        forAllRefs,
	}
}

// SearchCommits returns the commits match the keyword before current revision
func (c *Commit) SearchCommits(opts SearchCommitsOptions) (*list.List, error) {
	return c.repo.searchCommits(c.ID, opts)
}

// GetFilesChangedSinceCommit get all changed file names between pastCommit to current revision
func (c *Commit) GetFilesChangedSinceCommit(pastCommit string) ([]string, error) {
	return c.repo.getFilesChanged(pastCommit, c.ID.String())
}

// FileChangedSinceCommit Returns true if the file given has changed since the the past commit
// YOU MUST ENSURE THAT pastCommit is a valid commit ID.
func (c *Commit) FileChangedSinceCommit(filename, pastCommit string) (bool, error) {
	return c.repo.FileChangedBetweenCommits(filename, pastCommit, c.ID.String())
}

// HasFile returns true if the file given exists on this commit
// This does only mean it's there - it does not mean the file was changed during the commit.
func (c *Commit) HasFile(filename string) (bool, error) {
	_, err := c.GetBlobByPath(filename)
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetSubModules get all the sub modules of current revision git tree
func (c *Commit) GetSubModules() (*ObjectCache, error) {
	if c.submoduleCache != nil {
		return c.submoduleCache, nil
	}

	entry, err := c.GetTreeEntryByPath(".gitmodules")
	if err != nil {
		if _, ok := err.(ErrNotExist); ok {
			return nil, nil
		}
		return nil, err
	}

	rd, err := entry.Blob().DataAsync()
	if err != nil {
		return nil, err
	}

	defer rd.Close()
	scanner := bufio.NewScanner(rd)
	c.submoduleCache = newObjectCache()
	var ismodule bool
	var path string
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "[submodule") {
			ismodule = true
			continue
		}
		if ismodule {
			fields := strings.Split(scanner.Text(), "=")
			k := strings.TrimSpace(fields[0])
			if k == "path" {
				path = strings.TrimSpace(fields[1])
			} else if k == "url" {
				c.submoduleCache.Set(path, &SubModule{path, strings.TrimSpace(fields[1])})
				ismodule = false
			}
		}
	}

	return c.submoduleCache, nil
}

// GetSubModule get the sub module according entryname
func (c *Commit) GetSubModule(entryname string) (*SubModule, error) {
	modules, err := c.GetSubModules()
	if err != nil {
		return nil, err
	}

	if modules != nil {
		module, has := modules.Get(entryname)
		if has {
			return module.(*SubModule), nil
		}
	}
	return nil, nil
}

// GetBranchName gets the closest branch name (as returned by 'git name-rev --name-only')
func (c *Commit) GetBranchName() (string, error) {
	err := LoadGitVersion()
	if err != nil {
		return "", fmt.Errorf("Git version missing: %v", err)
	}

	args := []string{
		"name-rev",
	}
	if CheckGitVersionAtLeast("2.13.0") == nil {
		args = append(args, "--exclude", "refs/tags/*")
	}
	args = append(args, "--name-only", "--no-undefined", c.ID.String())

	data, err := NewCommand(args...).RunInDir(c.repo.Path)
	if err != nil {
		// handle special case where git can not describe commit
		if strings.Contains(err.Error(), "cannot describe") {
			return "", nil
		}

		return "", err
	}

	// name-rev commitID output will be "master" or "master~12"
	return strings.SplitN(strings.TrimSpace(data), "~", 2)[0], nil
}

// LoadBranchName load branch name for commit
func (c *Commit) LoadBranchName() (err error) {
	if len(c.Branch) != 0 {
		return
	}

	c.Branch, err = c.GetBranchName()
	return
}

// GetTagName gets the current tag name for given commit
func (c *Commit) GetTagName() (string, error) {
	data, err := NewCommand("describe", "--exact-match", "--tags", "--always", c.ID.String()).RunInDir(c.repo.Path)
	if err != nil {
		// handle special case where there is no tag for this commit
		if strings.Contains(err.Error(), "no tag exactly matches") {
			return "", nil
		}

		return "", err
	}

	return strings.TrimSpace(data), nil
}

// CommitFileStatus represents status of files in a commit.
type CommitFileStatus struct {
	Added    []string
	Removed  []string
	Modified []string
}

// NewCommitFileStatus creates a CommitFileStatus
func NewCommitFileStatus() *CommitFileStatus {
	return &CommitFileStatus{
		[]string{}, []string{}, []string{},
	}
}

func parseCommitFileStatus(fileStatus *CommitFileStatus, stdout io.Reader) {
	rd := bufio.NewReader(stdout)
	peek, err := rd.Peek(1)
	if err != nil {
		if err != io.EOF {
			log.Error("Unexpected error whilst reading from git log --name-status. Error: %v", err)
		}
		return
	}
	if peek[0] == '\n' || peek[0] == '\x00' {
		_, _ = rd.Discard(1)
	}
	for {
		modifier, err := rd.ReadSlice('\x00')
		if err != nil {
			if err != io.EOF {
				log.Error("Unexpected error whilst reading from git log --name-status. Error: %v", err)
			}
			return
		}
		file, err := rd.ReadString('\x00')
		if err != nil {
			if err != io.EOF {
				log.Error("Unexpected error whilst reading from git log --name-status. Error: %v", err)
			}
			return
		}
		file = file[:len(file)-1]
		switch modifier[0] {
		case 'A':
			fileStatus.Added = append(fileStatus.Added, file)
		case 'D':
			fileStatus.Removed = append(fileStatus.Removed, file)
		case 'M':
			fileStatus.Modified = append(fileStatus.Modified, file)
		}
	}
}

// GetCommitFileStatus returns file status of commit in given repository.
func GetCommitFileStatus(repoPath, commitID string) (*CommitFileStatus, error) {
	stdout, w := io.Pipe()
	done := make(chan struct{})
	fileStatus := NewCommitFileStatus()
	go func() {
		parseCommitFileStatus(fileStatus, stdout)
		close(done)
	}()

	stderr := new(bytes.Buffer)
	args := []string{"log", "--name-status", "-c", "--pretty=format:", "--parents", "--no-renames", "-z", "-1", commitID}

	err := NewCommand(args...).RunInDirPipeline(repoPath, w, stderr)
	w.Close() // Close writer to exit parsing goroutine
	if err != nil {
		return nil, ConcatenateError(err, stderr.String())
	}

	<-done
	return fileStatus, nil
}

// GetFullCommitID returns full length (40) of commit ID by given short SHA in a repository.
func GetFullCommitID(repoPath, shortID string) (string, error) {
	commitID, err := NewCommand("rev-parse", shortID).RunInDir(repoPath)
	if err != nil {
		if strings.Contains(err.Error(), "exit status 128") {
			return "", ErrNotExist{shortID, ""}
		}
		return "", err
	}
	return strings.TrimSpace(commitID), nil
}

// GetRepositoryDefaultPublicGPGKey returns the default public key for this commit
func (c *Commit) GetRepositoryDefaultPublicGPGKey(forceUpdate bool) (*GPGSettings, error) {
	if c.repo == nil {
		return nil, nil
	}
	return c.repo.GetDefaultPublicGPGKey(forceUpdate)
}
