// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/migrations/base"

	"github.com/stretchr/testify/assert"
)

func assertEqualIssue(t *testing.T, issueExp, issueGet *base.Issue) {
	assert.EqualValues(t, issueExp.Number, issueGet.Number)
	assert.EqualValues(t, issueExp.Title, issueGet.Title)
	assert.EqualValues(t, issueExp.Content, issueGet.Content)
	assert.EqualValues(t, issueExp.Milestone, issueGet.Milestone)
	assert.EqualValues(t, issueExp.PosterID, issueGet.PosterID)
	assert.EqualValues(t, issueExp.PosterName, issueGet.PosterName)
	assert.EqualValues(t, issueExp.PosterEmail, issueGet.PosterEmail)
	assert.EqualValues(t, issueExp.IsLocked, issueGet.IsLocked)
	assert.EqualValues(t, issueExp.Created.Unix(), issueGet.Created.Unix())
	assert.EqualValues(t, issueExp.Updated.Unix(), issueGet.Updated.Unix())
	if issueExp.Closed != nil {
		assert.EqualValues(t, issueExp.Closed.Unix(), issueGet.Closed.Unix())
	} else {
		assert.True(t, issueGet.Closed == nil)
	}
	sort.Strings(issueExp.Assignees)
	sort.Strings(issueGet.Assignees)
	assert.EqualValues(t, issueExp.Assignees, issueGet.Assignees)
	assert.EqualValues(t, issueExp.Labels, issueGet.Labels)
	assert.EqualValues(t, issueExp.Reactions, issueGet.Reactions)
}

