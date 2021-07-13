// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"

	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
)

var (
	_ base.Downloader        = &GithubDownloaderV3{}
	_ base.DownloaderFactory = &GithubDownloaderV3Factory{}
	// GithubLimitRateRemaining limit to wait for new rate to apply
	GithubLimitRateRemaining = 0
)

func init() {
	RegisterDownloaderFactory(&GithubDownloaderV3Factory{})
}

// GithubDownloaderV3Factory defines a github downloader v3 factory
type GithubDownloaderV3Factory struct {
}

// New returns a Downloader related to this factory according MigrateOptions
func (f *GithubDownloaderV3Factory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	baseURL := u.Scheme + "://" + u.Host
	fields := strings.Split(u.Path, "/")
	oldOwner := fields[1]
	oldName := strings.TrimSuffix(fields[2], ".git")

	log.Trace("Create github downloader: %s/%s", oldOwner, oldName)

	return NewGithubDownloaderV3(ctx, baseURL, opts.AuthUsername, opts.AuthPassword, opts.AuthToken, oldOwner, oldName), nil
}

// GitServiceType returns the type of git service
func (f *GithubDownloaderV3Factory) GitServiceType() structs.GitServiceType {
	return structs.GithubService
}

// GithubDownloaderV3 implements a Downloader interface to get repository information
// from github via APIv3
type GithubDownloaderV3 struct {
	base.NullDownloader
	ctx        context.Context
	client     *github.Client
	repoOwner  string
	repoName   string
	userName   string
	password   string
	rate       *github.Rate
	maxPerPage int
}

// NewGithubDownloaderV3 creates a github Downloader via github v3 API
func NewGithubDownloaderV3(ctx context.Context, baseURL, userName, password, token, repoOwner, repoName string) *GithubDownloaderV3 {
	var downloader = GithubDownloaderV3{
		userName:   userName,
		password:   password,
		ctx:        ctx,
		repoOwner:  repoOwner,
		repoName:   repoName,
		maxPerPage: 100,
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				req.SetBasicAuth(userName, password)
				return nil, nil
			},
		},
	}
	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		client = oauth2.NewClient(downloader.ctx, ts)
	}
	downloader.client = github.NewClient(client)
	if baseURL != "https://github.com" {
		downloader.client, _ = github.NewEnterpriseClient(baseURL, baseURL, client)
	}
	return &downloader
}

// SetContext set context
func (g *GithubDownloaderV3) SetContext(ctx context.Context) {
	g.ctx = ctx
}

func (g *GithubDownloaderV3) sleep() {
	for g.rate != nil && g.rate.Remaining <= GithubLimitRateRemaining {
		timer := time.NewTimer(time.Until(g.rate.Reset.Time))
		select {
		case <-g.ctx.Done():
			util.StopTimer(timer)
			return
		case <-timer.C:
		}

		err := g.RefreshRate()
		if err != nil {
			log.Error("g.client.RateLimits: %s", err)
		}
	}
}

// RefreshRate update the current rate (doesn't count in rate limit)
func (g *GithubDownloaderV3) RefreshRate() error {
	rates, _, err := g.client.RateLimits(g.ctx)
	if err != nil {
		// if rate limit is not enabled, ignore it
		if strings.Contains(err.Error(), "404") {
			g.rate = nil
			return nil
		}
		return err
	}

	g.rate = rates.GetCore()
	return nil
}

// GetRepoInfo returns a repository information
func (g *GithubDownloaderV3) GetRepoInfo() (*base.Repository, error) {
	g.sleep()
	gr, resp, err := g.client.Repositories.Get(g.ctx, g.repoOwner, g.repoName)
	if err != nil {
		return nil, err
	}
	g.rate = &resp.Rate

	defaultBranch := ""
	if gr.DefaultBranch != nil {
		defaultBranch = *gr.DefaultBranch
	}

	// convert github repo to stand Repo
	return &base.Repository{
		Owner:         g.repoOwner,
		Name:          gr.GetName(),
		IsPrivate:     *gr.Private,
		Description:   gr.GetDescription(),
		OriginalURL:   gr.GetHTMLURL(),
		CloneURL:      gr.GetCloneURL(),
		DefaultBranch: defaultBranch,
	}, nil
}

