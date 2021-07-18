// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPullRequest_LoadAttributes(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.NoError(t, pr.LoadAttributes())
	assert.NotNil(t, pr.Merger)
	assert.Equal(t, pr.MergerID, pr.Merger.ID)
}

func TestPullRequest_LoadIssue(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.NoError(t, pr.LoadIssue())
	assert.NotNil(t, pr.Issue)
	assert.Equal(t, int64(2), pr.Issue.ID)
	assert.NoError(t, pr.LoadIssue())
	assert.NotNil(t, pr.Issue)
	assert.Equal(t, int64(2), pr.Issue.ID)
}

func TestPullRequest_LoadBaseRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.NoError(t, pr.LoadBaseRepo())
	assert.NotNil(t, pr.BaseRepo)
	assert.Equal(t, pr.BaseRepoID, pr.BaseRepo.ID)
	assert.NoError(t, pr.LoadBaseRepo())
	assert.NotNil(t, pr.BaseRepo)
	assert.Equal(t, pr.BaseRepoID, pr.BaseRepo.ID)
}

func TestPullRequest_LoadHeadRepo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.NoError(t, pr.LoadHeadRepo())
	assert.NotNil(t, pr.HeadRepo)
	assert.Equal(t, pr.HeadRepoID, pr.HeadRepo.ID)
}

// TODO TestMerge

// TODO TestNewPullRequest

func TestPullRequestsNewest(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	prs, count, err := PullRequests(1, &PullRequestsOptions{
		ListOptions: ListOptions{
			Page: 1,
		},
		State:    "open",
		SortType: "newest",
		Labels:   []string{},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 3, count)
	if assert.Len(t, prs, 3) {
		assert.EqualValues(t, 5, prs[0].ID)
		assert.EqualValues(t, 2, prs[1].ID)
		assert.EqualValues(t, 1, prs[2].ID)
	}
}

func TestPullRequestsOldest(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	prs, count, err := PullRequests(1, &PullRequestsOptions{
		ListOptions: ListOptions{
			Page: 1,
		},
		State:    "open",
		SortType: "oldest",
		Labels:   []string{},
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 3, count)
	if assert.Len(t, prs, 3) {
		assert.EqualValues(t, 1, prs[0].ID)
		assert.EqualValues(t, 2, prs[1].ID)
		assert.EqualValues(t, 5, prs[2].ID)
	}
}

func TestGetUnmergedPullRequest(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr, err := GetUnmergedPullRequest(1, 1, "branch2", "master", PullRequestStyleGithub)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pr.ID)

	_, err = GetUnmergedPullRequest(1, 9223372036854775807, "branch1", "master", PullRequestStyleGithub)
	assert.Error(t, err)
	assert.True(t, IsErrPullRequestNotExist(err))
}

func TestGetUnmergedPullRequestsByHeadInfo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	prs, err := GetUnmergedPullRequestsByHeadInfo(1, "branch2")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	for _, pr := range prs {
		assert.Equal(t, int64(1), pr.HeadRepoID)
		assert.Equal(t, "branch2", pr.HeadBranch)
	}
}

func TestGetUnmergedPullRequestsByBaseInfo(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	prs, err := GetUnmergedPullRequestsByBaseInfo(1, "master")
	assert.NoError(t, err)
	assert.Len(t, prs, 1)
	pr := prs[0]
	assert.Equal(t, int64(2), pr.ID)
	assert.Equal(t, int64(1), pr.BaseRepoID)
	assert.Equal(t, "master", pr.BaseBranch)
}

func TestGetPullRequestByIndex(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr, err := GetPullRequestByIndex(1, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), pr.BaseRepoID)
	assert.Equal(t, int64(2), pr.Index)

	_, err = GetPullRequestByIndex(9223372036854775807, 9223372036854775807)
	assert.Error(t, err)
	assert.True(t, IsErrPullRequestNotExist(err))
}

func TestGetPullRequestByID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr, err := GetPullRequestByID(1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), pr.ID)
	assert.Equal(t, int64(2), pr.IssueID)

	_, err = GetPullRequestByID(9223372036854775807)
	assert.Error(t, err)
	assert.True(t, IsErrPullRequestNotExist(err))
}