func TestGiteaDownloadRepo(t *testing.T) {
	// Skip tests if Gitea token is not found
	giteaToken := os.Getenv("GITEA_TOKEN")
	if giteaToken == "" {
		t.Skip("skipped test because GITEA_TOKEN was not in the environment")
	}

	resp, err := http.Get("https://gitea.com/gitea")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("Can't reach https://gitea.com, skipping %s", t.Name())
	}

	downloader, err := NewGiteaDownloader(context.Background(), "https://gitea.com", "gitea/test_repo", "", "", giteaToken)
	if downloader == nil {
		t.Fatal("NewGitlabDownloader is nil")
	}
	if !assert.NoError(t, err) {
		t.Fatal("NewGitlabDownloader error occur")
	}

	repo, err := downloader.GetRepoInfo()
	assert.NoError(t, err)
	assert.EqualValues(t, &base.Repository{
		Name:          "test_repo",
		Owner:         "gitea",
		IsPrivate:     false,
		Description:   "Test repository for testing migration from gitea to gitea",
		CloneURL:      "https://gitea.com/gitea/test_repo.git",
		OriginalURL:   "https://gitea.com/gitea/test_repo",
		DefaultBranch: "master",
	}, repo)

	topics, err := downloader.GetTopics()
	assert.NoError(t, err)
	sort.Strings(topics)
	assert.EqualValues(t, []string{"ci", "gitea", "migration", "test"}, topics)

	labels, err := downloader.GetLabels()
	assert.NoError(t, err)
	assert.Len(t, labels, 6)
	for _, l := range labels {
		switch l.Name {
		case "Bug":
			assertLabelEqual(t, "Bug", "e11d21", "", l)
		case "documentation":
			assertLabelEqual(t, "Enhancement", "207de5", "", l)
		case "confirmed":
			assertLabelEqual(t, "Feature", "0052cc", "a feature request", l)
		case "enhancement":
			assertLabelEqual(t, "Invalid", "d4c5f9", "", l)
		case "critical":
			assertLabelEqual(t, "Question", "fbca04", "", l)
		case "discussion":
			assertLabelEqual(t, "Valid", "53e917", "", l)
		default:
			assert.Error(t, fmt.Errorf("unexpected label: %s", l.Name))
		}
	}

	milestones, err := downloader.GetMilestones()
	assert.NoError(t, err)
	assert.Len(t, milestones, 2)

	for _, milestone := range milestones {
		switch milestone.Title {
		case "V1":
			assert.EqualValues(t, "Generate Content", milestone.Description)
			// assert.EqualValues(t, "ToDo", milestone.Created)
			// assert.EqualValues(t, "ToDo", milestone.Updated)
			assert.EqualValues(t, 1598985406, milestone.Closed.Unix())
			assert.True(t, milestone.Deadline == nil)
			assert.EqualValues(t, "closed", milestone.State)
		case "V2 Finalize":
			assert.EqualValues(t, "", milestone.Description)
			// assert.EqualValues(t, "ToDo", milestone.Created)
			// assert.EqualValues(t, "ToDo", milestone.Updated)
			assert.True(t, milestone.Closed == nil)
			assert.EqualValues(t, 1599263999, milestone.Deadline.Unix())
			assert.EqualValues(t, "open", milestone.State)
		default:
			assert.Error(t, fmt.Errorf("unexpected milestone: %s", milestone.Title))
		}
	}

	releases, err := downloader.GetReleases()
	assert.NoError(t, err)
	assert.EqualValues(t, []*base.Release{
		{
			Name:            "Second Release",
			TagName:         "v2-rc1",
			TargetCommitish: "master",
			Body:            "this repo has:\r\n* reactions\r\n* wiki\r\n* issues  (open/closed)\r\n* pulls (open/closed/merged) (external/internal)\r\n* pull reviews\r\n* projects\r\n* milestones\r\n* labels\r\n* releases\r\n\r\nto test migration against",
			Draft:           false,
			Prerelease:      true,
			Created:         time.Date(2020, 9, 1, 18, 2, 43, 0, time.UTC),
			Published:       time.Date(2020, 9, 1, 18, 2, 43, 0, time.UTC),
			PublisherID:     689,
			PublisherName:   "6543",
			PublisherEmail:  "6543@noreply.gitea.io",
		},
		{
			Name:            "First Release",
			TagName:         "V1",
			TargetCommitish: "master",
			Body:            "as title",
			Draft:           false,
			Prerelease:      false,
			Created:         time.Date(2020, 9, 1, 17, 30, 32, 0, time.UTC),
			Published:       time.Date(2020, 9, 1, 17, 30, 32, 0, time.UTC),
			PublisherID:     689,
			PublisherName:   "6543",
			PublisherEmail:  "6543@noreply.gitea.io",
		},
	}, releases)

	issues, isEnd, err := downloader.GetIssues(1, 50)
	assert.NoError(t, err)
	assert.Len(t, issues, 7)
	assert.True(t, isEnd)
	assert.EqualValues(t, "open", issues[0].State)

	issues, isEnd, err = downloader.GetIssues(3, 2)
	assert.NoError(t, err)
	assert.Len(t, issues, 2)
	assert.False(t, isEnd)

	var (
		closed4 = time.Date(2020, 9, 1, 15, 49, 34, 0, time.UTC)
		closed2 = time.Unix(1598969497, 0)
	)

	assertEqualIssue(t, &base.Issue{
		Number:      4,
		Title:       "what is this repo about?",
		Content:     "",
		Milestone:   "V1",
		PosterID:    -1,
		PosterName:  "Ghost",
		PosterEmail: "",
		State:       "closed",
		IsLocked:    true,
		Created:     time.Unix(1598975321, 0),
		Updated:     time.Unix(1598975400, 0),
		Labels: []*base.Label{{
			Name:        "Question",
			Color:       "fbca04",
			Description: "",
		}},
		Reactions: []*base.Reaction{
			{
				UserID:   689,
				UserName: "6543",
				Content:  "gitea",
			},
			{
				UserID:   689,
				UserName: "6543",
				Content:  "laugh",
			},
		},
		Closed: &closed4,
	}, issues[0])
	assertEqualIssue(t, &base.Issue{
		Number:      2,
		Title:       "Spam",
		Content:     ":(",
		Milestone:   "",
		PosterID:    689,
		PosterName:  "6543",
		PosterEmail: "6543@noreply.gitea.io",
		State:       "closed",
		IsLocked:    false,
		Created:     time.Unix(1598919780, 0),
		Updated:     closed2,
		Labels: []*base.Label{{
			Name:        "Invalid",
			Color:       "d4c5f9",
			Description: "",
		}},
		Reactions: nil,
		Closed:    &closed2,
	}, issues[1])

	comments, _, err := downloader.GetComments(base.GetCommentOptions{
		IssueNumber: 4,
	})
	assert.NoError(t, err)
	assert.Len(t, comments, 2)
	assert.EqualValues(t, 1598975370, comments[0].Created.Unix())
	assert.EqualValues(t, 1599070865, comments[0].Updated.Unix())
	assert.EqualValues(t, 1598975393, comments[1].Created.Unix())
	assert.EqualValues(t, 1598975393, comments[1].Updated.Unix())
	assert.EqualValues(t, []*base.Comment{
		{
			IssueIndex:  4,
			PosterID:    689,
			PosterName:  "6543",
			PosterEmail: "6543@noreply.gitea.io",
			Created:     comments[0].Created,
			Updated:     comments[0].Updated,
			Content:     "a really good question!\n\nIt is the used as TESTSET for gitea2gitea repo migration function",
		},
		{
			IssueIndex:  4,
			PosterID:    -1,
			PosterName:  "Ghost",
			PosterEmail: "",
			Created:     comments[1].Created,
			Updated:     comments[1].Updated,
			Content:     "Oh!",
		},
	}, comments)

	prs, isEnd, err := downloader.GetPullRequests(1, 50)
	assert.NoError(t, err)
	assert.True(t, isEnd)
	assert.Len(t, prs, 6)
	prs, isEnd, err = downloader.GetPullRequests(1, 3)
	assert.NoError(t, err)
	assert.False(t, isEnd)
	assert.Len(t, prs, 3)
	merged12 := time.Unix(1598982934, 0)
	assertEqualPulls(t, &base.PullRequest{
		Number:      12,
		PosterID:    689,
		PosterName:  "6543",
		PosterEmail: "6543@noreply.gitea.io",
		Title:       "Dont Touch",
		Content:     "\r\nadd dont touch note",
		Milestone:   "V2 Finalize",
		State:       "closed",
		IsLocked:    false,
		Created:     time.Unix(1598982759, 0),
		Updated:     time.Unix(1599023425, 0),
		Closed:      &merged12,
		Assignees:   []string{"techknowlogick"},
		Labels:      []*base.Label{},

		Base: base.PullRequestBranch{
			CloneURL:  "",
			Ref:       "master",
			SHA:       "827aa28a907853e5ddfa40c8f9bc52471a2685fd",
			RepoName:  "test_repo",
			OwnerName: "gitea",
		},
		Head: base.PullRequestBranch{
			CloneURL:  "https://gitea.com/6543-forks/test_repo.git",
			Ref:       "refs/pull/12/head",
			SHA:       "b6ab5d9ae000b579a5fff03f92c486da4ddf48b6",
			RepoName:  "test_repo",
			OwnerName: "6543-forks",
		},
		Merged:         true,
		MergedTime:     &merged12,
		MergeCommitSHA: "827aa28a907853e5ddfa40c8f9bc52471a2685fd",
		PatchURL:       "https://gitea.com/gitea/test_repo/pulls/12.patch",
	}, prs[1])

	reviews, err := downloader.GetReviews(7)
	assert.NoError(t, err)
	if assert.Len(t, reviews, 3) {
		assert.EqualValues(t, 689, reviews[0].ReviewerID)
		assert.EqualValues(t, "6543", reviews[0].ReviewerName)
		assert.EqualValues(t, "techknowlogick", reviews[1].ReviewerName)
		assert.EqualValues(t, "techknowlogick", reviews[2].ReviewerName)
		assert.False(t, reviews[1].Official)
		assert.EqualValues(t, "I think this needs some changes", reviews[1].Content)
		assert.EqualValues(t, "REQUEST_CHANGES", reviews[1].State)
		assert.True(t, reviews[2].Official)
		assert.EqualValues(t, "looks good", reviews[2].Content)
		assert.EqualValues(t, "APPROVED", reviews[2].State)

		// TODO: https://github.com/go-gitea/gitea/issues/12846
		// assert.EqualValues(t, 9, reviews[1].ReviewerID)
		// assert.EqualValues(t, 9, reviews[2].ReviewerID)

		assert.Len(t, reviews[0].Comments, 1)
		assert.EqualValues(t, &base.ReviewComment{
			ID:        116561,
			InReplyTo: 0,
			Content:   "is one `\\newline` to less?",
			TreePath:  "README.md",
			DiffHunk:  "@@ -2,3 +2,3 @@\n \n-Test repository for testing migration from gitea 2 gitea\n\\ No newline at end of file\n+Test repository for testing migration from gitea 2 gitea",
			Position:  0,
			Line:      4,
			CommitID:  "187ece0cb6631e2858a6872e5733433bb3ca3b03",
			PosterID:  689,
			Reactions: nil,
			CreatedAt: time.Date(2020, 9, 1, 16, 12, 58, 0, time.UTC),
			UpdatedAt: time.Date(2020, 9, 1, 16, 12, 58, 0, time.UTC),
		}, reviews[0].Comments[0])
	}
}

