// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	logger "code.gitea.io/gitea/modules/log"
)

// CompareInfo represents needed information for comparing references.
type CompareInfo struct {
	MergeBase    string
	BaseCommitID string
	HeadCommitID string
	Commits      []*Commit
	NumFiles     int
}

// GetMergeBase checks and returns merge base of two branches and the reference used as base.
func (repo *Repository) GetMergeBase(tmpRemote string, base, head string) (string, string, error) {
	if tmpRemote == "" {
		tmpRemote = "origin"
	}

	if tmpRemote != "origin" {
		tmpBaseName := "refs/remotes/" + tmpRemote + "/tmp_" + base
		// Fetch commit into a temporary branch in order to be able to handle commits and tags
		_, err := NewCommand("fetch", tmpRemote, base+":"+tmpBaseName).RunInDir(repo.Path)
		if err == nil {
			base = tmpBaseName
		}
	}

	stdout, err := NewCommand("merge-base", "--", base, head).RunInDir(repo.Path)
	return strings.TrimSpace(stdout), base, err
}

// GetCompareInfo generates and returns compare information between base and head branches of repositories.
func (repo *Repository) GetCompareInfo(basePath, baseBranch, headBranch string) (_ *CompareInfo, err error) {
	var (
		remoteBranch string
		tmpRemote    string
	)

	// We don't need a temporary remote for same repository.
	if repo.Path != basePath {
		// Add a temporary remote
		tmpRemote = strconv.FormatInt(time.Now().UnixNano(), 10)
		if err = repo.AddRemote(tmpRemote, basePath, false); err != nil {
			return nil, fmt.Errorf("AddRemote: %v", err)
		}
		defer func() {
			if err := repo.RemoveRemote(tmpRemote); err != nil {
				logger.Error("GetPullRequestInfo: RemoveRemote: %v", err)
			}
		}()
	}

	compareInfo := new(CompareInfo)

	compareInfo.HeadCommitID, err = GetFullCommitID(repo.Path, headBranch)
	if err != nil {
		compareInfo.HeadCommitID = headBranch
	}

	compareInfo.MergeBase, remoteBranch, err = repo.GetMergeBase(tmpRemote, baseBranch, headBranch)
	if err == nil {
		compareInfo.BaseCommitID, err = GetFullCommitID(repo.Path, remoteBranch)
		if err != nil {
			compareInfo.BaseCommitID = remoteBranch
		}
		// We have a common base - therefore we know that ... should work
		logs, err := NewCommand("log", compareInfo.MergeBase+"..."+headBranch, prettyLogFormat).RunInDirBytes(repo.Path)
		if err != nil {
			return nil, err
		}
		compareInfo.Commits, err = repo.parsePrettyFormatLogToList(logs)
		if err != nil {
			return nil, fmt.Errorf("parsePrettyFormatLogToList: %v", err)
		}
	} else {
		compareInfo.Commits = []*Commit{}
		compareInfo.MergeBase, err = GetFullCommitID(repo.Path, remoteBranch)
		if err != nil {
			compareInfo.MergeBase = remoteBranch
		}
		compareInfo.BaseCommitID = compareInfo.MergeBase
	}

	// Count number of changed files.
	// This probably should be removed as we need to use shortstat elsewhere
	// Now there is git diff --shortstat but this appears to be slower than simply iterating with --nameonly
	compareInfo.NumFiles, err = repo.GetDiffNumChangedFiles(remoteBranch, headBranch)
	if err != nil {
		return nil, err
	}
	return compareInfo, nil
}

type lineCountWriter struct {
	numLines int
}

// Write counts the number of newlines in the provided bytestream
func (l *lineCountWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	l.numLines += bytes.Count(p, []byte{'\000'})
	return
}

// GetDiffNumChangedFiles counts the number of changed files
// This is substantially quicker than shortstat but...
func (repo *Repository) GetDiffNumChangedFiles(base, head string) (int, error) {
	// Now there is git diff --shortstat but this appears to be slower than simply iterating with --nameonly
	w := &lineCountWriter{}
	stderr := new(bytes.Buffer)

	if err := NewCommand("diff", "-z", "--name-only", base+"..."+head).
		RunInDirPipeline(repo.Path, w, stderr); err != nil {
		if strings.Contains(stderr.String(), "no merge base") {
			// git >= 2.28 now returns an error if base and head have become unrelated.
			// previously it would return the results of git diff -z --name-only base head so let's try that...
			w = &lineCountWriter{}
			stderr.Reset()
			if err = NewCommand("diff", "-z", "--name-only", base, head).RunInDirPipeline(repo.Path, w, stderr); err == nil {
				return w.numLines, nil
			}
		}
		return 0, fmt.Errorf("%v: Stderr: %s", err, stderr)
	}
	return w.numLines, nil
}

