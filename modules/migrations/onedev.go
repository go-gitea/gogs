// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/structs"

	jsoniter "github.com/json-iterator/go"
)

var (
	_ base.Downloader        = &OneDevDownloader{}
	_ base.DownloaderFactory = &OneDevDownloaderFactory{}
)

func init() {
	RegisterDownloaderFactory(&OneDevDownloaderFactory{})
}

// OneDevDownloaderFactory defines a downloader factory
type OneDevDownloaderFactory struct {
}

// New returns a downloader related to this factory according MigrateOptions
func (f *OneDevDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	repoName := ""

	fields := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(fields) == 2 && fields[0] == "projects" {
		repoName = fields[1]
	} else if len(fields) == 1 {
		repoName = fields[0]
	} else {
		return nil, fmt.Errorf("invalid path: %s", u.Path)
	}

	u.Path = ""
	u.Fragment = ""

	log.Trace("Create onedev downloader. BaseURL: %v RepoName: %s", u, repoName)

	return NewOneDevDownloader(ctx, u, opts.AuthUsername, opts.AuthPassword, repoName), nil
}

// GitServiceType returns the type of git service
func (f *OneDevDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.OneDevService
}

type onedevUser struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// OneDevDownloader implements a Downloader interface to get repository informations
// from OneDev
type OneDevDownloader struct {
	base.NullDownloader
	ctx           context.Context
	client        *http.Client
	baseURL       *url.URL
	repoName      string
	repoID        int64
	maxIssueIndex int64
	userMap       map[int64]*onedevUser
	milestoneMap  map[int64]string
}

// SetContext set context
func (d *OneDevDownloader) SetContext(ctx context.Context) {
	d.ctx = ctx
}

// NewOneDevDownloader creates a new downloader
func NewOneDevDownloader(ctx context.Context, baseURL *url.URL, username, password, repoName string) *OneDevDownloader {
	var downloader = &OneDevDownloader{
		ctx:      ctx,
		baseURL:  baseURL,
		repoName: repoName,
		client: &http.Client{
			Transport: &http.Transport{
				Proxy: func(req *http.Request) (*url.URL, error) {
					if len(username) > 0 && len(password) > 0 {
						req.SetBasicAuth(username, password)
					}
					return nil, nil
				},
			},
		},
		userMap:      make(map[int64]*onedevUser),
		milestoneMap: make(map[int64]string),
	}

	return downloader
}

func (d *OneDevDownloader) callAPI(endpoint string, parameter map[string]string, result interface{}) error {
	u, err := d.baseURL.Parse(endpoint)
	if err != nil {
		return err
	}

	if parameter != nil {
		query := u.Query()
		for k, v := range parameter {
			query.Set(k, v)
		}
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	decoder := jsoniter.NewDecoder(resp.Body)
	return decoder.Decode(&result)
}

// GetRepoInfo returns a repository information
func (d *OneDevDownloader) GetRepoInfo() (*base.Repository, error) {
	info := make([]struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}, 0, 1)

	err := d.callAPI(
		"/api/projects",
		map[string]string{
			"query":  `"Name" is "` + d.repoName + `"`,
			"offset": "0",
			"count":  "1",
		},
		&info,
	)
	if err != nil {
		return nil, err
	}
	if len(info) != 1 {
		return nil, fmt.Errorf("Project %s not found", d.repoName)
	}

	d.repoID = info[0].ID

	cloneURL, err := d.baseURL.Parse(info[0].Name)
	if err != nil {
		return nil, err
	}
	originalURL, err := d.baseURL.Parse("/projects/" + info[0].Name)
	if err != nil {
		return nil, err
	}

	return &base.Repository{
		Name:        info[0].Name,
		Description: info[0].Description,
		CloneURL:    cloneURL.String(),
		OriginalURL: originalURL.String(),
	}, nil
}

// GetMilestones returns milestones
func (d *OneDevDownloader) GetMilestones() ([]*base.Milestone, error) {
	rawMilestones := make([]struct {
		ID          int64      `json:"id"`
		Name        string     `json:"name"`
		Description string     `json:"description"`
		DueDate     *time.Time `json:"dueDate"`
		Closed      bool       `json:"closed"`
	}, 0, 100)

	endpoint := fmt.Sprintf("/api/projects/%d/milestones", d.repoID)

	t := time.Now()

	var milestones = make([]*base.Milestone, 0, 100)
	offset := 0
	for {
		err := d.callAPI(
			endpoint,
			map[string]string{
				"offset": strconv.Itoa(offset),
				"count":  "100",
			},
			&rawMilestones,
		)
		if err != nil {
			return nil, err
		}
		if len(rawMilestones) == 0 {
			break
		}
		offset += 100

		for _, milestone := range rawMilestones {
			d.milestoneMap[milestone.ID] = milestone.Name
			closed := &t
			if !milestone.Closed {
				closed = nil
			}

			milestones = append(milestones, &base.Milestone{
				Title:       milestone.Name,
				Description: milestone.Description,
				Deadline:    milestone.DueDate,
				Created:     t,
				Updated:     &t,
				Closed:      closed,
			})
		}
	}
	return milestones, nil
}

