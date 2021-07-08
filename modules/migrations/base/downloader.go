// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"context"

	"code.gitea.io/gitea/modules/structs"
)

// GetCommentOptions represents an options for get comment
type GetCommentOptions struct {
	IssueNumber int64
	Page        int
	PageSize    int
}

// Downloader downloads the site repo information
type Downloader interface {
	SetContext(context.Context)
	GetRepoInfo() (*Repository, error)
	GetTopics() ([]string, error)
	GetMilestones() ([]*Milestone, error)
	GetReleases() ([]*Release, error)
	GetLabels() ([]*Label, error)
	GetIssues(page, perPage int) ([]*Issue, bool, error)
	GetComments(opts GetCommentOptions) ([]*Comment, bool, error)
	SupportGetRepoComments() bool
	GetPullRequests(page, perPage int) ([]*PullRequest, bool, error)
	GetReviews(pullRequestNumber int64) ([]*Review, error)
	FormatCloneURL(opts MigrateOptions, remoteAddr string) (string, error)
}

// DownloaderFactory defines an interface to match a downloader implementation and create a downloader
type DownloaderFactory interface {
	New(ctx context.Context, opts MigrateOptions) (Downloader, error)
	GitServiceType() structs.GitServiceType
}
