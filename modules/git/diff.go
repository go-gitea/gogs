// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
)

// RawDiffType type of a raw diff.
type RawDiffType string

// RawDiffType possible values.
const (
	RawDiffNormal RawDiffType = "diff"
	RawDiffPatch  RawDiffType = "patch"
)

// GetRawDiff dumps diff results of repository in given commit ID to io.Writer.
func GetRawDiff(repoPath, commitID string, diffType RawDiffType, writer io.Writer) error {
	return GetRawDiffForFile(repoPath, "", commitID, diffType, "", writer)
}

// GetRawDiffForFile dumps diff results of file in given commit ID to io.Writer.
func GetRawDiffForFile(repoPath, startCommit, endCommit string, diffType RawDiffType, file string, writer io.Writer) error {
	repo, err := OpenRepository(repoPath)
	if err != nil {
		return fmt.Errorf("OpenRepository: %v", err)
	}
	defer repo.Close()

	return GetRepoRawDiffForFile(repo, startCommit, endCommit, diffType, file, writer)
}

// GetRepoRawDiffForFile dumps diff results of file in given commit ID to io.Writer according given repository
func GetRepoRawDiffForFile(repo *Repository, startCommit, endCommit string, diffType RawDiffType, file string, writer io.Writer) error {
	commit, err := repo.GetCommit(endCommit)
	if err != nil {
		return err
	}
	fileArgs := make([]string, 0)
	if len(file) > 0 {
		fileArgs = append(fileArgs, "--", file)
	}
	// FIXME: graceful: These commands should have a timeout
	ctx, cancel := context.WithCancel(DefaultContext)
	defer cancel()

	var cmd *exec.Cmd
	switch diffType {
	case RawDiffNormal:
		if len(startCommit) != 0 {
			cmd = exec.CommandContext(ctx, GitExecutable, append([]string{"diff", "-M", startCommit, endCommit}, fileArgs...)...)
		} else if commit.ParentCount() == 0 {
			cmd = exec.CommandContext(ctx, GitExecutable, append([]string{"show", endCommit}, fileArgs...)...)
		} else {
			c, _ := commit.Parent(0)
			cmd = exec.CommandContext(ctx, GitExecutable, append([]string{"diff", "-M", c.ID.String(), endCommit}, fileArgs...)...)
		}
	case RawDiffPatch:
		if len(startCommit) != 0 {
			query := fmt.Sprintf("%s...%s", endCommit, startCommit)
			cmd = exec.CommandContext(ctx, GitExecutable, append([]string{"format-patch", "--no-signature", "--stdout", "--root", query}, fileArgs...)...)
		} else if commit.ParentCount() == 0 {
			cmd = exec.CommandContext(ctx, GitExecutable, append([]string{"format-patch", "--no-signature", "--stdout", "--root", endCommit}, fileArgs...)...)
		} else {
			c, _ := commit.Parent(0)
			query := fmt.Sprintf("%s...%s", endCommit, c.ID.String())
			cmd = exec.CommandContext(ctx, GitExecutable, append([]string{"format-patch", "--no-signature", "--stdout", query}, fileArgs...)...)
		}
	default:
		return fmt.Errorf("invalid diffType: %s", diffType)
	}

	stderr := new(bytes.Buffer)

	cmd.Dir = repo.Path
	cmd.Stdout = writer
	cmd.Stderr = stderr
	pid := process.GetManager().Add(fmt.Sprintf("GetRawDiffForFile: [repo_path: %s]", repo.Path), cancel)
	defer process.GetManager().Remove(pid)

	if err = cmd.Run(); err != nil {
		return fmt.Errorf("Run: %v - %s", err, stderr)
	}
	return nil
}

// ParseDiffHunkString parse the diffhunk content and return
func ParseDiffHunkString(diffhunk string) (leftLine, leftHunk, rightLine, righHunk int) {
	ss := strings.Split(diffhunk, "@@")
	ranges := strings.Split(ss[1][1:], " ")
	leftRange := strings.Split(ranges[0], ",")
	leftLine, _ = strconv.Atoi(leftRange[0][1:])
	if len(leftRange) > 1 {
		leftHunk, _ = strconv.Atoi(leftRange[1])
	}
	if len(ranges) > 1 {
		rightRange := strings.Split(ranges[1], ",")
		rightLine, _ = strconv.Atoi(rightRange[0])
		if len(rightRange) > 1 {
			righHunk, _ = strconv.Atoi(rightRange[1])
		}
	} else {
		log.Debug("Parse line number failed: %v", diffhunk)
		rightLine = leftLine
		righHunk = leftHunk
	}
	return
}

// Example: @@ -1,8 +1,9 @@ => [..., 1, 8, 1, 9]
var hunkRegex = regexp.MustCompile(`^@@ -(?P<beginOld>[0-9]+)(,(?P<endOld>[0-9]+))? \+(?P<beginNew>[0-9]+)(,(?P<endNew>[0-9]+))? @@`)