// GetDiffShortStat counts number of changed files, number of additions and deletions
func (repo *Repository) GetDiffShortStat(base, head string) (numFiles, totalAdditions, totalDeletions int, err error) {
	numFiles, totalAdditions, totalDeletions, err = GetDiffShortStat(repo.Path, base+"..."+head)
	if err != nil && strings.Contains(err.Error(), "no merge base") {
		return GetDiffShortStat(repo.Path, base, head)
	}
	return
}

// GetDiffShortStat counts number of changed files, number of additions and deletions
func GetDiffShortStat(repoPath string, args ...string) (numFiles, totalAdditions, totalDeletions int, err error) {
	// Now if we call:
	// $ git diff --shortstat 1ebb35b98889ff77299f24d82da426b434b0cca0...788b8b1440462d477f45b0088875
	// we get:
	// " 9902 files changed, 2034198 insertions(+), 298800 deletions(-)\n"
	args = append([]string{
		"diff",
		"--shortstat",
	}, args...)

	stdout, err := NewCommand(args...).RunInDir(repoPath)
	if err != nil {
		return 0, 0, 0, err
	}

	return parseDiffStat(stdout)
}

var shortStatFormat = regexp.MustCompile(
	`\s*(\d+) files? changed(?:, (\d+) insertions?\(\+\))?(?:, (\d+) deletions?\(-\))?`)

func parseDiffStat(stdout string) (numFiles, totalAdditions, totalDeletions int, err error) {
	if len(stdout) == 0 || stdout == "\n" {
		return 0, 0, 0, nil
	}
	groups := shortStatFormat.FindStringSubmatch(stdout)
	if len(groups) != 4 {
		return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s groups: %s", stdout, groups)
	}

	numFiles, err = strconv.Atoi(groups[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumFiles %v", stdout, err)
	}

	if len(groups[2]) != 0 {
		totalAdditions, err = strconv.Atoi(groups[2])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumAdditions %v", stdout, err)
		}
	}

	if len(groups[3]) != 0 {
		totalDeletions, err = strconv.Atoi(groups[3])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumDeletions %v", stdout, err)
		}
	}
	return
}

// GetDiffOrPatch generates either diff or formatted patch data between given revisions
func (repo *Repository) GetDiffOrPatch(base, head string, w io.Writer, formatted bool) error {
	if formatted {
		return repo.GetPatch(base, head, w)
	}
	return repo.GetDiff(base, head, w)
}

// GetDiff generates and returns patch data between given revisions.
func (repo *Repository) GetDiff(base, head string, w io.Writer) error {
	return NewCommand("diff", "-p", "--binary", base, head).
		RunInDirPipeline(repo.Path, w, nil)
}

// GetPatch generates and returns format-patch data between given revisions.
func (repo *Repository) GetPatch(base, head string, w io.Writer) error {
	stderr := new(bytes.Buffer)
	err := NewCommand("format-patch", "--binary", "--stdout", base+"..."+head).
		RunInDirPipeline(repo.Path, w, stderr)
	if err != nil && bytes.Contains(stderr.Bytes(), []byte("no merge base")) {
		return NewCommand("format-patch", "--binary", "--stdout", base, head).
			RunInDirPipeline(repo.Path, w, nil)
	}
	return err
}

// GetDiffFromMergeBase generates and return patch data from merge base to head
func (repo *Repository) GetDiffFromMergeBase(base, head string, w io.Writer) error {
	stderr := new(bytes.Buffer)
	err := NewCommand("diff", "-p", "--binary", base+"..."+head).
		RunInDirPipeline(repo.Path, w, stderr)
	if err != nil && bytes.Contains(stderr.Bytes(), []byte("no merge base")) {
		return NewCommand("diff", "-p", "--binary", base, head).
			RunInDirPipeline(repo.Path, w, nil)
	}
	return err
}