// GetLabels returns labels
func (d *OneDevDownloader) GetLabels() ([]*base.Label, error) {
	return nil, nil
}

type onedevIssueContext struct {
	OriginalID    int64
	MigratedID    int64
	IsPullRequest bool
}

// GetIssues returns issues according start and limit, perPage is not supported
func (d *OneDevDownloader) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	rawIssues := make([]struct {
		ID          int64     `json:"id"`
		Number      int64     `json:"number"`
		State       string    `json:"state"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		MilestoneID int64     `json:"milestoneId"`
		SubmitterID int64     `json:"submitterId"`
		SubmitDate  time.Time `json:"submitDate"`
	}, 0, perPage)

	err := d.callAPI(
		"/api/issues",
		map[string]string{
			"query":  `"Project" is "` + d.repoName + `"`,
			"offset": strconv.Itoa((page - 1) * perPage),
			"count":  strconv.Itoa(perPage),
		},
		&rawIssues,
	)
	if err != nil {
		return nil, false, err
	}

	issues := make([]*base.Issue, 0, len(rawIssues))
	for _, issue := range rawIssues {
		state := strings.ToLower(issue.State)
		if state == "released" {
			state = "closed"
		}
		poster := d.tryGetUser(issue.SubmitterID)
		issues = append(issues, &base.Issue{
			Title:       issue.Title,
			Number:      issue.Number,
			PosterName:  poster.Name,
			PosterEmail: poster.Email,
			Content:     issue.Description,
			Milestone:   d.milestoneMap[issue.MilestoneID],
			State:       state,
			Created:     issue.SubmitDate,
			//Closed:      closed,
			Context: onedevIssueContext{
				OriginalID:    issue.ID,
				MigratedID:    issue.Number,
				IsPullRequest: false,
			},
		})

		if d.maxIssueIndex < issue.Number {
			d.maxIssueIndex = issue.Number
		}
	}

	return issues, len(issues) == 0, nil
}

// GetComments returns requested comments
func (d *OneDevDownloader) GetComments(opts base.GetCommentOptions) ([]*base.Comment, bool, error) {
	context, ok := opts.Context.(onedevIssueContext)
	if !ok {
		return nil, false, fmt.Errorf("unexpected comment context: %+v", opts.Context)
	}

	rawComments := make([]struct {
		Date    time.Time `json:"date"`
		UserID  int64     `json:"userId"`
		Content string    `json:"content"`
	}, 0, 100)

	var endpoint string
	if context.IsPullRequest {
		endpoint = fmt.Sprintf("/api/pull-requests/%d/comments", context.OriginalID)
	} else {
		endpoint = fmt.Sprintf("/api/issues/%d/comments", context.OriginalID)
	}

	err := d.callAPI(
		endpoint,
		nil,
		&rawComments,
	)
	if err != nil {
		return nil, false, err
	}

	rawChanges := make([]struct {
		Date   time.Time              `json:"date"`
		UserID int64                  `json:"userId"`
		Data   map[string]interface{} `json:"data"`
	}, 0, 100)

	if context.IsPullRequest {
		endpoint = fmt.Sprintf("/api/pull-requests/%d/changes", context.OriginalID)
	} else {
		endpoint = fmt.Sprintf("/api/issues/%d/changes", context.OriginalID)
	}

	err = d.callAPI(
		endpoint,
		nil,
		&rawChanges,
	)
	if err != nil {
		return nil, false, err
	}

	comments := make([]*base.Comment, 0, len(rawComments)+len(rawChanges))
	for _, comment := range rawComments {
		if len(comment.Content) == 0 {
			continue
		}
		poster := d.tryGetUser(comment.UserID)
		comments = append(comments, &base.Comment{
			IssueIndex:  context.MigratedID,
			PosterID:    poster.ID,
			PosterName:  poster.Name,
			PosterEmail: poster.Email,
			Content:     comment.Content,
			Created:     comment.Date,
			Updated:     comment.Date,
		})
	}
	for _, change := range rawChanges {
		contentV, ok := change.Data["content"]
		if !ok {
			contentV, ok = change.Data["comment"]
			if !ok {
				continue
			}
		}
		content, ok := contentV.(string)
		if !ok || len(content) == 0 {
			continue
		}

		poster := d.tryGetUser(change.UserID)
		comments = append(comments, &base.Comment{
			IssueIndex:  context.MigratedID,
			PosterID:    poster.ID,
			PosterName:  poster.Name,
			PosterEmail: poster.Email,
			Content:     content,
			Created:     change.Date,
			Updated:     change.Date,
		})
	}

	return comments, false, nil
}

// GetPullRequests returns requested pull requests
func (d *OneDevDownloader) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	rawPullRequests := make([]struct {
		ID             int64     `json:"id"`
		Number         int64     `json:"number"`
		Title          string    `json:"title"`
		SubmitterID    int64     `json:"submitterId"`
		SubmitDate     time.Time `json:"submitDate"`
		Description    string    `json:"description"`
		TargetBranch   string    `json:"targetBranch"`
		SourceBranch   string    `json:"sourceBranch"`
		BaseCommitHash string    `json:"baseCommitHash"`
		CloseInfo      *struct {
			Date   *time.Time `json:"date"`
			Status string     `json:"status"`
		}
	}, 0, perPage)

	err := d.callAPI(
		"/api/pull-requests",
		map[string]string{
			"query":  `"Target Project" is "` + d.repoName + `"`,
			"offset": strconv.Itoa((page - 1) * perPage),
			"count":  strconv.Itoa(perPage),
		},
		&rawPullRequests,
	)
	if err != nil {
		return nil, false, err
	}

	pullRequests := make([]*base.PullRequest, 0, len(rawPullRequests))
	for _, pr := range rawPullRequests {
		state := "open"
		merged := false
		var closeTime *time.Time
		var mergedTime *time.Time
		if pr.CloseInfo != nil {
			state = "closed"
			closeTime = pr.CloseInfo.Date
			if pr.CloseInfo.Status == "MERGED" { // "DISCARDED"
				merged = true
				mergedTime = pr.CloseInfo.Date
			}
		}
		poster := d.tryGetUser(pr.SubmitterID)

		number := pr.Number + d.maxIssueIndex
		pullRequests = append(pullRequests, &base.PullRequest{
			Title:      pr.Title,
			Number:     number,
			PosterName: poster.Name,
			PosterID:   poster.ID,
			Content:    pr.Description,
			State:      state,
			Created:    pr.SubmitDate,
			Closed:     closeTime,
			Merged:     merged,
			MergedTime: mergedTime,
			Head: base.PullRequestBranch{
				Ref:      pr.SourceBranch,
				SHA:      pr.BaseCommitHash,
				RepoName: d.repoName,
			},
			Base: base.PullRequestBranch{
				Ref:      pr.TargetBranch,
				SHA:      "",
				RepoName: d.repoName,
			},
			Context: onedevIssueContext{
				OriginalID:    pr.ID,
				MigratedID:    number,
				IsPullRequest: true,
			},
		})
	}

	return pullRequests, len(pullRequests) == 0, nil
}

// GetReviews returns requested pull requests reviews
func (d *OneDevDownloader) GetReviews(context interface{}) ([]*base.Review, error) {
	issueContext, ok := context.(onedevIssueContext)
	if !ok {
		return nil, fmt.Errorf("unexpected context: %+v", context)
	}

	rawReviews := make([]struct {
		ID     int64 `json:"id"`
		UserID int64 `json:"userId"`
		Result *struct {
			Commit   string `json:"commit"`
			Approved bool   `json:"approved"`
			Comment  string `json:"comment"`
		}
	}, 0, 100)

	err := d.callAPI(
		fmt.Sprintf("/api/pull-requests/%d/reviews", issueContext.OriginalID),
		nil,
		&rawReviews,
	)
	if err != nil {
		return nil, err
	}

	var reviews = make([]*base.Review, 0, len(rawReviews))
	for _, review := range rawReviews {
		state := base.ReviewStatePending
		content := ""
		if review.Result != nil {
			if len(review.Result.Comment) > 0 {
				state = base.ReviewStateCommented
				content = review.Result.Comment
			}
			if review.Result.Approved {
				state = base.ReviewStateApproved
			}
		}

		poster := d.tryGetUser(review.UserID)
		reviews = append(reviews, &base.Review{
			IssueIndex:   issueContext.MigratedID,
			ReviewerID:   poster.ID,
			ReviewerName: poster.Name,
			CreatedAt:    time.Now(),
			Content:      content,
			State:        state,
		})
	}

	return reviews, nil
}

// GetTopics return repository topics
func (d *OneDevDownloader) GetTopics() ([]string, error) {
	return []string{}, nil
}

func (d *OneDevDownloader) tryGetUser(userID int64) *onedevUser {
	user, ok := d.userMap[userID]
	if !ok {
		err := d.callAPI(
			fmt.Sprintf("/api/users/%d", userID),
			nil,
			&user,
		)
		if err != nil {
			user = &onedevUser{
				Name: fmt.Sprintf("User %d", userID),
			}
		}
		d.userMap[userID] = user
	}

	return user
}