const cmdDiffHead = "diff --git "

func isHeader(lof string, inHunk bool) bool {
	return strings.HasPrefix(lof, cmdDiffHead) || (!inHunk && (strings.HasPrefix(lof, "---") || strings.HasPrefix(lof, "+++")))
}

// CutDiffAroundLine cuts a diff of a file in way that only the given line + numberOfLine above it will be shown
// it also recalculates hunks and adds the appropriate headers to the new diff.
// Warning: Only one-file diffs are allowed.
func CutDiffAroundLine(originalDiff io.Reader, line int64, old bool, numbersOfLine int) (string, error) {
	if line == 0 || numbersOfLine == 0 {
		// no line or num of lines => no diff
		return "", nil
	}

	scanner := bufio.NewScanner(originalDiff)
	hunk := make([]string, 0)

	// begin is the start of the hunk containing searched line
	// end is the end of the hunk ...
	// currentLine is the line number on the side of the searched line (differentiated by old)
	// otherLine is the line number on the opposite side of the searched line (differentiated by old)
	var begin, end, currentLine, otherLine int64
	var headerLines int

	inHunk := false

	for scanner.Scan() {
		lof := scanner.Text()
		// Add header to enable parsing

		if isHeader(lof, inHunk) {
			if strings.HasPrefix(lof, cmdDiffHead) {
				inHunk = false
			}
			hunk = append(hunk, lof)
			headerLines++
		}
		if currentLine > line {
			break
		}
		// Detect "hunk" with contains commented lof
		if strings.HasPrefix(lof, "@@") {
			inHunk = true
			// Already got our hunk. End of hunk detected!
			if len(hunk) > headerLines {
				break
			}
			// A map with named groups of our regex to recognize them later more easily
			submatches := hunkRegex.FindStringSubmatch(lof)
			groups := make(map[string]string)
			for i, name := range hunkRegex.SubexpNames() {
				if i != 0 && name != "" {
					groups[name] = submatches[i]
				}
			}
			if old {
				begin, _ = strconv.ParseInt(groups["beginOld"], 10, 64)
				end, _ = strconv.ParseInt(groups["endOld"], 10, 64)
				// init otherLine with begin of opposite side
				otherLine, _ = strconv.ParseInt(groups["beginNew"], 10, 64)
			} else {
				begin, _ = strconv.ParseInt(groups["beginNew"], 10, 64)
				if groups["endNew"] != "" {
					end, _ = strconv.ParseInt(groups["endNew"], 10, 64)
				} else {
					end = 0
				}
				// init otherLine with begin of opposite side
				otherLine, _ = strconv.ParseInt(groups["beginOld"], 10, 64)
			}
			end += begin // end is for real only the number of lines in hunk
			// lof is between begin and end
			if begin <= line && end >= line {
				hunk = append(hunk, lof)
				currentLine = begin
				continue
			}
		} else if len(hunk) > headerLines {
			hunk = append(hunk, lof)
			// Count lines in context
			switch lof[0] {
			case '+':
				if !old {
					currentLine++
				} else {
					otherLine++
				}
			case '-':
				if old {
					currentLine++
				} else {
					otherLine++
				}
			default:
				currentLine++
				otherLine++
			}
		}
	}
	err := scanner.Err()
	if err != nil {
		return "", err
	}

	// No hunk found
	if currentLine == 0 {
		return "", nil
	}
	// headerLines + hunkLine (1) = totalNonCodeLines
	if len(hunk)-headerLines-1 <= numbersOfLine {
		// No need to cut the hunk => return existing hunk
		return strings.Join(hunk, "\n"), nil
	}
	var oldBegin, oldNumOfLines, newBegin, newNumOfLines int64
	if old {
		oldBegin = currentLine
		newBegin = otherLine
	} else {
		oldBegin = otherLine
		newBegin = currentLine
	}
	// headers + hunk header
	newHunk := make([]string, headerLines)
	// transfer existing headers
	copy(newHunk, hunk[:headerLines])
	// transfer last n lines
	newHunk = append(newHunk, hunk[len(hunk)-numbersOfLine-1:]...)
	// calculate newBegin, ... by counting lines
	for i := len(hunk) - 1; i >= len(hunk)-numbersOfLine; i-- {
		switch hunk[i][0] {
		case '+':
			newBegin--
			newNumOfLines++
		case '-':
			oldBegin--
			oldNumOfLines++
		default:
			oldBegin--
			newBegin--
			newNumOfLines++
			oldNumOfLines++
		}
	}
	// construct the new hunk header
	newHunk[headerLines] = fmt.Sprintf("@@ -%d,%d +%d,%d @@",
		oldBegin, oldNumOfLines, newBegin, newNumOfLines)
	return strings.Join(newHunk, "\n"), nil
}
