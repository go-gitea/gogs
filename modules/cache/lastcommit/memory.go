// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lastcommit

import (
	"fmt"
	"sync"

	"code.gitea.io/git"
)

var (
	_ git.LastCommitCache = &MemoryCache{}
)

// MemoryCache implements git.LastCommitCache interface to save the last commits on memory
type MemoryCache struct {
	commits sync.Map
}

func getKey(repoPath, ref, entryPath string) string {
	return fmt.Sprintf("%s:%s:%s", repoPath, ref, entryPath)
}

// Get implements git.LastCommitCache
func (c *MemoryCache) Get(repoPath, ref, entryPath string) (*git.Commit, error) {
	v, ok := c.commits.Load(getKey(repoPath, ref, entryPath))
	if ok {
		return v.(*git.Commit), nil
	}
	return nil, nil
}

// Put implements git.LastCommitCache
func (c *MemoryCache) Put(repoPath, ref, entryPath string, commit *git.Commit) error {
	c.commits.Store(getKey(repoPath, ref, entryPath), commit)
	return nil
}
