// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"bufio"
	"context"

	"code.gitea.io/gitea/modules/log"
)

// LastCommitCache represents a cache to store last commit
type LastCommitCache struct {
	repoPath    string
	ttl         func() int64
	repo        *Repository
	commitCache map[string]*Commit
	cache       Cache
}

// NewLastCommitCache creates a new last commit cache for repo
func NewLastCommitCache(repoPath string, gitRepo *Repository, ttl func() int64, cache Cache) *LastCommitCache {
	if cache == nil {
		return nil
	}
	return &LastCommitCache{
		repoPath:    repoPath,
		repo:        gitRepo,
		commitCache: make(map[string]*Commit),
		ttl:         ttl,
		cache:       cache,
	}
}

// Get get the last commit information by commit id and entry path
func (c *LastCommitCache) Get(ref, entryPath string, wr WriteCloserError, rd *bufio.Reader) (interface{}, error) {
	v := c.cache.Get(c.getCacheKey(c.repoPath, ref, entryPath))
	if vs, ok := v.(string); ok {
		log.Debug("LastCommitCache hit level 1: [%s:%s:%s]", ref, entryPath, vs)
		if commit, ok := c.commitCache[vs]; ok {
			log.Debug("LastCommitCache hit level 2: [%s:%s:%s]", ref, entryPath, vs)
			return commit, nil
		}
		id, err := c.repo.ConvertToSHA1(vs)
		if err != nil {
			return nil, err
		}
		if _, err := wr.Write([]byte(vs + "\n")); err != nil {
			return nil, err
		}
		commit, err := c.repo.getCommitFromBatchReader(rd, id)
		if err != nil {
			return nil, err
		}
		c.commitCache[vs] = commit
		return commit, nil
	}
	return nil, nil
}

// CacheCommit will cache the commit from the gitRepository
func (c *LastCommitCache) CacheCommit(ctx context.Context, commit *Commit) error {
	return c.recursiveCache(ctx, commit, &commit.Tree, "", 1)
}

func (c *LastCommitCache) recursiveCache(ctx context.Context, commit *Commit, tree *Tree, treePath string, level int) error {
	if level == 0 {
		return nil
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return err
	}

	entryPaths := make([]string, len(entries))
	for i, entry := range entries {
		entryPaths[i] = entry.Name()
	}

	_, err = WalkGitLog(ctx, c, commit.repo, commit, treePath, entryPaths...)
	if err != nil {
		return err
	}

	for _, treeEntry := range entries {
		// entryMap won't contain "" therefore skip this.
		if treeEntry.IsDir() {
			subTree, err := tree.SubTree(treeEntry.Name())
			if err != nil {
				return err
			}
			if err := c.recursiveCache(ctx, commit, subTree, treeEntry.Name(), level-1); err != nil {
				return err
			}
		}
	}

	return nil
}