// GetTopics return github topics
func (g *GithubDownloaderV3) GetTopics() ([]string, error) {
	g.sleep()
	r, resp, err := g.client.Repositories.Get(g.ctx, g.repoOwner, g.repoName)
	if err != nil {
		return nil, err
	}
	g.rate = &resp.Rate
	return r.Topics, nil
}

// GetMilestones returns milestones
func (g *GithubDownloaderV3) GetMilestones() ([]*base.Milestone, error) {
	var perPage = g.maxPerPage
	var milestones = make([]*base.Milestone, 0, perPage)
	for i := 1; ; i++ {
		g.sleep()
		ms, resp, err := g.client.Issues.ListMilestones(g.ctx, g.repoOwner, g.repoName,
			&github.MilestoneListOptions{
				State: "all",
				ListOptions: github.ListOptions{
					Page:    i,
					PerPage: perPage,
				}})
		if err != nil {
			return nil, err
		}
		g.rate = &resp.Rate

		for _, m := range ms {
			var desc string
			if m.Description != nil {
				desc = *m.Description
			}
			var state = "open"
			if m.State != nil {
				state = *m.State
			}
			milestones = append(milestones, &base.Milestone{
				Title:       *m.Title,
				Description: desc,
				Deadline:    m.DueOn,
				State:       state,
				Created:     *m.CreatedAt,
				Updated:     m.UpdatedAt,
				Closed:      m.ClosedAt,
			})
		}
		if len(ms) < perPage {
			break
		}
	}
	return milestones, nil
}

func convertGithubLabel(label *github.Label) *base.Label {
	var desc string
	if label.Description != nil {
		desc = *label.Description
	}
	return &base.Label{
		Name:        *label.Name,
		Color:       *label.Color,
		Description: desc,
	}
}

// GetLabels returns labels
func (g *GithubDownloaderV3) GetLabels() ([]*base.Label, error) {
	var perPage = g.maxPerPage
	var labels = make([]*base.Label, 0, perPage)
	for i := 1; ; i++ {
		g.sleep()
		ls, resp, err := g.client.Issues.ListLabels(g.ctx, g.repoOwner, g.repoName,
			&github.ListOptions{
				Page:    i,
				PerPage: perPage,
			})
		if err != nil {
			return nil, err
		}
		g.rate = &resp.Rate

		for _, label := range ls {
			labels = append(labels, convertGithubLabel(label))
		}
		if len(ls) < perPage {
			break
		}
	}
	return labels, nil
}

func (g *GithubDownloaderV3) convertGithubRelease(rel *github.RepositoryRelease) *base.Release {
	r := &base.Release{
		TagName:         *rel.TagName,
		TargetCommitish: *rel.TargetCommitish,
		Draft:           *rel.Draft,
		Prerelease:      *rel.Prerelease,
		Created:         rel.CreatedAt.Time,
		PublisherID:     *rel.Author.ID,
		PublisherName:   *rel.Author.Login,
	}

	if rel.Body != nil {
		r.Body = *rel.Body
	}
	if rel.Name != nil {
		r.Name = *rel.Name
	}

	if rel.Author.Email != nil {
		r.PublisherEmail = *rel.Author.Email
	}

	if rel.PublishedAt != nil {
		r.Published = rel.PublishedAt.Time
	}

	for _, asset := range rel.Assets {
		var assetID = *asset.ID // Don't optimize this, for closure we need a local variable
		r.Assets = append(r.Assets, &base.ReleaseAsset{
			ID:            *asset.ID,
			Name:          *asset.Name,
			ContentType:   asset.ContentType,
			Size:          asset.Size,
			DownloadCount: asset.DownloadCount,
			Created:       asset.CreatedAt.Time,
			Updated:       asset.UpdatedAt.Time,
			DownloadFunc: func() (io.ReadCloser, error) {
				g.sleep()
				asset, redirectURL, err := g.client.Repositories.DownloadReleaseAsset(g.ctx, g.repoOwner, g.repoName, assetID, nil)
				if err != nil {
					return nil, err
				}
				if err := g.RefreshRate(); err != nil {
					log.Error("g.client.RateLimits: %s", err)
				}
				if asset == nil {
					if redirectURL != "" {
						g.sleep()
						req, err := http.NewRequestWithContext(g.ctx, "GET", redirectURL, nil)
						if err != nil {
							return nil, err
						}
						resp, err := http.DefaultClient.Do(req)
						err1 := g.RefreshRate()
						if err1 != nil {
							log.Error("g.client.RateLimits: %s", err1)
						}
						if err != nil {
							return nil, err
						}
						return resp.Body, nil
					}
					return nil, fmt.Errorf("No release asset found for %d", assetID)
				}
				return asset, nil
			},
		})
	}
	return r
}

