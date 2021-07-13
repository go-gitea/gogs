// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build gogit

package git

import (
	"context"
	"path"

	"code.gitea.io/gitea/modules/log"

	"github.com/go-git/go-git/v5/plumbing/object"
	cgobject "github.com/go-git/go-git/v5/plumbing/object/commitgraph"
)

// LastCommitCache represents a cache to store last commit
type LastCommitCache struct {
	repoPath    string
	ttl         func() int64
	repo        *Repository
	commitCache map[string]*object.Commit
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
		commitCache: make(map[string]*object.Commit),
		ttl:         ttl,
		cache:       cache,
	}
}

// Get get the last commit information by commit id and entry path
func (c *LastCommitCache) Get(ref, entryPath string) (interface{}, error) {
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
		commit, err := c.repo.GoGitRepo().CommitObject(id)
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

	commitNodeIndex, _ := commit.repo.CommitNodeIndex()

	index, err := commitNodeIndex.Get(commit.ID)
	if err != nil {
		return err
	}

	return c.recursiveCache(ctx, index, &commit.Tree, "", 1)
}

func (c *LastCommitCache) recursiveCache(ctx context.Context, index cgobject.CommitNode, tree *Tree, treePath string, level int) error {
	if level == 0 {
		return nil
	}

	entries, err := tree.ListEntries()
	if err != nil {
		return err
	}

	entryPaths := make([]string, len(entries))
	entryMap := make(map[string]*TreeEntry)
	for i, entry := range entries {
		entryPaths[i] = entry.Name()
		entryMap[entry.Name()] = entry
	}

	commits, err := GetLastCommitForPaths(ctx, index, treePath, entryPaths)
	if err != nil {
		return err
	}

	for entry, cm := range commits {
		if err := c.Put(index.ID().String(), path.Join(treePath, entry), cm.ID().String()); err != nil {
			return err
		}
		if entryMap[entry].IsDir() {
			subTree, err := tree.SubTree(entry)
			if err != nil {
				return err
			}
			if err := c.recursiveCache(ctx, index, subTree, entry, level-1); err != nil {
				return err
			}
		}
	}

	return nil
}
