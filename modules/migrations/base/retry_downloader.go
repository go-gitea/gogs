// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"context"
	"time"
)

var (
	_ Downloader = &RetryDownloader{}
)

// RetryDownloader retry the downloads
type RetryDownloader struct {
	Downloader
	ctx        context.Context
	RetryTimes int // the total execute times
	RetryDelay int // time to delay seconds
}

// NewRetryDownloader creates a retry downloader
func NewRetryDownloader(ctx context.Context, downloader Downloader, retryTimes, retryDelay int) *RetryDownloader {
	return &RetryDownloader{
		Downloader: downloader,
		ctx:        ctx,
		RetryTimes: retryTimes,
		RetryDelay: retryDelay,
	}
}

func (d *RetryDownloader) retry(work func() error) error {
	var (
		times = d.RetryTimes
		err   error
	)
	for ; times > 0; times-- {
		if err = work(); err == nil {
			return nil
		}
		if IsErrNotSupported(err) {
			return err
		}
		select {
		case <-d.ctx.Done():
			return d.ctx.Err()
		case <-time.After(time.Second * time.Duration(d.RetryDelay)):
		}
	}
	return err
}

// SetContext set context
func (d *RetryDownloader) SetContext(ctx context.Context) {
	d.ctx = ctx
	d.Downloader.SetContext(ctx)
}

// GetRepoInfo returns a repository information with retry
func (d *RetryDownloader) GetRepoInfo() (*Repository, error) {
	var (
		repo *Repository
		err  error
	)

	err = d.retry(func() error {
		repo, err = d.Downloader.GetRepoInfo()
		return err
	})

	return repo, err
}

// GetTopics returns a repository's topics with retry
func (d *RetryDownloader) GetTopics() ([]string, error) {
	var (
		topics []string
		err    error
	)

	err = d.retry(func() error {
		topics, err = d.Downloader.GetTopics()
		return err
	})

	return topics, err
}

// GetMilestones returns a repository's milestones with retry
func (d *RetryDownloader) GetMilestones() ([]*Milestone, error) {
	var (
		milestones []*Milestone
		err        error
	)

	err = d.retry(func() error {
		milestones, err = d.Downloader.GetMilestones()
		return err
	})

	return milestones, err
}

// GetReleases returns a repository's releases with retry
func (d *RetryDownloader) GetReleases() ([]*Release, error) {
	var (
		releases []*Release
		err      error
	)

	err = d.retry(func() error {
		releases, err = d.Downloader.GetReleases()
		return err
	})

	return releases, err
}

// GetLabels returns a repository's labels with retry
func (d *RetryDownloader) GetLabels() ([]*Label, error) {
	var (
		labels []*Label
		err    error
	)

	err = d.retry(func() error {
		labels, err = d.Downloader.GetLabels()
		return err
	})

	return labels, err
}

// GetIssues returns a repository's issues with retry
func (d *RetryDownloader) GetIssues(page, perPage int) ([]*Issue, bool, error) {
	var (
		issues []*Issue
		isEnd  bool
		err    error
	)

	err = d.retry(func() error {
		issues, isEnd, err = d.Downloader.GetIssues(page, perPage)
		return err
	})

	return issues, isEnd, err
}

// GetComments returns a repository's comments with retry
func (d *RetryDownloader) GetComments(opts GetCommentOptions) ([]*Comment, bool, error) {
	var (
		comments []*Comment
		isEnd    bool
		err      error
	)

	err = d.retry(func() error {
		comments, isEnd, err = d.Downloader.GetComments(opts)
		return err
	})

	return comments, isEnd, err
}

// GetPullRequests returns a repository's pull requests with retry
func (d *RetryDownloader) GetPullRequests(page, perPage int) ([]*PullRequest, bool, error) {
	var (
		prs   []*PullRequest
		err   error
		isEnd bool
	)

	err = d.retry(func() error {
		prs, isEnd, err = d.Downloader.GetPullRequests(page, perPage)
		return err
	})

	return prs, isEnd, err
}

// GetReviews returns pull requests reviews
func (d *RetryDownloader) GetReviews(pullRequestNumber int64) ([]*Review, error) {
	var (
		reviews []*Review
		err     error
	)

	err = d.retry(func() error {
		reviews, err = d.Downloader.GetReviews(pullRequestNumber)
		return err
	})

	return reviews, err
}
