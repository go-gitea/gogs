// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"container/list"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	logger "code.gitea.io/gitea/modules/log"
)

// CompareInfo represents needed information for comparing references.
type CompareInfo struct {
	MergeBase string
	Commits   *list.List
	NumFiles  int
}

// DiffFiles slice of DiffFile
type DiffFiles struct {
	Files []*DiffFile
}

// DiffFile represents stats of a file modified between two commits (based on gitdiff.DiffFile)
type DiffFile struct {
	Name               string
	OldName            string
	Index              int
	Addition, Deletion int
	Type               uint8
	IsCreated          bool
	IsDeleted          bool
	IsBin              bool
	IsLFSFile          bool
	IsRenamed          bool
	IsSubmodule        bool
	IsIncomplete       bool
	IsProtected        bool
	IsIgnored          bool
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
	compareInfo.MergeBase, remoteBranch, err = repo.GetMergeBase(tmpRemote, baseBranch, headBranch)
	if err == nil {
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
		compareInfo.Commits = list.New()
		compareInfo.MergeBase, err = GetFullCommitID(repo.Path, remoteBranch)
		if err != nil {
			compareInfo.MergeBase = remoteBranch
		}
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
	numFiles, totalAdditions, totalDeletions, _, err = parseDiffStat(stdout)
	if err != nil {
		return 0, 0, 0, err
	}
	return
}

// GetDiffAllStats counts number of changed files, number of additions and deletions
func GetDiffAllStats(repoPath string, args ...string) (numFiles, totalAdditions, totalDeletions int, filesChanged *DiffFiles, err error) {
	args = append([]string{
		"diff-tree",
		"--raw",
		"-r",
		"--find-renames=100%",
		"--find-copies=100%",
		"--numstat",
		"--shortstat",
		"-z",
	}, args...)

	stdout, err := NewCommand(args...).RunInDir(repoPath)
	if err != nil {
		return 0, 0, 0, filesChanged, err
	}
	return parseDiffStat(stdout)
}

var shortStatFormat = regexp.MustCompile(`\s*(\d+) files? changed(?:, (\d+) insertions?\(\+\))?(?:, (\d+) deletions?\(-\))?`)

// If file is binary numstat will use "-" in the added/deleted lines columns
// use /s regex option since it is technically possible for odd filenames to include newlines
var fileChange = regexp.MustCompile(`(?s)(\d+|-)\s+(\d+|-)\s+(.+)$`)

func parseDiffStat(stdout string) (numFiles, totalAdditions, totalDeletions int, filesChanged *DiffFiles, err error) {
	filesChanged = &DiffFiles{}
	if len(stdout) == 0 || stdout == "\n" {
		return 0, 0, 0, filesChanged, nil
	}
	// map of files we've seen using full filename as unique key
	m := make(map[string]*DiffFile)

	// split on NUL because we used -z option to git diff-tree
	lines := strings.Split(stdout, "\x00")

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		// Extra information beyond filename is mostly for the benefit of vendored/ignored files for now
		// For normal files we will find this information when parsing the patch file
		// In the future we can use it to replace that
		if strings.HasPrefix(line, ":") && i+2 < len(lines) {
			switch line[strings.LastIndex(line, " ")+1:] {
			// Detect Renamed files:
			// :100644 100644 92e798b17543cade621b435eb56946896a70f365 92e798b17543cade621b435eb56946896a70f365 R100\x00b b/b\x00b b/b b/b b/b\x00
			case "R100":
				oldName := strconv.Quote(lines[i+1])
				fileName := lines[i+2]
				m[fileName] = &DiffFile{}
				m[fileName].Name = fileName
				m[fileName].OldName = oldName
				m[fileName].IsRenamed = true
				m[fileName].Type = 4
				i += 2
				continue
			// Detect Added Files
			// :000000 100644 0000000000000000000000000000000000000000 aa541dd7e2b1e9262154f8113e1557106840d361 A\x00Icon\x00
			case "A":
				fileName := strconv.Quote(lines[i+1])
				m[fileName] = &DiffFile{}
				m[fileName].Name = fileName
				m[fileName].IsCreated = true
				m[fileName].Type = 1
				i++
				continue
				// Detect Modified Files
				// :000000 100644 0000000000000000000000000000000000000000 11fd2cb03f758e1f79518d382579512be10e134c A\x00cmd/flags.go\x00
			case "M":
				fileName := strconv.Quote(lines[i+1])
				m[fileName] = &DiffFile{}
				m[fileName].Name = fileName
				m[fileName].Type = 2
				i++
				continue
			// Detect Deleted Files
			// :100644 000000 c836a24d0b335033fc6e2c24e191072323e32e32 0000000000000000000000000000000000000000 D\x00readme.txt\x00
			case "D":
				fileName := strconv.Quote(lines[i+1])
				m[fileName] = &DiffFile{}
				m[fileName].Name = fileName
				m[fileName].IsDeleted = true
				m[fileName].Type = 3
				i++
			// Detect Copied Files
			// :100644 100644 92e798b17543cade621b435eb56946896a70f365 92e798b17543cade621b435eb56946896a70f365 C100\x00a\x00b\x00
			case "C100":
				oldName := strconv.Quote(lines[i+1])
				fileName := strconv.Quote(lines[i+2])
				m[fileName] = &DiffFile{}
				m[fileName].Name = fileName
				m[fileName].OldName = oldName
				m[fileName].IsRenamed = true
				m[fileName].Type = 5
			default:
				logger.Error("parseDiffStat: We don't recognize this git diff raw line", line)
				continue
			}
			continue
		}
		numstat := fileChange.FindStringSubmatch(lines[i])
		if len(numstat) > 0 {
			if len(numstat) != 4 {
				logger.Error("parseDiffStat: Error parsing changed file: %v", numstat)
				continue
			} else {
				fileName := strconv.Quote(numstat[3])
				if _, ok := m[fileName]; !ok {
					// This shouldn't happen and indicates an error parsing somewhere
					logger.Error("parseDiffStat: We found a file in numstat output that we didn't see in raw output:", line)
					continue
				}
				if numstat[1] == "-" {
					m[fileName].Addition = -1
					m[fileName].IsBin = true
				} else {
					m[fileName].Addition, err = strconv.Atoi(numstat[1])
					if err != nil {
						logger.Error("parseDiffStat: can't strconv %s: %v", numstat[1], err)
					}
				}
				if numstat[2] == "-" {
					m[fileName].Deletion = -1
				} else {
					m[fileName].Deletion, err = strconv.Atoi(numstat[2])
					if err != nil {
						logger.Error("parseDiffStat: can't strconv %s: %v", numstat[2], err)
					}
				}
				continue
			}
		}
		groups := shortStatFormat.FindStringSubmatch(lines[i])
		if len(groups) > 0 {
			if len(groups) != 4 {
				return 0, 0, 0, filesChanged, fmt.Errorf("unable to parse shortstat: %s groups: %s", stdout, groups)
			}
			numFiles, err = strconv.Atoi(groups[1])
			if err != nil {
				return 0, 0, 0, filesChanged, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumFiles %v", stdout, err)
			}
			if len(groups[2]) != 0 {
				totalAdditions, err = strconv.Atoi(groups[2])
				if err != nil {
					return 0, 0, 0, filesChanged, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumAdditions %v", stdout, err)
				}
			}
			if len(groups[3]) != 0 {
				totalDeletions, err = strconv.Atoi(groups[3])
				if err != nil {
					return 0, 0, 0, filesChanged, fmt.Errorf("unable to parse shortstat: %s. Error parsing NumDeletions %v", stdout, err)
				}
			}
			// this should always be last line
			break
		}
	}

	// sort so we always have consistent output
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		filesChanged.Files = append(filesChanged.Files, m[k])
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
