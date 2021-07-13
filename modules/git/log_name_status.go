// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
)

// LogNameStatusRepo opens git log --raw in the provided repo and returns a stdin pipe, a stdout reader and cancel function
func LogNameStatusRepo(repository, head, treepath string, paths ...string) (*bufio.Reader, func()) {
	// We often want to feed the commits in order into cat-file --batch, followed by their trees and sub trees as necessary.
	// so let's create a batch stdin and stdout
	stdoutReader, stdoutWriter := nio.Pipe(buffer.New(32 * 1024))
	cancel := func() {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
	}

	args := make([]string, 0, 8+len(paths))
	args = append(args, "log", "--name-status", "-c", "--format=commit%x00%H %P%x00", "--parents", "--no-renames", "-t", "-z", head, "--")
	if len(paths) < 70 {
		if treepath != "" {
			args = append(args, treepath)
			for _, pth := range paths {
				if pth != "" {
					args = append(args, path.Join(treepath, pth))
				}
			}
		} else {
			for _, pth := range paths {
				if pth != "" {
					args = append(args, pth)
				}
			}
		}
	} else if treepath != "" {
		args = append(args, treepath)
	}

	go func() {
		stderr := strings.Builder{}
		err := NewCommand(args...).RunInDirFullPipeline(repository, stdoutWriter, &stderr, nil)
		if err != nil {
			_ = stdoutWriter.CloseWithError(ConcatenateError(err, (&stderr).String()))
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	// For simplicities sake we'll us a buffered reader to read from the cat-file --batch
	bufReader := bufio.NewReaderSize(stdoutReader, 32*1024)

	return bufReader, cancel
}

// LogNameStatusRepoParser parses a git log raw output from LogRawRepo
type LogNameStatusRepoParser struct {
	treepath string
	paths    []string
	next     []byte
	buffull  bool
	rd       *bufio.Reader
	cancel   func()
}

// NewLogNameStatusRepoParser returns a new parser for a git log raw output
func NewLogNameStatusRepoParser(repository, head, treepath string, paths ...string) *LogNameStatusRepoParser {
	rd, cancel := LogNameStatusRepo(repository, head, treepath, paths...)
	return &LogNameStatusRepoParser{
		treepath: treepath,
		paths:    paths,
		rd:       rd,
		cancel:   cancel,
	}
}

// LogNameStatusCommitData represents a commit artefact from git log raw
type LogNameStatusCommitData struct {
	CommitID  string
	ParentIDs []string
	Paths     []bool
}

// Next returns the next LogStatusCommitData
func (g *LogNameStatusRepoParser) Next(treepath string, paths2ids map[string]int, changed []bool, maxpathlen int) (*LogNameStatusCommitData, error) {
	var err error
	if g.next == nil || len(g.next) == 0 {
		g.buffull = false
		g.next, err = g.rd.ReadSlice('\x00')
		if err != nil {
			if err == bufio.ErrBufferFull {
				g.buffull = true
			} else if err == io.EOF {
				return nil, nil
			} else {
				return nil, err
			}
		}
	}

	ret := LogNameStatusCommitData{}
	if bytes.Equal(g.next, []byte("commit\000")) {
		g.next, err = g.rd.ReadSlice('\x00')
		if err != nil {
			if err == bufio.ErrBufferFull {
				g.buffull = true
			} else if err == io.EOF {
				return nil, nil
			} else {
				return nil, err
			}
		}
	}

	// Our "line" must look like: <commitid> SP (<parent> SP) * NUL
	ret.CommitID = string(g.next[0:40])
	parents := string(g.next[41:])
	if g.buffull {
		more, err := g.rd.ReadString('\x00')
		if err != nil {
			return nil, err
		}
		parents += more
	}
	parents = parents[:len(parents)-1]
	ret.ParentIDs = strings.Split(parents, " ")

	// now read the next "line"
	g.buffull = false
	g.next, err = g.rd.ReadSlice('\x00')
	if err != nil {
		if err == bufio.ErrBufferFull {
			g.buffull = true
		} else if err != io.EOF {
			return nil, err
		}
	}

	if err == io.EOF || !(g.next[0] == '\n' || g.next[0] == '\000') {
		return &ret, nil
	}

	// Ok we have some changes.
	// This line will look like: NL <fname> NUL
	//
	// Subsequent lines will not have the NL - so drop it here - g.bufffull must also be false at this point too.
	if g.next[0] == '\n' {
		g.next = g.next[1:]
	} else {
		g.buffull = false
		g.next, err = g.rd.ReadSlice('\x00')
		if err != nil {
			if err == bufio.ErrBufferFull {
				g.buffull = true
			} else if err != io.EOF {
				return nil, err
			}
		}
		if g.next[0] == '\x00' {
			g.buffull = false
			g.next, err = g.rd.ReadSlice('\x00')
			if err != nil {
				if err == bufio.ErrBufferFull {
					g.buffull = true
				} else if err != io.EOF {
					return nil, err
				}
			}
		}
	}

	fnameBuf := make([]byte, 4096)

diffloop:
	for {
		if err == io.EOF || bytes.Equal(g.next, []byte("commit\000")) {
			return &ret, nil
		}
		g.next, err = g.rd.ReadSlice('\x00')
		if err != nil {
			if err == bufio.ErrBufferFull {
				g.buffull = true
			} else if err == io.EOF {
				return &ret, nil
			} else {
				return nil, err
			}
		}
		copy(fnameBuf, g.next)
		if len(fnameBuf) < len(g.next) {
			fnameBuf = append(fnameBuf, g.next[len(fnameBuf):]...)
		} else {
			fnameBuf = fnameBuf[:len(g.next)]
		}
		if err != nil {
			if err != bufio.ErrBufferFull {
				return nil, err
			}
			more, err := g.rd.ReadBytes('\x00')
			if err != nil {
				return nil, err
			}
			fnameBuf = append(fnameBuf, more...)
		}

		// read the next line
		g.buffull = false
		g.next, err = g.rd.ReadSlice('\x00')
		if err != nil {
			if err == bufio.ErrBufferFull {
				g.buffull = true
			} else if err != io.EOF {
				return nil, err
			}
		}

		if treepath != "" {
			if !bytes.HasPrefix(fnameBuf, []byte(treepath)) {
				fnameBuf = fnameBuf[:cap(fnameBuf)]
				continue diffloop
			}
		}
		fnameBuf = fnameBuf[len(treepath) : len(fnameBuf)-1]
		if len(fnameBuf) > maxpathlen {
			fnameBuf = fnameBuf[:cap(fnameBuf)]
			continue diffloop
		}
		if len(fnameBuf) > 0 {
			if len(treepath) > 0 {
				if fnameBuf[0] != '/' || bytes.IndexByte(fnameBuf[1:], '/') >= 0 {
					fnameBuf = fnameBuf[:cap(fnameBuf)]
					continue diffloop
				}
				fnameBuf = fnameBuf[1:]
			} else if bytes.IndexByte(fnameBuf, '/') >= 0 {
				fnameBuf = fnameBuf[:cap(fnameBuf)]
				continue diffloop
			}
		}

		idx, ok := paths2ids[string(fnameBuf)]
		if !ok {
			fnameBuf = fnameBuf[:cap(fnameBuf)]
			continue diffloop
		}
		if ret.Paths == nil {
			ret.Paths = changed
		}
		changed[idx] = true
	}
}

// Close closes the parser
func (g *LogNameStatusRepoParser) Close() {
	g.cancel()
}

// WalkGitLog walks the git log --name-status for the head commit in the provided treepath and files
func WalkGitLog(ctx context.Context, repo *Repository, head *Commit, treepath string, paths ...string) (map[string]string, error) {
	tree, err := head.SubTree(treepath)
	if err != nil {
		return nil, err
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		paths = make([]string, 0, len(entries)+1)
		paths = append(paths, "")
		for _, entry := range entries {
			paths = append(paths, entry.Name())
		}
	} else {
		sort.Strings(paths)
		if paths[0] != "" {
			paths = append([]string{""}, paths...)
		}
		// remove duplicates
		for i := len(paths) - 1; i > 0; i-- {
			if paths[i] == paths[i-1] {
				paths = append(paths[:i-1], paths[i:]...)
			}
		}
	}

	path2idx := map[string]int{}
	maxpathlen := len(treepath)

	for i := range paths {
		path2idx[paths[i]] = i
		pthlen := len(paths[i]) + len(treepath) + 1
		if pthlen > maxpathlen {
			maxpathlen = pthlen
		}
	}

	g := NewLogNameStatusRepoParser(repo.Path, head.ID.String(), treepath, paths...)
	defer g.Close()

	results := make([]string, len(paths))
	remaining := len(paths)
	nextRestart := (len(paths) * 3) / 4
	if nextRestart > 70 {
		nextRestart = 70
	}
	lastEmptyParent := head.ID.String()
	commitSinceLastEmptyParent := uint64(0)
	commitSinceNextRestart := uint64(0)
	parentRemaining := map[string]bool{}

	changed := make([]bool, len(paths))

heaploop:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		current, err := g.Next(treepath, path2idx, changed, maxpathlen)
		if err != nil {
			g.Close()
			return nil, err
		}
		if current == nil {
			break heaploop
		}
		delete(parentRemaining, current.CommitID)
		if current.Paths != nil {
			for i, found := range current.Paths {
				if !found {
					continue
				}
				changed[i] = false
				if results[i] == "" {
					results[i] = current.CommitID
					delete(path2idx, paths[i])
					remaining--
					if results[0] == "" {
						results[0] = current.CommitID
						delete(path2idx, "")
						remaining--
					}
				}
			}
		}

		if remaining <= 0 {
			break heaploop
		}
		commitSinceLastEmptyParent++
		if len(parentRemaining) == 0 {
			lastEmptyParent = current.CommitID
			commitSinceLastEmptyParent = 0
		}
		if remaining <= nextRestart {
			commitSinceNextRestart++
			if 4*commitSinceNextRestart > 3*commitSinceLastEmptyParent {
				g.Close()
				remainingPaths := make([]string, 0, len(paths))
				for i, pth := range paths {
					if results[i] == "" {
						remainingPaths = append(remainingPaths, pth)
					}
				}
				g = NewLogNameStatusRepoParser(repo.Path, lastEmptyParent, treepath, remainingPaths...)
				parentRemaining = map[string]bool{}
				nextRestart = (remaining * 3) / 4
				continue heaploop
			}
		}
		for _, parent := range current.ParentIDs {
			parentRemaining[parent] = true
		}
	}
	g.Close()

	resultsMap := map[string]string{}
	for i, pth := range paths {
		resultsMap[pth] = results[i]
	}

	return resultsMap, nil
}
