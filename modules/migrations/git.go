// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"

	"code.gitea.io/gitea/modules/migrations/base"
)

var _ base.Downloader = &PlainGitDownloader{}

// PlainGitDownloader implements a Downloader interface to clone git from a http/https URL
type PlainGitDownloader struct {
	base.NullDownloader
	ownerName string
	repoName  string
	remoteURL string
}

// NewPlainGitDownloader creates a git Downloader
func NewPlainGitDownloader(ownerName, repoName, remoteURL string) *PlainGitDownloader {
	return &PlainGitDownloader{
		ownerName: ownerName,
		repoName:  repoName,
		remoteURL: remoteURL,
	}
}

// SetContext set context
func (g *PlainGitDownloader) SetContext(ctx context.Context) {
}

// GetRepoInfo returns a repository information
func (g *PlainGitDownloader) GetRepoInfo() (*base.Repository, error) {
	// convert github repo to stand Repo
	return &base.Repository{
		Owner:    g.ownerName,
		Name:     g.repoName,
		CloneURL: g.remoteURL,
	}, nil
}

// GetTopics return empty string slice
func (g PlainGitDownloader) GetTopics() ([]string, error) {
	return []string{}, nil
}
