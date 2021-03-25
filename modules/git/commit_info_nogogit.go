// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"path"
	"sort"
	"strings"
)

// GetCommitsInfo gets information of all commits that are corresponding to these entries
func (tes Entries) GetCommitsInfo(commit *Commit, treePath string, cache *LastCommitCache) ([]CommitInfo, *Commit, error) {
	entryPaths := make([]string, len(tes)+1)
	// Get the commit for the treePath itself
	entryPaths[0] = ""
	for i, entry := range tes {
		entryPaths[i+1] = entry.Name()
	}

	var err error

	var revs map[string]*Commit
	if cache != nil {
		var unHitPaths []string
		revs, unHitPaths, err = getLastCommitForPathsByCache(commit.ID.String(), treePath, entryPaths, cache)
		if err != nil {
			return nil, nil, err
		}
		if len(unHitPaths) > 0 {
			sort.Strings(unHitPaths)
			commits, err := GetLastCommitForPaths(commit, treePath, unHitPaths)
			if err != nil {
				return nil, nil, err
			}

			for i, found := range commits {
				if err := cache.Put(commit.ID.String(), path.Join(treePath, unHitPaths[i]), found.ID.String()); err != nil {
					return nil, nil, err
				}
				revs[unHitPaths[i]] = found
			}
		}
	} else {
		sort.Strings(entryPaths)
		revs = map[string]*Commit{}
		var foundCommits []*Commit
		foundCommits, err = GetLastCommitForPaths(commit, treePath, entryPaths)
		for i, found := range foundCommits {
			revs[entryPaths[i]] = found
		}
	}
	if err != nil {
		return nil, nil, err
	}

	commitsInfo := make([]CommitInfo, len(tes))
	for i, entry := range tes {
		commitsInfo[i] = CommitInfo{
			Entry: entry,
		}
		if entryCommit, ok := revs[entry.Name()]; ok {
			commitsInfo[i].Commit = entryCommit
			if entry.IsSubModule() {
				subModuleURL := ""
				var fullPath string
				if len(treePath) > 0 {
					fullPath = treePath + "/" + entry.Name()
				} else {
					fullPath = entry.Name()
				}
				if subModule, err := commit.GetSubModule(fullPath); err != nil {
					return nil, nil, err
				} else if subModule != nil {
					subModuleURL = subModule.URL
				}
				subModuleFile := NewSubModuleFile(entryCommit, subModuleURL, entry.ID.String())
				commitsInfo[i].SubModuleFile = subModuleFile
			}
		}
	}

	// Retrieve the commit for the treePath itself (see above). We basically
	// get it for free during the tree traversal and it's used for listing
	// pages to display information about newest commit for a given path.
	var treeCommit *Commit
	var ok bool
	if treePath == "" {
		treeCommit = commit
	} else if treeCommit, ok = revs[""]; ok {
		treeCommit.repo = commit.repo
	}
	return commitsInfo, treeCommit, nil
}

func getLastCommitForPathsByCache(commitID, treePath string, paths []string, cache *LastCommitCache) (map[string]*Commit, []string, error) {
	var unHitEntryPaths []string
	results := make(map[string]*Commit)
	for _, p := range paths {
		lastCommit, err := cache.Get(commitID, path.Join(treePath, p))
		if err != nil {
			return nil, nil, err
		}
		if lastCommit != nil {
			results[p] = lastCommit.(*Commit)
			continue
		}

		unHitEntryPaths = append(unHitEntryPaths, p)
	}

	return results, unHitEntryPaths, nil
}