// GetReleases returns releases
func (g *GithubDownloaderV3) GetReleases() ([]*base.Release, error) {
	var perPage = g.maxPerPage
	var releases = make([]*base.Release, 0, perPage)
	for i := 1; ; i++ {
		g.sleep()
		ls, resp, err := g.client.Repositories.ListReleases(g.ctx, g.repoOwner, g.repoName,
			&github.ListOptions{
				Page:    i,
				PerPage: perPage,
			})
		if err != nil {
			return nil, err
		}
		g.rate = &resp.Rate

		for _, release := range ls {
			releases = append(releases, g.convertGithubRelease(release))
		}
		if len(ls) < perPage {
			break
		}
	}
	return releases, nil
}

// GetIssues returns issues according start and limit
func (g *GithubDownloaderV3) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	opt := &github.IssueListByRepoOptions{
		Sort:      "created",
		Direction: "asc",
		State:     "all",
		ListOptions: github.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}

	var allIssues = make([]*base.Issue, 0, perPage)
	g.sleep()
	issues, resp, err := g.client.Issues.ListByRepo(g.ctx, g.repoOwner, g.repoName, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %v", err)
	}
	log.Trace("Request get issues %d/%d, but in fact get %d", perPage, page, len(issues))
	g.rate = &resp.Rate
	for _, issue := range issues {
		if issue.IsPullRequest() {
			continue
		}
		var body string
		if issue.Body != nil {
			body = *issue.Body
		}
		var milestone string
		if issue.Milestone != nil {
			milestone = *issue.Milestone.Title
		}
		var labels = make([]*base.Label, 0, len(issue.Labels))
		for _, l := range issue.Labels {
			labels = append(labels, convertGithubLabel(l))
		}

		var email string
		if issue.User.Email != nil {
			email = *issue.User.Email
		}

		// get reactions
		var reactions []*base.Reaction
		for i := 1; ; i++ {
			g.sleep()
			res, resp, err := g.client.Reactions.ListIssueReactions(g.ctx, g.repoOwner, g.repoName, issue.GetNumber(), &github.ListOptions{
				Page:    i,
				PerPage: perPage,
			})
			if err != nil {
				return nil, false, err
			}
			g.rate = &resp.Rate
			if len(res) == 0 {
				break
			}
			for _, reaction := range res {
				reactions = append(reactions, &base.Reaction{
					UserID:   reaction.User.GetID(),
					UserName: reaction.User.GetLogin(),
					Content:  reaction.GetContent(),
				})
			}
		}

		allIssues = append(allIssues, &base.Issue{
			Title:       *issue.Title,
			Number:      int64(*issue.Number),
			PosterID:    *issue.User.ID,
			PosterName:  *issue.User.Login,
			PosterEmail: email,
			Content:     body,
			Milestone:   milestone,
			State:       *issue.State,
			Created:     *issue.CreatedAt,
			Updated:     *issue.UpdatedAt,
			Labels:      labels,
			Reactions:   reactions,
			Closed:      issue.ClosedAt,
			IsLocked:    *issue.Locked,
		})
	}

	return allIssues, len(issues) < perPage, nil
}