func TestGetPullRequestByIssueID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr, err := GetPullRequestByIssueID(2)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), pr.IssueID)

	_, err = GetPullRequestByIssueID(9223372036854775807)
	assert.Error(t, err)
	assert.True(t, IsErrPullRequestNotExist(err))
}

func TestPullRequest_Update(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	pr.BaseBranch = "baseBranch"
	pr.HeadBranch = "headBranch"
	pr.Update()

	pr = AssertExistsAndLoadBean(t, &PullRequest{ID: pr.ID}).(*PullRequest)
	assert.Equal(t, "baseBranch", pr.BaseBranch)
	assert.Equal(t, "headBranch", pr.HeadBranch)
	CheckConsistencyFor(t, pr)
}

func TestPullRequest_UpdateCols(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := &PullRequest{
		ID:         1,
		BaseBranch: "baseBranch",
		HeadBranch: "headBranch",
	}
	assert.NoError(t, pr.UpdateCols("head_branch"))

	pr = AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest)
	assert.Equal(t, "master", pr.BaseBranch)
	assert.Equal(t, "headBranch", pr.HeadBranch)
	CheckConsistencyFor(t, pr)
}

func TestPullRequestList_LoadAttributes(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	prs := []*PullRequest{
		AssertExistsAndLoadBean(t, &PullRequest{ID: 1}).(*PullRequest),
		AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest),
	}
	assert.NoError(t, PullRequestList(prs).LoadAttributes())
	for _, pr := range prs {
		assert.NotNil(t, pr.Issue)
		assert.Equal(t, pr.IssueID, pr.Issue.ID)
	}

	assert.NoError(t, PullRequestList([]*PullRequest{}).LoadAttributes())
}

// TODO TestAddTestPullRequestTask

func TestPullRequest_IsWorkInProgress(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest)
	pr.LoadIssue()

	assert.False(t, pr.IsWorkInProgress())

	pr.Issue.Title = "WIP: " + pr.Issue.Title
	assert.True(t, pr.IsWorkInProgress())

	pr.Issue.Title = "[wip]: " + pr.Issue.Title
	assert.True(t, pr.IsWorkInProgress())
}

func TestPullRequest_GetWorkInProgressPrefixWorkInProgress(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest)
	pr.LoadIssue()

	assert.Empty(t, pr.GetWorkInProgressPrefix())

	original := pr.Issue.Title
	pr.Issue.Title = "WIP: " + original
	assert.Equal(t, "WIP:", pr.GetWorkInProgressPrefix())

	pr.Issue.Title = "[wip] " + original
	assert.Equal(t, "[wip]", pr.GetWorkInProgressPrefix())
}

func TestPullRequest_GetDefaultMergeMessage_InternalTracker(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 2}).(*PullRequest)

	assert.Equal(t, "Merge pull request 'issue3' (#3) from branch2 into master", pr.GetDefaultMergeMessage())

	pr.BaseRepoID = 1
	pr.HeadRepoID = 2
	assert.Equal(t, "Merge pull request 'issue3' (#3) from user2/repo1:branch2 into master", pr.GetDefaultMergeMessage())
}

func TestPullRequest_GetDefaultMergeMessage_ExternalTracker(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	externalTracker := RepoUnit{
		Type: UnitTypeExternalTracker,
		Config: &ExternalTrackerConfig{
			ExternalTrackerFormat: "https://someurl.com/{user}/{repo}/{issue}",
		},
	}
	baseRepo := &Repository{Name: "testRepo", ID: 1}
	baseRepo.Owner = &User{Name: "testOwner"}
	baseRepo.Units = []*RepoUnit{&externalTracker}

	pr := AssertExistsAndLoadBean(t, &PullRequest{ID: 2, BaseRepo: baseRepo}).(*PullRequest)

	assert.Equal(t, "Merge pull request 'issue3' (!3) from branch2 into master", pr.GetDefaultMergeMessage())

	pr.BaseRepoID = 1
	pr.HeadRepoID = 2
	assert.Equal(t, "Merge pull request 'issue3' (!3) from user2/repo1:branch2 into master", pr.GetDefaultMergeMessage())
}
