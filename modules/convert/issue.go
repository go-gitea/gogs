// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"strings"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

// ToAPIIssue converts an Issue to API format
// it assumes some fields assigned with values:
// Required - Poster, Labels,
// Optional - Milestone, Assignee, PullRequest
func ToAPIIssue(issue *models.Issue) *api.Issue {
	if err := issue.LoadLabels(); err != nil {
		return &api.Issue{}
	}
	if err := issue.LoadPoster(); err != nil {
		return &api.Issue{}
	}
	if err := issue.LoadRepo(); err != nil {
		return &api.Issue{}
	}

	assets := make([]*api.Attachment, 0, len(issue.Attachments))
	for _, att := range issue.Attachments {
		assets = append(assets, ToIssueAttachment(att))
	}

	apiIssue := &api.Issue{
		ID:          issue.ID,
		URL:         issue.APIURL(),
		HTMLURL:     issue.HTMLURL(),
		Index:       issue.Index,
		Poster:      ToUser(issue.Poster, nil),
		Title:       issue.Title,
		Body:        issue.Content,
		Attachments: assets,
		Ref:         issue.Ref,
		Labels:      ToLabelList(issue.Labels),
		State:       issue.State(),
		IsLocked:    issue.IsLocked,
		Comments:    issue.NumComments,
		Created:     issue.CreatedUnix.AsTime(),
		Updated:     issue.UpdatedUnix.AsTime(),
	}

	apiIssue.Repo = &api.RepositoryMeta{
		ID:       issue.Repo.ID,
		Name:     issue.Repo.Name,
		Owner:    issue.Repo.OwnerName,
		FullName: issue.Repo.FullName(),
	}

	if issue.ClosedUnix != 0 {
		apiIssue.Closed = issue.ClosedUnix.AsTimePtr()
	}

	if err := issue.LoadMilestone(); err != nil {
		return &api.Issue{}
	}
	if issue.Milestone != nil {
		apiIssue.Milestone = ToAPIMilestone(issue.Milestone)
	}

	if err := issue.LoadAssignees(); err != nil {
		return &api.Issue{}
	}
	if len(issue.Assignees) > 0 {
		for _, assignee := range issue.Assignees {
			apiIssue.Assignees = append(apiIssue.Assignees, ToUser(assignee, nil))
		}
		apiIssue.Assignee = ToUser(issue.Assignees[0], nil) // For compatibility, we're keeping the first assignee as `apiIssue.Assignee`
	}
	if issue.IsPull {
		if err := issue.LoadPullRequest(); err != nil {
			return &api.Issue{}
		}
		apiIssue.PullRequest = &api.PullRequestMeta{
			HasMerged: issue.PullRequest.HasMerged,
		}
		if issue.PullRequest.HasMerged {
			apiIssue.PullRequest.Merged = issue.PullRequest.MergedUnix.AsTimePtr()
		}
	}
	if issue.DeadlineUnix != 0 {
		apiIssue.Deadline = issue.DeadlineUnix.AsTimePtr()
	}

	return apiIssue
}

// ToAPIIssueList converts an IssueList to API format
func ToAPIIssueList(il models.IssueList) []*api.Issue {
	result := make([]*api.Issue, len(il))
	for i := range il {
		result[i] = ToAPIIssue(il[i])
	}
	return result
}

// ToTrackedTime converts TrackedTime to API format
func ToTrackedTime(t *models.TrackedTime) (apiT *api.TrackedTime) {
	apiT = &api.TrackedTime{
		ID:       t.ID,
		IssueID:  t.IssueID,
		UserID:   t.UserID,
		UserName: t.User.Name,
		Time:     t.Time,
		Created:  t.Created,
	}
	if t.Issue != nil {
		apiT.Issue = ToAPIIssue(t.Issue)
	}
	if t.User != nil {
		apiT.UserName = t.User.Name
	}
	return
}

// ToStopWatches convert Stopwatch list to api.StopWatches
func ToStopWatches(sws []*models.Stopwatch) (api.StopWatches, error) {
	result := api.StopWatches(make([]api.StopWatch, 0, len(sws)))

	issueCache := make(map[int64]*models.Issue)
	repoCache := make(map[int64]*models.Repository)
	var (
		issue *models.Issue
		repo  *models.Repository
		ok    bool
		err   error
	)

	for _, sw := range sws {
		issue, ok = issueCache[sw.IssueID]
		if !ok {
			issue, err = models.GetIssueByID(sw.IssueID)
			if err != nil {
				return nil, err
			}
		}
		repo, ok = repoCache[issue.RepoID]
		if !ok {
			repo, err = models.GetRepositoryByID(issue.RepoID)
			if err != nil {
				return nil, err
			}
		}

		result = append(result, api.StopWatch{
			Created:       sw.CreatedUnix.AsTime(),
			Seconds:       sw.Seconds(),
			Duration:      sw.Duration(),
			IssueIndex:    issue.Index,
			IssueTitle:    issue.Title,
			RepoOwnerName: repo.OwnerName,
			RepoName:      repo.Name,
		})
	}
	return result, nil
}

// ToTrackedTimeList converts TrackedTimeList to API format
func ToTrackedTimeList(tl models.TrackedTimeList) api.TrackedTimeList {
	result := make([]*api.TrackedTime, 0, len(tl))
	for _, t := range tl {
		result = append(result, ToTrackedTime(t))
	}
	return result
}

// ToLabel converts Label to API format
func ToLabel(label *models.Label) *api.Label {
	return &api.Label{
		ID:          label.ID,
		Name:        label.Name,
		Color:       strings.TrimLeft(label.Color, "#"),
		Description: label.Description,
	}
}

// ToLabelList converts list of Label to API format
func ToLabelList(labels []*models.Label) []*api.Label {
	result := make([]*api.Label, len(labels))
	for i := range labels {
		result[i] = ToLabel(labels[i])
	}
	return result
}

// ToAPIMilestone converts Milestone into API Format
func ToAPIMilestone(m *models.Milestone) *api.Milestone {
	apiMilestone := &api.Milestone{
		ID:           m.ID,
		State:        m.State(),
		Title:        m.Name,
		Description:  m.Content,
		OpenIssues:   m.NumOpenIssues,
		ClosedIssues: m.NumClosedIssues,
		Created:      m.CreatedUnix.AsTime(),
		Updated:      m.UpdatedUnix.AsTimePtr(),
	}
	if m.IsClosed {
		apiMilestone.Closed = m.ClosedDateUnix.AsTimePtr()
	}
	if m.DeadlineUnix.Year() < 9999 {
		apiMilestone.Deadline = m.DeadlineUnix.AsTimePtr()
	}
	return apiMilestone
}

// ToIssueAttachment converts models.Attachment to api.Attachment
func ToIssueAttachment(a *models.Attachment) *api.Attachment {
	return &api.Attachment{
		ID:            a.ID,
		Name:          a.Name,
		Created:       a.CreatedUnix.AsTime(),
		DownloadCount: a.DownloadCount,
		Size:          a.Size,
		UUID:          a.UUID,
		DownloadURL:   a.DownloadURL(),
	}
}
