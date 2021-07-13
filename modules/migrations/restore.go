// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"code.gitea.io/gitea/modules/migrations/base"

	"gopkg.in/yaml.v2"
)

// RepositoryRestorer implements an Downloader from the local directory
type RepositoryRestorer struct {
	base.NullDownloader
	ctx       context.Context
	baseDir   string
	repoOwner string
	repoName  string
}

// NewRepositoryRestorer creates a repository restorer which could restore repository from a dumped folder
func NewRepositoryRestorer(ctx context.Context, baseDir string, owner, repoName string) (*RepositoryRestorer, error) {
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, err
	}
	return &RepositoryRestorer{
		ctx:       ctx,
		baseDir:   baseDir,
		repoOwner: owner,
		repoName:  repoName,
	}, nil
}

func (r *RepositoryRestorer) commentDir() string {
	return filepath.Join(r.baseDir, "comments")
}

func (r *RepositoryRestorer) reviewDir() string {
	return filepath.Join(r.baseDir, "reviews")
}

// SetContext set context
func (r *RepositoryRestorer) SetContext(ctx context.Context) {
	r.ctx = ctx
}

func (r *RepositoryRestorer) getRepoOptions() (map[string]string, error) {
	p := filepath.Join(r.baseDir, "repo.yml")
	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	var opts = make(map[string]string)
	err = yaml.Unmarshal(bs, &opts)
	if err != nil {
		return nil, err
	}
	return opts, nil
}

// GetRepoInfo returns a repository information
func (r *RepositoryRestorer) GetRepoInfo() (*base.Repository, error) {
	opts, err := r.getRepoOptions()
	if err != nil {
		return nil, err
	}

	isPrivate, _ := strconv.ParseBool(opts["is_private"])

	return &base.Repository{
		Owner:         r.repoOwner,
		Name:          r.repoName,
		IsPrivate:     isPrivate,
		Description:   opts["description"],
		OriginalURL:   opts["original_url"],
		CloneURL:      filepath.Join(r.baseDir, "git"),
		DefaultBranch: opts["default_branch"],
	}, nil
}

// GetTopics return github topics
func (r *RepositoryRestorer) GetTopics() ([]string, error) {
	p := filepath.Join(r.baseDir, "topic.yml")

	var topics = struct {
		Topics []string `yaml:"topics"`
	}{}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &topics)
	if err != nil {
		return nil, err
	}
	return topics.Topics, nil
}

// GetMilestones returns milestones
func (r *RepositoryRestorer) GetMilestones() ([]*base.Milestone, error) {
	var milestones = make([]*base.Milestone, 0, 10)
	p := filepath.Join(r.baseDir, "milestone.yml")
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &milestones)
	if err != nil {
		return nil, err
	}
	return milestones, nil
}

// GetReleases returns releases
func (r *RepositoryRestorer) GetReleases() ([]*base.Release, error) {
	var releases = make([]*base.Release, 0, 10)
	p := filepath.Join(r.baseDir, "release.yml")
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &releases)
	if err != nil {
		return nil, err
	}
	for _, rel := range releases {
		for _, asset := range rel.Assets {
			if asset.DownloadURL != nil {
				*asset.DownloadURL = "file://" + filepath.Join(r.baseDir, *asset.DownloadURL)
			}
		}
	}
	return releases, nil
}

// GetLabels returns labels
func (r *RepositoryRestorer) GetLabels() ([]*base.Label, error) {
	var labels = make([]*base.Label, 0, 10)
	p := filepath.Join(r.baseDir, "label.yml")
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &labels)
	if err != nil {
		return nil, err
	}
	return labels, nil
}

// GetIssues returns issues according start and limit
func (r *RepositoryRestorer) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	var issues = make([]*base.Issue, 0, 10)
	p := filepath.Join(r.baseDir, "issue.yml")
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true, nil
		}
		return nil, false, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, false, err
	}

	err = yaml.Unmarshal(bs, &issues)
	if err != nil {
		return nil, false, err
	}
	return issues, true, nil
}

// GetComments returns comments according issueNumber
func (r *RepositoryRestorer) GetComments(opts base.GetCommentOptions) ([]*base.Comment, bool, error) {
	var comments = make([]*base.Comment, 0, 10)
	p := filepath.Join(r.commentDir(), fmt.Sprintf("%d.yml", opts.IssueNumber))
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, false, err
	}

	err = yaml.Unmarshal(bs, &comments)
	if err != nil {
		return nil, false, err
	}
	return comments, false, nil
}

// GetPullRequests returns pull requests according page and perPage
func (r *RepositoryRestorer) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	var pulls = make([]*base.PullRequest, 0, 10)
	p := filepath.Join(r.baseDir, "pull_request.yml")
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true, nil
		}
		return nil, false, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, false, err
	}

	err = yaml.Unmarshal(bs, &pulls)
	if err != nil {
		return nil, false, err
	}
	for _, pr := range pulls {
		pr.PatchURL = "file://" + filepath.Join(r.baseDir, pr.PatchURL)
	}
	return pulls, true, nil
}

// GetReviews returns pull requests review
func (r *RepositoryRestorer) GetReviews(pullRequestNumber int64) ([]*base.Review, error) {
	var reviews = make([]*base.Review, 0, 10)
	p := filepath.Join(r.reviewDir(), fmt.Sprintf("%d.yml", pullRequestNumber))
	_, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	bs, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(bs, &reviews)
	if err != nil {
		return nil, err
	}
	return reviews, nil
}