// SupportGetRepoComments return true if it supports get repo comments
func (g *GithubDownloaderV3) SupportGetRepoComments() bool {
	return true
}

// GetComments returns comments according issueNumber
func (g *GithubDownloaderV3) GetComments(opts base.GetCommentOptions) ([]*base.Comment, bool, error) {
	if opts.IssueNumber > 0 {
		comments, err := g.getComments(opts.IssueNumber)
		return comments, false, err
	}

	return g.GetAllComments(opts.Page, opts.PageSize)
}

func (g *GithubDownloaderV3) getComments(issueNumber int64) ([]*base.Comment, error) {
	var (
		allComments = make([]*base.Comment, 0, g.maxPerPage)
		created     = "created"
		asc         = "asc"
	)
	opt := &github.IssueListCommentsOptions{
		Sort:      &created,
		Direction: &asc,
		ListOptions: github.ListOptions{
			PerPage: g.maxPerPage,
		},
	}
	for {
		g.sleep()
		comments, resp, err := g.client.Issues.ListComments(g.ctx, g.repoOwner, g.repoName, int(issueNumber), opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %v", err)
		}
		g.rate = &resp.Rate
		for _, comment := range comments {
			var email string
			if comment.User.Email != nil {
				email = *comment.User.Email
			}

			// get reactions
			var reactions []*base.Reaction
			for i := 1; ; i++ {
				g.sleep()
				res, resp, err := g.client.Reactions.ListIssueCommentReactions(g.ctx, g.repoOwner, g.repoName, comment.GetID(), &github.ListOptions{
					Page:    i,
					PerPage: g.maxPerPage,
				})
				if err != nil {
					return nil, err
				}
				g.rate = &resp.Rate
				if len(res) == 0 {
					break
				}
				for _, reaction := range res {
					reactions = append(reactions, &base.Reaction{
						UserID:   reaction.User.GetID(),
						UserName: reaction.User.GetLogin(),
						Content:  reaction.GetContent(),
					})
				}
			}
			allComments = append(allComments, &base.Comment{
				IssueIndex:  issueNumber,
				PosterID:    *comment.User.ID,
				PosterName:  *comment.User.Login,
				PosterEmail: email,
				Content:     *comment.Body,
				Created:     *comment.CreatedAt,
				Updated:     *comment.UpdatedAt,
				Reactions:   reactions,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allComments, nil
}

// GetAllComments returns repository comments according page and perPageSize
func (g *GithubDownloaderV3) GetAllComments(page, perPage int) ([]*base.Comment, bool, error) {
	var (
		allComments = make([]*base.Comment, 0, perPage)
		created     = "created"
		asc         = "asc"
	)
	opt := &github.IssueListCommentsOptions{
		Sort:      &created,
		Direction: &asc,
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: perPage,
		},
	}

	g.sleep()
	comments, resp, err := g.client.Issues.ListComments(g.ctx, g.repoOwner, g.repoName, 0, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %v", err)
	}
	log.Trace("Request get comments %d/%d, but in fact get %d", perPage, page, len(comments))
	g.rate = &resp.Rate
	for _, comment := range comments {
		var email string
		if comment.User.Email != nil {
			email = *comment.User.Email
		}

		// get reactions
		var reactions []*base.Reaction
		for i := 1; ; i++ {
			g.sleep()
			res, resp, err := g.client.Reactions.ListIssueCommentReactions(g.ctx, g.repoOwner, g.repoName, comment.GetID(), &github.ListOptions{
				Page:    i,
				PerPage: g.maxPerPage,
			})
			if err != nil {
				return nil, false, err
			}
			g.rate = &resp.Rate
			if len(res) == 0 {
				break
			}
			for _, reaction := range res {
				reactions = append(reactions, &base.Reaction{
					UserID:   reaction.User.GetID(),
					UserName: reaction.User.GetLogin(),
					Content:  reaction.GetContent(),
				})
			}
		}
		idx := strings.LastIndex(*comment.IssueURL, "/")
		issueIndex, _ := strconv.ParseInt((*comment.IssueURL)[idx+1:], 10, 64)
		allComments = append(allComments, &base.Comment{
			IssueIndex:  issueIndex,
			PosterID:    *comment.User.ID,
			PosterName:  *comment.User.Login,
			PosterEmail: email,
			Content:     *comment.Body,
			Created:     *comment.CreatedAt,
			Updated:     *comment.UpdatedAt,
			Reactions:   reactions,
		})
	}

	return allComments, len(allComments) < perPage, nil
}

// GetPullRequests returns pull requests according page and perPage
func (g *GithubDownloaderV3) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	opt := &github.PullRequestListOptions{
		Sort:      "created",
		Direction: "asc",
		State:     "all",
		ListOptions: github.ListOptions{
			PerPage: perPage,
			Page:    page,
		},
	}
	var allPRs = make([]*base.PullRequest, 0, perPage)
	g.sleep()
	prs, resp, err := g.client.PullRequests.List(g.ctx, g.repoOwner, g.repoName, opt)
	if err != nil {
		return nil, false, fmt.Errorf("error while listing repos: %v", err)
	}
	log.Trace("Request get pull requests %d/%d, but in fact get %d", perPage, page, len(prs))
	g.rate = &resp.Rate
	for _, pr := range prs {
		var body string
		if pr.Body != nil {
			body = *pr.Body
		}
		var milestone string
		if pr.Milestone != nil {
			milestone = *pr.Milestone.Title
		}
		var labels = make([]*base.Label, 0, len(pr.Labels))
		for _, l := range pr.Labels {
			labels = append(labels, convertGithubLabel(l))
		}

		var email string
		if pr.User.Email != nil {
			email = *pr.User.Email
		}
		var merged bool
		// pr.Merged is not valid, so use MergedAt to test if it's merged
		if pr.MergedAt != nil {
			merged = true
		}

		var (
			headRepoName string
			cloneURL     string
			headRef      string
			headSHA      string
		)
		if pr.Head.Repo != nil {
			if pr.Head.Repo.Name != nil {
				headRepoName = *pr.Head.Repo.Name
			}
			if pr.Head.Repo.CloneURL != nil {
				cloneURL = *pr.Head.Repo.CloneURL
			}
		}
		if pr.Head.Ref != nil {
			headRef = *pr.Head.Ref
		}
		if pr.Head.SHA != nil {
			headSHA = *pr.Head.SHA
		}
		var mergeCommitSHA string
		if pr.MergeCommitSHA != nil {
			mergeCommitSHA = *pr.MergeCommitSHA
		}

		var headUserName string
		if pr.Head.User != nil && pr.Head.User.Login != nil {
			headUserName = *pr.Head.User.Login
		}

		// get reactions
		var reactions []*base.Reaction
		for i := 1; ; i++ {
			g.sleep()
			res, resp, err := g.client.Reactions.ListIssueReactions(g.ctx, g.repoOwner, g.repoName, pr.GetNumber(), &github.ListOptions{
				Page:    i,
				PerPage: perPage,
			})
			if err != nil {
				return nil, false, err
			}
			g.rate = &resp.Rate
			if len(res) == 0 {
				break
			}
			for _, reaction := range res {
				reactions = append(reactions, &base.Reaction{
					UserID:   reaction.User.GetID(),
					UserName: reaction.User.GetLogin(),
					Content:  reaction.GetContent(),
				})
			}
		}

		allPRs = append(allPRs, &base.PullRequest{
			Title:          *pr.Title,
			Number:         int64(*pr.Number),
			PosterName:     *pr.User.Login,
			PosterID:       *pr.User.ID,
			PosterEmail:    email,
			Content:        body,
			Milestone:      milestone,
			State:          *pr.State,
			Created:        *pr.CreatedAt,
			Updated:        *pr.UpdatedAt,
			Closed:         pr.ClosedAt,
			Labels:         labels,
			Merged:         merged,
			MergeCommitSHA: mergeCommitSHA,
			MergedTime:     pr.MergedAt,
			IsLocked:       pr.ActiveLockReason != nil,
			Head: base.PullRequestBranch{
				Ref:       headRef,
				SHA:       headSHA,
				RepoName:  headRepoName,
				OwnerName: headUserName,
				CloneURL:  cloneURL,
			},
			Base: base.PullRequestBranch{
				Ref:       *pr.Base.Ref,
				SHA:       *pr.Base.SHA,
				RepoName:  *pr.Base.Repo.Name,
				OwnerName: *pr.Base.User.Login,
			},
			PatchURL:  *pr.PatchURL,
			Reactions: reactions,
		})
	}

	return allPRs, len(prs) < perPage, nil
}

func convertGithubReview(r *github.PullRequestReview) *base.Review {
	return &base.Review{
		ID:           r.GetID(),
		ReviewerID:   r.GetUser().GetID(),
		ReviewerName: r.GetUser().GetLogin(),
		CommitID:     r.GetCommitID(),
		Content:      r.GetBody(),
		CreatedAt:    r.GetSubmittedAt(),
		State:        r.GetState(),
	}
}

func (g *GithubDownloaderV3) convertGithubReviewComments(cs []*github.PullRequestComment) ([]*base.ReviewComment, error) {
	var rcs = make([]*base.ReviewComment, 0, len(cs))
	for _, c := range cs {
		// get reactions
		var reactions []*base.Reaction
		for i := 1; ; i++ {
			g.sleep()
			res, resp, err := g.client.Reactions.ListPullRequestCommentReactions(g.ctx, g.repoOwner, g.repoName, c.GetID(), &github.ListOptions{
				Page:    i,
				PerPage: g.maxPerPage,
			})
			if err != nil {
				return nil, err
			}
			g.rate = &resp.Rate
			if len(res) == 0 {
				break
			}
			for _, reaction := range res {
				reactions = append(reactions, &base.Reaction{
					UserID:   reaction.User.GetID(),
					UserName: reaction.User.GetLogin(),
					Content:  reaction.GetContent(),
				})
			}
		}

		rcs = append(rcs, &base.ReviewComment{
			ID:        c.GetID(),
			InReplyTo: c.GetInReplyTo(),
			Content:   c.GetBody(),
			TreePath:  c.GetPath(),
			DiffHunk:  c.GetDiffHunk(),
			Position:  c.GetPosition(),
			CommitID:  c.GetCommitID(),
			PosterID:  c.GetUser().GetID(),
			Reactions: reactions,
			CreatedAt: c.GetCreatedAt(),
			UpdatedAt: c.GetUpdatedAt(),
		})
	}
	return rcs, nil
}

// GetReviews returns pull requests review
func (g *GithubDownloaderV3) GetReviews(pullRequestNumber int64) ([]*base.Review, error) {
	var allReviews = make([]*base.Review, 0, g.maxPerPage)
	opt := &github.ListOptions{
		PerPage: g.maxPerPage,
	}
	for {
		g.sleep()
		reviews, resp, err := g.client.PullRequests.ListReviews(g.ctx, g.repoOwner, g.repoName, int(pullRequestNumber), opt)
		if err != nil {
			return nil, fmt.Errorf("error while listing repos: %v", err)
		}
		g.rate = &resp.Rate
		for _, review := range reviews {
			r := convertGithubReview(review)
			r.IssueIndex = pullRequestNumber
			// retrieve all review comments
			opt2 := &github.ListOptions{
				PerPage: g.maxPerPage,
			}
			for {
				g.sleep()
				reviewComments, resp, err := g.client.PullRequests.ListReviewComments(g.ctx, g.repoOwner, g.repoName, int(pullRequestNumber), review.GetID(), opt2)
				if err != nil {
					return nil, fmt.Errorf("error while listing repos: %v", err)
				}
				g.rate = &resp.Rate

				cs, err := g.convertGithubReviewComments(reviewComments)
				if err != nil {
					return nil, err
				}
				r.Comments = append(r.Comments, cs...)
				if resp.NextPage == 0 {
					break
				}
				opt2.Page = resp.NextPage
			}
			allReviews = append(allReviews, r)
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return allReviews, nil
}