// GetLastCommitForPaths returns last commit information
func GetLastCommitForPaths(commit *Commit, treePath string, paths []string) ([]*Commit, error) {
	// We read backwards from the commit to obtain all of the commits

	// We'll do this by using rev-list to provide us with parent commits in order
	revListReader, revListWriter := io.Pipe()
	defer func() {
		_ = revListWriter.Close()
		_ = revListReader.Close()
	}()

	go func() {
		stderr := strings.Builder{}
		err := NewCommand("rev-list", "--format=%T", commit.ID.String()).RunInDirPipeline(commit.repo.Path, revListWriter, &stderr)
		if err != nil {
			_ = revListWriter.CloseWithError(ConcatenateError(err, (&stderr).String()))
		} else {
			_ = revListWriter.Close()
		}
	}()

	batchStdinWriter, batchReader, cancel := CatFileBatch(commit.repo.Path)
	defer cancel()

	mapsize := 4096
	if len(paths) > mapsize {
		mapsize = len(paths)
	}

	path2idx := make(map[string]int, mapsize)
	for i, path := range paths {
		path2idx[path] = i
	}

	fnameBuf := make([]byte, 4096)
	modeBuf := make([]byte, 40)

	allShaBuf := make([]byte, (len(paths)+1)*20)
	shaBuf := make([]byte, 20)
	tmpTreeID := make([]byte, 40)

	// commits is the returnable commits matching the paths provided
	commits := make([]string, len(paths))
	// ids are the blob/tree ids for the paths
	ids := make([][]byte, len(paths))

	// We'll use a scanner for the revList because it's simpler than a bufio.Reader
	scan := bufio.NewScanner(revListReader)
revListLoop:
	for scan.Scan() {
		// Get the next parent commit ID
		commitID := scan.Text()
		if !scan.Scan() {
			break revListLoop
		}
		commitID = commitID[7:]
		rootTreeID := scan.Text()

		// push the tree to the cat-file --batch process
		_, err := batchStdinWriter.Write([]byte(rootTreeID + "\n"))
		if err != nil {
			return nil, err
		}

		currentPath := ""

		// OK if the target tree path is "" and the "" is in the paths just set this now
		if treePath == "" && paths[0] == "" {
			// If this is the first time we see this set the id appropriate for this paths to this tree and set the last commit to curCommit
			if len(ids[0]) == 0 {
				ids[0] = []byte(rootTreeID)
				commits[0] = string(commitID)
			} else if bytes.Equal(ids[0], []byte(rootTreeID)) {
				commits[0] = string(commitID)
			}
		}

	treeReadingLoop:
		for {
			_, _, size, err := ReadBatchLine(batchReader)
			if err != nil {
				return nil, err
			}

			// Handle trees

			// n is counter for file position in the tree file
			var n int64

			// Two options: currentPath is the targetTreepath
			if treePath == currentPath {
				// We are in the right directory
				// Parse each tree line in turn. (don't care about mode here.)
				for n < size {
					fname, sha, count, err := ParseTreeLineSkipMode(batchReader, fnameBuf, shaBuf)
					shaBuf = sha
					if err != nil {
						return nil, err
					}
					n += int64(count)
					idx, ok := path2idx[string(fname)]
					if ok {
						// Now if this is the first time round set the initial Blob(ish) SHA ID and the commit
						if len(ids[idx]) == 0 {
							copy(allShaBuf[20*(idx+1):20*(idx+2)], shaBuf)
							ids[idx] = allShaBuf[20*(idx+1) : 20*(idx+2)]
							commits[idx] = string(commitID)
						} else if bytes.Equal(ids[idx], shaBuf) {
							commits[idx] = string(commitID)
						}
					}
					// FIXME: is there any order to the way strings are emitted from cat-file?
					// if there is - then we could skip once we've passed all of our data
				}
				break treeReadingLoop
			}

			var treeID []byte

			// We're in the wrong directory
			// Find target directory in this directory
			idx := len(currentPath)
			if idx > 0 {
				idx++
			}
			target := strings.SplitN(treePath[idx:], "/", 2)[0]

			for n < size {
				// Read each tree entry in turn
				mode, fname, sha, count, err := ParseTreeLine(batchReader, modeBuf, fnameBuf, shaBuf)
				if err != nil {
					return nil, err
				}
				n += int64(count)

				// if we have found the target directory
				if bytes.Equal(fname, []byte(target)) && bytes.Equal(mode, []byte("40000")) {
					copy(tmpTreeID, sha)
					treeID = tmpTreeID
					break
				}
			}

			if n < size {
				// Discard any remaining entries in the current tree
				discard := size - n
				for discard > math.MaxInt32 {
					_, err := batchReader.Discard(math.MaxInt32)
					if err != nil {
						return nil, err
					}
					discard -= math.MaxInt32
				}
				_, err := batchReader.Discard(int(discard))
				if err != nil {
					return nil, err
				}
			}

			// if we haven't found a treeID for the target directory our search is over
			if len(treeID) == 0 {
				break treeReadingLoop
			}

			// add the target to the current path
			if idx > 0 {
				currentPath += "/"
			}
			currentPath += target

			// if we've now found the current path check its sha id and commit status
			if treePath == currentPath && paths[0] == "" {
				if len(ids[0]) == 0 {
					copy(allShaBuf[0:20], treeID)
					ids[0] = allShaBuf[0:20]
					commits[0] = string(commitID)
				} else if bytes.Equal(ids[0], treeID) {
					commits[0] = string(commitID)
				}
			}
			treeID = to40ByteSHA(treeID)
			_, err = batchStdinWriter.Write(treeID)
			if err != nil {
				return nil, err
			}
			_, err = batchStdinWriter.Write([]byte("\n"))
			if err != nil {
				return nil, err
			}
		}
	}

	commitsMap := make(map[string]*Commit, len(commits))
	commitsMap[commit.ID.String()] = commit

	commitCommits := make([]*Commit, len(commits))
	for i, commitID := range commits {
		c, ok := commitsMap[commitID]
		if ok {
			commitCommits[i] = c
			continue
		}

		if len(commitID) == 0 {
			continue
		}

		_, err := batchStdinWriter.Write([]byte(commitID + "\n"))
		if err != nil {
			return nil, err
		}
		_, typ, size, err := ReadBatchLine(batchReader)
		if err != nil {
			return nil, err
		}
		if typ != "commit" {
			return nil, fmt.Errorf("unexpected type: %s for commit id: %s", typ, commitID)
		}
		c, err = CommitFromReader(commit.repo, MustIDFromString(string(commitID)), io.LimitReader(batchReader, int64(size)))
		if err != nil {
			return nil, err
		}
		commitCommits[i] = c
	}

	return commitCommits, scan.Err()
}
