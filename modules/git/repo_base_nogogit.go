// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"bufio"
	"context"
	"errors"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
)

// Repository represents a Git repository.
type Repository struct {
	Path string

	tagCache *ObjectCache

	gpgSettings *GPGSettings

	batchCancel context.CancelFunc
	batchReader *bufio.Reader
	batchWriter WriteCloserError

	checkCancel context.CancelFunc
	checkReader *bufio.Reader
	checkWriter WriteCloserError
}

// OpenRepository opens the repository at the given path.
func OpenRepository(repoPath string) (*Repository, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	} else if !isDir(repoPath) {
		return nil, errors.New("no such file or directory")
	}

	repo := &Repository{
		Path:     repoPath,
		tagCache: newObjectCache(),
	}

	repo.batchWriter, repo.batchReader, repo.batchCancel = CatFileBatch(repoPath)
	repo.checkWriter, repo.checkReader, repo.checkCancel = CatFileBatchCheck(repo.Path)

	return repo, nil
}

// CatFileBatch obtains a CatFileBatch for this repository
func (repo *Repository) CatFileBatch() (WriteCloserError, *bufio.Reader, func()) {
	if repo.batchCancel == nil || repo.batchReader.Buffered() > 0 {
		log.Debug("Opening temporary cat file batch for: %s", repo.Path)
		return CatFileBatch(repo.Path)
	}
	return repo.batchWriter, repo.batchReader, func() {}
}

// CatFileBatchCheck obtains a CatFileBatchCheck for this repository
func (repo *Repository) CatFileBatchCheck() (WriteCloserError, *bufio.Reader, func()) {
	if repo.checkCancel == nil || repo.checkReader.Buffered() > 0 {
		log.Debug("Opening temporary cat file batch-check: %s", repo.Path)
		return CatFileBatchCheck(repo.Path)
	}
	return repo.checkWriter, repo.checkReader, func() {}
}

// Close this repository, in particular close the underlying gogitStorage if this is not nil
func (repo *Repository) Close() {
	if repo == nil {
		return
	}
	if repo.batchCancel != nil {
		repo.batchCancel()
		repo.batchReader = nil
		repo.batchWriter = nil
		repo.batchCancel = nil
	}
	if repo.checkCancel != nil {
		repo.checkCancel()
		repo.checkCancel = nil
		repo.checkReader = nil
		repo.checkWriter = nil
	}
}