func assertEqualPulls(t *testing.T, pullExp, pullGet *base.PullRequest) {
	assertEqualIssue(t, pull2issue(pullExp), pull2issue(pullGet))
	assert.EqualValues(t, 0, pullGet.OriginalNumber)
	assert.EqualValues(t, pullExp.PatchURL, pullGet.PatchURL)
	assert.EqualValues(t, pullExp.Merged, pullGet.Merged)
	assert.EqualValues(t, pullExp.MergedTime.Unix(), pullGet.MergedTime.Unix())
	assert.EqualValues(t, pullExp.MergeCommitSHA, pullGet.MergeCommitSHA)
	assert.EqualValues(t, pullExp.Base, pullGet.Base)
	assert.EqualValues(t, pullExp.Head, pullGet.Head)
}

func pull2issue(pull *base.PullRequest) *base.Issue {
	return &base.Issue{
		Number:      pull.Number,
		PosterID:    pull.PosterID,
		PosterName:  pull.PosterName,
		PosterEmail: pull.PosterEmail,
		Title:       pull.Title,
		Content:     pull.Content,
		Milestone:   pull.Milestone,
		State:       pull.State,
		IsLocked:    pull.IsLocked,
		Created:     pull.Created,
		Updated:     pull.Updated,
		Closed:      pull.Closed,
		Labels:      pull.Labels,
		Reactions:   pull.Reactions,
		Assignees:   pull.Assignees,
	}
}
