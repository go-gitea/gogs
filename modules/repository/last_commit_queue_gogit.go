// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build gogit

package repository

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sync"

	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
)

var lock = sync.Mutex{}
var table = map[CommitCacheRequest]bool{}
var lastCommitQueue queue.UniqueQueue

// CommitCacheRequest represents a cache request
type CommitCacheRequest struct {
	Repo      string
	CommitID  string
	TreePath  string
	Recursive bool
}

// Do runs the cache request uniquely ensuring that only one cache request is running for this request triple
func (req *CommitCacheRequest) Do() error {
	ctx, cancel, _ := process.GetManager().AddContext(graceful.GetManager().HammerContext(), fmt.Sprintf("Cache: %s:%s:%s:%t", req.Repo, req.CommitID, req.TreePath, req.Recursive))
	defer cancel()

	recursive := req.Recursive
	req.Recursive = false

	repo, err := git.OpenRepository(filepath.Join(setting.RepoRootPath, req.Repo+".git"))
	if err != nil {
		return err
	}
	commit, err := repo.GetCommit(req.CommitID)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil
		}
		return err
	}

	lccache := git.NewLastCommitCache(req.Repo, repo, setting.LastCommitCacheTTLSeconds, cache.GetCache())

	directories := []string{req.TreePath}
	for len(directories) > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req.TreePath = directories[len(directories)-1]
		next, err := req.doTree(ctx, repo, commit, recursive, lccache)
		if err != nil {
			return err
		}
		directories = append(next, directories[:len(directories)-1]...)
	}
	return nil
}

func (req *CommitCacheRequest) doTree(ctx context.Context, repo *git.Repository, commit *git.Commit, recursive bool, lccache *git.LastCommitCache) ([]string, error) {
	tree, err := commit.Tree.SubTree(req.TreePath)
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries, err := tree.ListEntries()
	if err != nil {
		if git.IsErrNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	directories := make([]string, 0, len(entries))

	commitNodeIndex, commitGraphFile := repo.CommitNodeIndex()
	if commitGraphFile != nil {
		defer commitGraphFile.Close()
	}

	commitNode, err := commitNodeIndex.Get(commit.ID)
	if err != nil {
		return nil, err
	}

	lock.Lock()
	if has := table[*req]; has {
		lock.Unlock()
		if recursive {
			for _, entry := range entries {
				if entry.IsDir() {
					directories = append(directories, path.Join(req.TreePath, entry.Name()))
				}
			}
		}
		return directories, nil
	}
	table[*req] = true
	lock.Unlock()
	defer func() {
		lock.Lock()
		delete(table, *req)
		lock.Unlock()
	}()

	entryPaths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if recursive && entry.IsDir() {
			directories = append(directories, path.Join(req.TreePath, entry.Name()))
		}
		_, ok := lccache.GetCachedCommitID(req.CommitID, path.Join(req.TreePath, entry.Name()))
		if !ok {
			entryPaths = append(entryPaths, entry.Name())
		}
	}

	if len(entryPaths) == 0 {
		return directories, nil
	}

	commits, err := git.GetLastCommitForPaths(ctx, commitNode, req.TreePath, entryPaths)
	if err != nil {
		return nil, err
	}

	for entryPath, entryCommit := range commits {
		if err := lccache.Put(commit.ID.String(), path.Join(req.TreePath, entryPath), entryCommit.ID().String()); err != nil {
			return nil, err
		}
	}

	return directories, nil
}

func handle(data ...queue.Data) {
	for _, datum := range data {
		req := datum.(*CommitCacheRequest)
		if err := req.Do(); err != nil {
			log.Error("Unable to process commit cache request for %s:%s:%s:%t: %v", req.Repo, req.CommitID, req.TreePath, req.Recursive, err)
		}
	}
}

// Init initialises the queue
func Init() error {
	lastCommitQueue = queue.CreateUniqueQueue("last_commit_queue", handle, &CommitCacheRequest{}).(queue.UniqueQueue)

	return nil
}

// UpdateCache queues the the request
func UpdateCache(repo, commitID, treePath string, recursive bool) error {
	return lastCommitQueue.Push(&CommitCacheRequest{
		Repo:      repo,
		CommitID:  commitID,
		TreePath:  treePath,
		Recursive: recursive,
	})
}
