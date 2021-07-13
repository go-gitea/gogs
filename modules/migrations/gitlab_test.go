// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/migrations/base"

	"github.com/stretchr/testify/assert"
)

func TestGitlabDownloadRepo(t *testing.T) {
	// Skip tests if Gitlab token is not found
	gitlabPersonalAccessToken := os.Getenv("GITLAB_READ_TOKEN")
	if gitlabPersonalAccessToken == "" {
		t.Skip("skipped test because GITLAB_READ_TOKEN was not in the environment")
	}

	resp, err := http.Get("https://gitlab.com/gitea/test_repo")
	if err != nil || resp.StatusCode != 200 {
		t.Skipf("Can't access test repo, skipping %s", t.Name())
	}

	downloader, err := NewGitlabDownloader(context.Background(), "https://gitlab.com", "gitea/test_repo", "", "", gitlabPersonalAccessToken)
	if err != nil {
		t.Fatal(fmt.Sprintf("NewGitlabDownloader is nil: %v", err))
	}
	repo, err := downloader.GetRepoInfo()
	assert.NoError(t, err)
	// Repo Owner is blank in Gitlab Group repos
	assert.EqualValues(t, &base.Repository{
		Name:          "test_repo",
		Owner:         "",
		Description:   "Test repository for testing migration from gitlab to gitea",
		CloneURL:      "https://gitlab.com/gitea/test_repo.git",
		OriginalURL:   "https://gitlab.com/gitea/test_repo",
		DefaultBranch: "master",
	}, repo)

	topics, err := downloader.GetTopics()
	assert.NoError(t, err)
	assert.True(t, len(topics) == 2)
	assert.EqualValues(t, []string{"migration", "test"}, topics)

	milestones, err := downloader.GetMilestones()
	assert.NoError(t, err)
	assert.True(t, len(milestones) >= 2)

	for _, milestone := range milestones {
		switch milestone.Title {
		case "1.0":
			assertMilestoneEqual(t, "", "1.0",
				"",
				"2019-11-28 08:42:30.301 +0000 UTC",
				"2019-11-28 15:57:52.401 +0000 UTC",
				"",
				"closed", milestone)
		case "1.1.0":
			assertMilestoneEqual(t, "", "1.1.0",
				"",
				"2019-11-28 08:42:44.575 +0000 UTC",
				"2019-11-28 08:42:44.575 +0000 UTC",
				"",
				"active", milestone)
		}
	}

	labels, err := downloader.GetLabels()
	assert.NoError(t, err)
	assert.True(t, len(labels) >= 9)
	for _, l := range labels {
		switch l.Name {
		case "bug":
			assertLabelEqual(t, "bug", "d9534f", "", l)
		case "documentation":
			assertLabelEqual(t, "documentation", "f0ad4e", "", l)
		case "confirmed":
			assertLabelEqual(t, "confirmed", "d9534f", "", l)
		case "enhancement":
			assertLabelEqual(t, "enhancement", "5cb85c", "", l)
		case "critical":
			assertLabelEqual(t, "critical", "d9534f", "", l)
		case "discussion":
			assertLabelEqual(t, "discussion", "428bca", "", l)
		case "suggestion":
			assertLabelEqual(t, "suggestion", "428bca", "", l)
		case "support":
			assertLabelEqual(t, "support", "f0ad4e", "", l)
		case "duplicate":
			assertLabelEqual(t, "duplicate", "7F8C8D", "", l)
		}
	}

	releases, err := downloader.GetReleases()
	assert.NoError(t, err)
	assert.EqualValues(t, []*base.Release{
		{
			TagName:         "v0.9.99",
			TargetCommitish: "0720a3ec57c1f843568298117b874319e7deee75",
			Name:            "First Release",
			Body:            "A test release",
			Created:         time.Date(2019, 11, 28, 9, 9, 48, 840000000, time.UTC),
			PublisherID:     1241334,
			PublisherName:   "lafriks",
		},
	}, releases[len(releases)-1:])

	issues, isEnd, err := downloader.GetIssues(1, 2)
	assert.NoError(t, err)
	assert.Len(t, issues, 2)
	assert.False(t, isEnd)

	var (
		closed1 = time.Date(2019, 11, 28, 8, 46, 23, 275000000, time.UTC)
		closed2 = time.Date(2019, 11, 28, 8, 45, 44, 959000000, time.UTC)
	)
	assert.EqualValues(t, []*base.Issue{
		{
			Number:     1,
			Title:      "Please add an animated gif icon to the merge button",
			Content:    "I just want the merge button to hurt my eyes a little. :stuck_out_tongue_closed_eyes:",
			Milestone:  "1.0.0",
			PosterID:   1241334,
			PosterName: "lafriks",
			State:      "closed",
			Created:    time.Date(2019, 11, 28, 8, 43, 35, 459000000, time.UTC),
			Updated:    time.Date(2019, 11, 28, 8, 46, 23, 304000000, time.UTC),
			Labels: []*base.Label{
				{
					Name: "bug",
				},
				{
					Name: "discussion",
				},
			},
			Reactions: []*base.Reaction{
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "thumbsup",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "open_mouth",
				}},
			Closed: &closed1,
		},
		{
			Number:     2,
			Title:      "Test issue",
			Content:    "This is test issue 2, do not touch!",
			Milestone:  "1.1.0",
			PosterID:   1241334,
			PosterName: "lafriks",
			State:      "closed",
			Created:    time.Date(2019, 11, 28, 8, 44, 46, 277000000, time.UTC),
			Updated:    time.Date(2019, 11, 28, 8, 45, 44, 987000000, time.UTC),
			Labels: []*base.Label{
				{
					Name: "duplicate",
				},
			},
			Reactions: []*base.Reaction{
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "thumbsup",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "thumbsdown",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "laughing",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "tada",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "confused",
				},
				{
					UserID:   1241334,
					UserName: "lafriks",
					Content:  "hearts",
				}},
			Closed: &closed2,
		},
	}, issues)

	comments, _, err := downloader.GetComments(base.GetCommentOptions{
		IssueNumber: 2,
	})
	assert.NoError(t, err)
	assert.Len(t, comments, 4)
	assert.EqualValues(t, []*base.Comment{
		{
			IssueIndex: 2,
			PosterID:   1241334,
			PosterName: "lafriks",
			Created:    time.Date(2019, 11, 28, 8, 44, 52, 501000000, time.UTC),
			Content:    "This is a comment",
			Reactions:  nil,
		},
		{
			IssueIndex: 2,
			PosterID:   1241334,
			PosterName: "lafriks",
			Created:    time.Date(2019, 11, 28, 8, 45, 2, 329000000, time.UTC),
			Content:    "changed milestone to %2",
			Reactions:  nil,
		},
		{
			IssueIndex: 2,
			PosterID:   1241334,
			PosterName: "lafriks",
			Created:    time.Date(2019, 11, 28, 8, 45, 45, 7000000, time.UTC),
			Content:    "closed",
			Reactions:  nil,
		},
		{
			IssueIndex: 2,
			PosterID:   1241334,
			PosterName: "lafriks",
			Created:    time.Date(2019, 11, 28, 8, 45, 53, 501000000, time.UTC),
			Content:    "A second comment",
			Reactions:  nil,
		},
	}, comments[:4])

	prs, _, err := downloader.GetPullRequests(1, 1)
	assert.NoError(t, err)
	assert.Len(t, prs, 1)

	assert.EqualValues(t, []*base.PullRequest{
		{
			Number:         4,
			OriginalNumber: 2,
			Title:          "Test branch",
			Content:        "do not merge this PR",
			Milestone:      "1.0.0",
			PosterID:       1241334,
			PosterName:     "lafriks",
			State:          "opened",
			Created:        time.Date(2019, 11, 28, 15, 56, 54, 104000000, time.UTC),
			Labels: []*base.Label{
				{
					Name: "bug",
				},
			},
			Reactions: []*base.Reaction{{
				UserID:   4575606,
				UserName: "real6543",
				Content:  "thumbsup",
			}, {
				UserID:   4575606,
				UserName: "real6543",
				Content:  "tada",
			}},
			PatchURL: "https://gitlab.com/gitea/test_repo/-/merge_requests/2.patch",
			Head: base.PullRequestBranch{
				Ref:       "feat/test",
				CloneURL:  "https://gitlab.com/gitea/test_repo/-/merge_requests/2",
				SHA:       "9f733b96b98a4175276edf6a2e1231489c3bdd23",
				RepoName:  "test_repo",
				OwnerName: "lafriks",
			},
			Base: base.PullRequestBranch{
				Ref:       "master",
				SHA:       "",
				OwnerName: "lafriks",
				RepoName:  "test_repo",
			},
			Closed:         nil,
			Merged:         false,
			MergedTime:     nil,
			MergeCommitSHA: "",
		},
	}, prs)

	rvs, err := downloader.GetReviews(1)
	assert.NoError(t, err)
	if assert.Len(t, rvs, 2) {
		for i := range rvs {
			switch rvs[i].ReviewerID {
			case 4102996:
				assert.EqualValues(t, "zeripath", rvs[i].ReviewerName)
				assert.EqualValues(t, "APPROVED", rvs[i].State)
			case 527793:
				assert.EqualValues(t, "axifive", rvs[i].ReviewerName)
				assert.EqualValues(t, "APPROVED", rvs[i].State)
			default:
				t.Errorf("Unexpected Reviewer ID: %d", rvs[i].ReviewerID)

			}
		}
	}
	rvs, err = downloader.GetReviews(2)
	assert.NoError(t, err)
	if assert.Len(t, prs, 1) {
		assert.EqualValues(t, 4575606, rvs[0].ReviewerID)
		assert.EqualValues(t, "real6543", rvs[0].ReviewerName)
		assert.EqualValues(t, "APPROVED", rvs[0].State)
	}

}
