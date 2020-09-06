// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"

	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

// PullRequestsOptions holds the options for PRs
type PullRequestsOptions struct {
	ListOptions
	State       string
	SortType    string
	Labels      []string
	MilestoneID int64
}

func listPullRequestStatement(baseRepoID int64, opts *PullRequestsOptions) (*xorm.Session, error) {
	sess := x.Where(tbPullRequest+".base_repo_id=?", baseRepoID)

	sess.Join("INNER", tbIssue, tbPullRequest+".issue_id = "+tbIssue+".id")
	switch opts.State {
	case "closed", "open":
		sess.And(tbIssue+".is_closed=?", opts.State == "closed")
	}

	if labelIDs, err := base.StringsToInt64s(opts.Labels); err != nil {
		return nil, err
	} else if len(labelIDs) > 0 {
		sess.Join("INNER", tbIssueLabel, tbIssue+".id = "+tbIssueLabel+".issue_id").
			In(tbIssueLabel+".label_id", labelIDs)
	}

	if opts.MilestoneID > 0 {
		sess.And(tbIssue+".milestone_id=?", opts.MilestoneID)
	}

	return sess, nil
}

// GetUnmergedPullRequestsByHeadInfo returns all pull requests that are open and has not been merged
// by given head information (repo and branch).
func GetUnmergedPullRequestsByHeadInfo(repoID int64, branch string) ([]*PullRequest, error) {
	prs := make([]*PullRequest, 0, 2)
	return prs, x.
		Where("head_repo_id = ? AND head_branch = ? AND has_merged = ? AND "+tbIssue+".is_closed = ?",
			repoID, branch, false, false).
		Join("INNER", tbIssue, tbIssue+".id = "+tbPullRequest+".issue_id").
		Find(&prs)
}

// GetUnmergedPullRequestsByBaseInfo returns all pull requests that are open and has not been merged
// by given base information (repo and branch).
func GetUnmergedPullRequestsByBaseInfo(repoID int64, branch string) ([]*PullRequest, error) {
	prs := make([]*PullRequest, 0, 2)
	return prs, x.
		Where("base_repo_id=? AND base_branch=? AND has_merged=? AND "+tbIssue+".is_closed=?",
			repoID, branch, false, false).
		Join("INNER", tbIssue, tbIssue+".id="+tbPullRequest+".issue_id").
		Find(&prs)
}

// GetPullRequestIDsByCheckStatus returns all pull requests according the special checking status.
func GetPullRequestIDsByCheckStatus(status PullRequestStatus) ([]int64, error) {
	prs := make([]int64, 0, 10)
	return prs, x.Table(tbPullRequest).
		Where("status=?", status).
		Cols(tbPullRequest + ".id").
		Find(&prs)
}

// PullRequests returns all pull requests for a base Repo by the given conditions
func PullRequests(baseRepoID int64, opts *PullRequestsOptions) ([]*PullRequest, int64, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}

	countSession, err := listPullRequestStatement(baseRepoID, opts)
	if err != nil {
		log.Error("listPullRequestStatement: %v", err)
		return nil, 0, err
	}
	maxResults, err := countSession.Count(new(PullRequest))
	if err != nil {
		log.Error("Count PRs: %v", err)
		return nil, maxResults, err
	}

	findSession, err := listPullRequestStatement(baseRepoID, opts)
	sortIssuesSession(findSession, opts.SortType, 0)
	if err != nil {
		log.Error("listPullRequestStatement: %v", err)
		return nil, maxResults, err
	}
	findSession = opts.setSessionPagination(findSession)
	prs := make([]*PullRequest, 0, opts.PageSize)
	return prs, maxResults, findSession.Find(&prs)
}

// PullRequestList defines a list of pull requests
type PullRequestList []*PullRequest

func (prs PullRequestList) loadAttributes(e Engine) error {
	if len(prs) == 0 {
		return nil
	}

	// Load issues.
	issueIDs := prs.getIssueIDs()
	issues := make([]*Issue, 0, len(issueIDs))
	if err := e.
		Where("id > 0").
		In("id", issueIDs).
		Find(&issues); err != nil {
		return fmt.Errorf("find issues: %v", err)
	}

	set := make(map[int64]*Issue)
	for i := range issues {
		set[issues[i].ID] = issues[i]
	}
	for i := range prs {
		prs[i].Issue = set[prs[i].IssueID]
	}
	return nil
}

func (prs PullRequestList) getIssueIDs() []int64 {
	issueIDs := make([]int64, 0, len(prs))
	for i := range prs {
		issueIDs = append(issueIDs, prs[i].IssueID)
	}
	return issueIDs
}

// LoadAttributes load all the prs attributes
func (prs PullRequestList) LoadAttributes() error {
	return prs.loadAttributes(x)
}

func (prs PullRequestList) invalidateCodeComments(e Engine, doer *User, repo *git.Repository, branch string) error {
	if len(prs) == 0 {
		return nil
	}
	issueIDs := prs.getIssueIDs()
	var codeComments []*Comment
	if err := e.
		Where("type = ? and invalidated = ?", CommentTypeCode, false).
		In("issue_id", issueIDs).
		Find(&codeComments); err != nil {
		return fmt.Errorf("find code comments: %v", err)
	}
	for _, comment := range codeComments {
		if err := comment.CheckInvalidation(repo, doer, branch); err != nil {
			return err
		}
	}
	return nil
}

// InvalidateCodeComments will lookup the prs for code comments which got invalidated by change
func (prs PullRequestList) InvalidateCodeComments(doer *User, repo *git.Repository, branch string) error {
	return prs.invalidateCodeComments(x, doer, repo, branch)
}
