// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import (
	"fmt"
	"time"
)

// PullRequest defines a standard pull request information
type PullRequest struct {
	Number         int64
	Title          string
	PosterName     string `yaml:"poster_name"`
	PosterID       int64  `yaml:"poster_id"`
	PosterEmail    string `yaml:"poster_email"`
	Content        string
	Milestone      string
	State          string
	Created        time.Time
	Updated        time.Time
	Closed         *time.Time
	Labels         []*Label
	PatchURL       string `yaml:"patch_url"`
	Merged         bool
	MergedTime     *time.Time `yaml:"merged_time"`
	MergeCommitSHA string     `yaml:"merge_commit_sha"`
	Head           PullRequestBranch
	Base           PullRequestBranch
	Assignees      []string
	IsLocked       bool `yaml:"is_locked"`
	Reactions      []*Reaction
	Context        interface{} `yaml:"-"`
}

// IsForkPullRequest returns true if the pull request from a forked repository but not the same repository
func (p *PullRequest) IsForkPullRequest() bool {
	return p.Head.RepoPath() != p.Base.RepoPath()
}

// PullRequestBranch represents a pull request branch
type PullRequestBranch struct {
	CloneURL  string `yaml:"clone_url"`
	Ref       string
	SHA       string
	RepoName  string `yaml:"repo_name"`
	OwnerName string `yaml:"owner_name"`
}

// RepoPath returns pull request repo path
func (p PullRequestBranch) RepoPath() string {
	return fmt.Sprintf("%s/%s", p.OwnerName, p.RepoName)
}
