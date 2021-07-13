// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/migrations/base"
	"code.gitea.io/gitea/modules/structs"

	gitea_sdk "code.gitea.io/sdk/gitea"
)

var (
	_ base.Downloader        = &GiteaDownloader{}
	_ base.DownloaderFactory = &GiteaDownloaderFactory{}
)

func init() {
	RegisterDownloaderFactory(&GiteaDownloaderFactory{})
}

// GiteaDownloaderFactory defines a gitea downloader factory
type GiteaDownloaderFactory struct {
}

// New returns a Downloader related to this factory according MigrateOptions
func (f *GiteaDownloaderFactory) New(ctx context.Context, opts base.MigrateOptions) (base.Downloader, error) {
	u, err := url.Parse(opts.CloneAddr)
	if err != nil {
		return nil, err
	}

	baseURL := u.Scheme + "://" + u.Host
	repoNameSpace := strings.TrimPrefix(u.Path, "/")
	repoNameSpace = strings.TrimSuffix(repoNameSpace, ".git")

	path := strings.Split(repoNameSpace, "/")
	if len(path) < 2 {
		return nil, fmt.Errorf("invalid path: %s", repoNameSpace)
	}

	repoPath := strings.Join(path[len(path)-2:], "/")
	if len(path) > 2 {
		subPath := strings.Join(path[:len(path)-2], "/")
		baseURL += "/" + subPath
	}

	log.Trace("Create gitea downloader. BaseURL: %s RepoName: %s", baseURL, repoNameSpace)

	return NewGiteaDownloader(ctx, baseURL, repoPath, opts.AuthUsername, opts.AuthPassword, opts.AuthToken)
}

// GitServiceType returns the type of git service
func (f *GiteaDownloaderFactory) GitServiceType() structs.GitServiceType {
	return structs.GiteaService
}

// GiteaDownloader implements a Downloader interface to get repository information's
type GiteaDownloader struct {
	base.NullDownloader
	ctx        context.Context
	client     *gitea_sdk.Client
	repoOwner  string
	repoName   string
	pagination bool
	maxPerPage int
}

// NewGiteaDownloader creates a gitea Downloader via gitea API
//   Use either a username/password or personal token. token is preferred
//   Note: Public access only allows very basic access
func NewGiteaDownloader(ctx context.Context, baseURL, repoPath, username, password, token string) (*GiteaDownloader, error) {
	giteaClient, err := gitea_sdk.NewClient(
		baseURL,
		gitea_sdk.SetToken(token),
		gitea_sdk.SetBasicAuth(username, password),
		gitea_sdk.SetContext(ctx),
	)
	if err != nil {
		log.Error(fmt.Sprintf("Failed to create NewGiteaDownloader for: %s. Error: %v", baseURL, err))
		return nil, err
	}

	path := strings.Split(repoPath, "/")

	paginationSupport := true
	if err = giteaClient.CheckServerVersionConstraint(">=1.12"); err != nil {
		paginationSupport = false
	}

	// set small maxPerPage since we can only guess
	// (default would be 50 but this can differ)
	maxPerPage := 10
	// gitea instances >=1.13 can tell us what maximum they have
	apiConf, _, err := giteaClient.GetGlobalAPISettings()
	if err != nil {
		log.Info("Unable to get global API settings. Ignoring these.")
		log.Debug("giteaClient.GetGlobalAPISettings. Error: %v", err)
	}
	if apiConf != nil {
		maxPerPage = apiConf.MaxResponseItems
	}

	return &GiteaDownloader{
		ctx:        ctx,
		client:     giteaClient,
		repoOwner:  path[0],
		repoName:   path[1],
		pagination: paginationSupport,
		maxPerPage: maxPerPage,
	}, nil
}

// SetContext set context
func (g *GiteaDownloader) SetContext(ctx context.Context) {
	g.ctx = ctx
}

// GetRepoInfo returns a repository information
func (g *GiteaDownloader) GetRepoInfo() (*base.Repository, error) {
	if g == nil {
		return nil, errors.New("error: GiteaDownloader is nil")
	}

	repo, _, err := g.client.GetRepo(g.repoOwner, g.repoName)
	if err != nil {
		return nil, err
	}

	return &base.Repository{
		Name:          repo.Name,
		Owner:         repo.Owner.UserName,
		IsPrivate:     repo.Private,
		Description:   repo.Description,
		CloneURL:      repo.CloneURL,
		OriginalURL:   repo.HTMLURL,
		DefaultBranch: repo.DefaultBranch,
	}, nil
}

// GetTopics return gitea topics
func (g *GiteaDownloader) GetTopics() ([]string, error) {
	topics, _, err := g.client.ListRepoTopics(g.repoOwner, g.repoName, gitea_sdk.ListRepoTopicsOptions{})
	return topics, err
}

// GetMilestones returns milestones
func (g *GiteaDownloader) GetMilestones() ([]*base.Milestone, error) {
	var milestones = make([]*base.Milestone, 0, g.maxPerPage)

	for i := 1; ; i++ {
		// make sure gitea can shutdown gracefully
		select {
		case <-g.ctx.Done():
			return nil, nil
		default:
		}

		ms, _, err := g.client.ListRepoMilestones(g.repoOwner, g.repoName, gitea_sdk.ListMilestoneOption{
			ListOptions: gitea_sdk.ListOptions{
				PageSize: g.maxPerPage,
				Page:     i,
			},
			State: gitea_sdk.StateAll,
		})
		if err != nil {
			return nil, err
		}

		for i := range ms {
			// old gitea instances dont have this information
			createdAT := time.Now()
			var updatedAT *time.Time
			if ms[i].Closed != nil {
				createdAT = *ms[i].Closed
				updatedAT = ms[i].Closed
			}

			// new gitea instances (>=1.13) do
			if !ms[i].Created.IsZero() {
				createdAT = ms[i].Created
			}
			if ms[i].Updated != nil && !ms[i].Updated.IsZero() {
				updatedAT = ms[i].Updated
			}

			milestones = append(milestones, &base.Milestone{
				Title:       ms[i].Title,
				Description: ms[i].Description,
				Deadline:    ms[i].Deadline,
				Created:     createdAT,
				Updated:     updatedAT,
				Closed:      ms[i].Closed,
				State:       string(ms[i].State),
			})
		}
		if !g.pagination || len(ms) < g.maxPerPage {
			break
		}
	}
	return milestones, nil
}

func (g *GiteaDownloader) convertGiteaLabel(label *gitea_sdk.Label) *base.Label {
	return &base.Label{
		Name:        label.Name,
		Color:       label.Color,
		Description: label.Description,
	}
}

// GetLabels returns labels
func (g *GiteaDownloader) GetLabels() ([]*base.Label, error) {
	var labels = make([]*base.Label, 0, g.maxPerPage)

	for i := 1; ; i++ {
		// make sure gitea can shutdown gracefully
		select {
		case <-g.ctx.Done():
			return nil, nil
		default:
		}

		ls, _, err := g.client.ListRepoLabels(g.repoOwner, g.repoName, gitea_sdk.ListLabelsOptions{ListOptions: gitea_sdk.ListOptions{
			PageSize: g.maxPerPage,
			Page:     i,
		}})
		if err != nil {
			return nil, err
		}

		for i := range ls {
			labels = append(labels, g.convertGiteaLabel(ls[i]))
		}
		if !g.pagination || len(ls) < g.maxPerPage {
			break
		}
	}
	return labels, nil
}

func (g *GiteaDownloader) convertGiteaRelease(rel *gitea_sdk.Release) *base.Release {
	r := &base.Release{
		TagName:         rel.TagName,
		TargetCommitish: rel.Target,
		Name:            rel.Title,
		Body:            rel.Note,
		Draft:           rel.IsDraft,
		Prerelease:      rel.IsPrerelease,
		PublisherID:     rel.Publisher.ID,
		PublisherName:   rel.Publisher.UserName,
		PublisherEmail:  rel.Publisher.Email,
		Published:       rel.PublishedAt,
		Created:         rel.CreatedAt,
	}

	for _, asset := range rel.Attachments {
		size := int(asset.Size)
		dlCount := int(asset.DownloadCount)
		r.Assets = append(r.Assets, &base.ReleaseAsset{
			ID:            asset.ID,
			Name:          asset.Name,
			Size:          &size,
			DownloadCount: &dlCount,
			Created:       asset.Created,
			DownloadURL:   &asset.DownloadURL,
			DownloadFunc: func() (io.ReadCloser, error) {
				asset, _, err := g.client.GetReleaseAttachment(g.repoOwner, g.repoName, rel.ID, asset.ID)
				if err != nil {
					return nil, err
				}
				// FIXME: for a private download?
				resp, err := http.Get(asset.DownloadURL)
				if err != nil {
					return nil, err
				}

				// resp.Body is closed by the uploader
				return resp.Body, nil
			},
		})
	}
	return r
}

// GetReleases returns releases
func (g *GiteaDownloader) GetReleases() ([]*base.Release, error) {
	var releases = make([]*base.Release, 0, g.maxPerPage)

	for i := 1; ; i++ {
		// make sure gitea can shutdown gracefully
		select {
		case <-g.ctx.Done():
			return nil, nil
		default:
		}

		rl, _, err := g.client.ListReleases(g.repoOwner, g.repoName, gitea_sdk.ListReleasesOptions{ListOptions: gitea_sdk.ListOptions{
			PageSize: g.maxPerPage,
			Page:     i,
		}})
		if err != nil {
			return nil, err
		}

		for i := range rl {
			releases = append(releases, g.convertGiteaRelease(rl[i]))
		}
		if !g.pagination || len(rl) < g.maxPerPage {
			break
		}
	}
	return releases, nil
}

func (g *GiteaDownloader) getIssueReactions(index int64) ([]*base.Reaction, error) {
	var reactions []*base.Reaction
	if err := g.client.CheckServerVersionConstraint(">=1.11"); err != nil {
		log.Info("GiteaDownloader: instance to old, skip getIssueReactions")
		return reactions, nil
	}
	rl, _, err := g.client.GetIssueReactions(g.repoOwner, g.repoName, index)
	if err != nil {
		return nil, err
	}

	for _, reaction := range rl {
		reactions = append(reactions, &base.Reaction{
			UserID:   reaction.User.ID,
			UserName: reaction.User.UserName,
			Content:  reaction.Reaction,
		})
	}
	return reactions, nil
}

func (g *GiteaDownloader) getCommentReactions(commentID int64) ([]*base.Reaction, error) {
	var reactions []*base.Reaction
	if err := g.client.CheckServerVersionConstraint(">=1.11"); err != nil {
		log.Info("GiteaDownloader: instance to old, skip getCommentReactions")
		return reactions, nil
	}
	rl, _, err := g.client.GetIssueCommentReactions(g.repoOwner, g.repoName, commentID)
	if err != nil {
		return nil, err
	}

	for i := range rl {
		reactions = append(reactions, &base.Reaction{
			UserID:   rl[i].User.ID,
			UserName: rl[i].User.UserName,
			Content:  rl[i].Reaction,
		})
	}
	return reactions, nil
}

// GetIssues returns issues according start and limit
func (g *GiteaDownloader) GetIssues(page, perPage int) ([]*base.Issue, bool, error) {
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	var allIssues = make([]*base.Issue, 0, perPage)

	issues, _, err := g.client.ListRepoIssues(g.repoOwner, g.repoName, gitea_sdk.ListIssueOption{
		ListOptions: gitea_sdk.ListOptions{Page: page, PageSize: perPage},
		State:       gitea_sdk.StateAll,
		Type:        gitea_sdk.IssueTypeIssue,
	})
	if err != nil {
		return nil, false, fmt.Errorf("error while listing issues: %v", err)
	}
	for _, issue := range issues {

		var labels = make([]*base.Label, 0, len(issue.Labels))
		for i := range issue.Labels {
			labels = append(labels, g.convertGiteaLabel(issue.Labels[i]))
		}

		var milestone string
		if issue.Milestone != nil {
			milestone = issue.Milestone.Title
		}

		reactions, err := g.getIssueReactions(issue.Index)
		if err != nil {
			log.Warn("Unable to load reactions during migrating issue #%d to %s/%s. Error: %v", issue.Index, g.repoOwner, g.repoName, err)
			if err2 := models.CreateRepositoryNotice(
				fmt.Sprintf("Unable to load reactions during migrating issue #%d to %s/%s. Error: %v", issue.Index, g.repoOwner, g.repoName, err)); err2 != nil {
				log.Error("create repository notice failed: ", err2)
			}
		}

		var assignees []string
		for i := range issue.Assignees {
			assignees = append(assignees, issue.Assignees[i].UserName)
		}

		allIssues = append(allIssues, &base.Issue{
			Title:       issue.Title,
			Number:      issue.Index,
			PosterID:    issue.Poster.ID,
			PosterName:  issue.Poster.UserName,
			PosterEmail: issue.Poster.Email,
			Content:     issue.Body,
			Milestone:   milestone,
			State:       string(issue.State),
			Created:     issue.Created,
			Updated:     issue.Updated,
			Closed:      issue.Closed,
			Reactions:   reactions,
			Labels:      labels,
			Assignees:   assignees,
			IsLocked:    issue.IsLocked,
		})
	}

	isEnd := len(issues) < perPage
	if !g.pagination {
		isEnd = len(issues) == 0
	}
	return allIssues, isEnd, nil
}

// GetComments returns comments according issueNumber
func (g *GiteaDownloader) GetComments(opts base.GetCommentOptions) ([]*base.Comment, bool, error) {
	var allComments = make([]*base.Comment, 0, g.maxPerPage)

	// for i := 1; ; i++ {
	// make sure gitea can shutdown gracefully
	select {
	case <-g.ctx.Done():
		return nil, false, nil
	default:
	}

	comments, _, err := g.client.ListIssueComments(g.repoOwner, g.repoName, opts.IssueNumber, gitea_sdk.ListIssueCommentOptions{ListOptions: gitea_sdk.ListOptions{
		// PageSize: g.maxPerPage,
		// Page:     i,
	}})
	if err != nil {
		return nil, false, fmt.Errorf("error while listing comments for issue #%d. Error: %v", opts.IssueNumber, err)
	}

	for _, comment := range comments {
		reactions, err := g.getCommentReactions(comment.ID)
		if err != nil {
			log.Warn("Unable to load comment reactions during migrating issue #%d for comment %d to %s/%s. Error: %v", opts.IssueNumber, comment.ID, g.repoOwner, g.repoName, err)
			if err2 := models.CreateRepositoryNotice(
				fmt.Sprintf("Unable to load reactions during migrating issue #%d for comment %d to %s/%s. Error: %v", opts.IssueNumber, comment.ID, g.repoOwner, g.repoName, err)); err2 != nil {
				log.Error("create repository notice failed: ", err2)
			}
		}

		allComments = append(allComments, &base.Comment{
			IssueIndex:  opts.IssueNumber,
			PosterID:    comment.Poster.ID,
			PosterName:  comment.Poster.UserName,
			PosterEmail: comment.Poster.Email,
			Content:     comment.Body,
			Created:     comment.Created,
			Updated:     comment.Updated,
			Reactions:   reactions,
		})
	}

	// TODO enable pagination vor (gitea >= 1.14) when it got implemented
	// 	if !g.pagination || len(comments) < g.maxPerPage {
	//		break
	//	}
	//}
	return allComments, true, nil
}

// GetPullRequests returns pull requests according page and perPage
func (g *GiteaDownloader) GetPullRequests(page, perPage int) ([]*base.PullRequest, bool, error) {
	if perPage > g.maxPerPage {
		perPage = g.maxPerPage
	}
	var allPRs = make([]*base.PullRequest, 0, perPage)

	prs, _, err := g.client.ListRepoPullRequests(g.repoOwner, g.repoName, gitea_sdk.ListPullRequestsOptions{
		ListOptions: gitea_sdk.ListOptions{
			Page:     page,
			PageSize: perPage,
		},
		State: gitea_sdk.StateAll,
	})
	if err != nil {
		return nil, false, fmt.Errorf("error while listing pull requests (page: %d, pagesize: %d). Error: %v", page, perPage, err)
	}
	for _, pr := range prs {
		var milestone string
		if pr.Milestone != nil {
			milestone = pr.Milestone.Title
		}

		var labels = make([]*base.Label, 0, len(pr.Labels))
		for i := range pr.Labels {
			labels = append(labels, g.convertGiteaLabel(pr.Labels[i]))
		}

		var (
			headUserName string
			headRepoName string
			headCloneURL string
			headRef      string
			headSHA      string
		)
		if pr.Head != nil {
			if pr.Head.Repository != nil {
				headUserName = pr.Head.Repository.Owner.UserName
				headRepoName = pr.Head.Repository.Name
				headCloneURL = pr.Head.Repository.CloneURL
			}
			headSHA = pr.Head.Sha
			headRef = pr.Head.Ref
		}

		var mergeCommitSHA string
		if pr.MergedCommitID != nil {
			mergeCommitSHA = *pr.MergedCommitID
		}

		reactions, err := g.getIssueReactions(pr.Index)
		if err != nil {
			log.Warn("Unable to load reactions during migrating pull #%d to %s/%s. Error: %v", pr.Index, g.repoOwner, g.repoName, err)
			if err2 := models.CreateRepositoryNotice(
				fmt.Sprintf("Unable to load reactions during migrating pull #%d to %s/%s. Error: %v", pr.Index, g.repoOwner, g.repoName, err)); err2 != nil {
				log.Error("create repository notice failed: ", err2)
			}
		}

		var assignees []string
		for i := range pr.Assignees {
			assignees = append(assignees, pr.Assignees[i].UserName)
		}

		createdAt := time.Now()
		if pr.Created != nil {
			createdAt = *pr.Created
		}
		updatedAt := time.Now()
		if pr.Created != nil {
			updatedAt = *pr.Updated
		}

		closedAt := pr.Closed
		if pr.Merged != nil && closedAt == nil {
			closedAt = pr.Merged
		}

		allPRs = append(allPRs, &base.PullRequest{
			Title:          pr.Title,
			Number:         pr.Index,
			PosterID:       pr.Poster.ID,
			PosterName:     pr.Poster.UserName,
			PosterEmail:    pr.Poster.Email,
			Content:        pr.Body,
			State:          string(pr.State),
			Created:        createdAt,
			Updated:        updatedAt,
			Closed:         closedAt,
			Labels:         labels,
			Milestone:      milestone,
			Reactions:      reactions,
			Assignees:      assignees,
			Merged:         pr.HasMerged,
			MergedTime:     pr.Merged,
			MergeCommitSHA: mergeCommitSHA,
			IsLocked:       pr.IsLocked,
			PatchURL:       pr.PatchURL,
			Head: base.PullRequestBranch{
				Ref:       headRef,
				SHA:       headSHA,
				RepoName:  headRepoName,
				OwnerName: headUserName,
				CloneURL:  headCloneURL,
			},
			Base: base.PullRequestBranch{
				Ref:       pr.Base.Ref,
				SHA:       pr.Base.Sha,
				RepoName:  g.repoName,
				OwnerName: g.repoOwner,
			},
		})
	}

	isEnd := len(prs) < perPage
	if !g.pagination {
		isEnd = len(prs) == 0
	}
	return allPRs, isEnd, nil
}

// GetReviews returns pull requests review
func (g *GiteaDownloader) GetReviews(index int64) ([]*base.Review, error) {
	if err := g.client.CheckServerVersionConstraint(">=1.12"); err != nil {
		log.Info("GiteaDownloader: instance to old, skip GetReviews")
		return nil, nil
	}

	var allReviews = make([]*base.Review, 0, g.maxPerPage)

	for i := 1; ; i++ {
		// make sure gitea can shutdown gracefully
		select {
		case <-g.ctx.Done():
			return nil, nil
		default:
		}

		prl, _, err := g.client.ListPullReviews(g.repoOwner, g.repoName, index, gitea_sdk.ListPullReviewsOptions{ListOptions: gitea_sdk.ListOptions{
			Page:     i,
			PageSize: g.maxPerPage,
		}})
		if err != nil {
			return nil, err
		}

		for _, pr := range prl {

			rcl, _, err := g.client.ListPullReviewComments(g.repoOwner, g.repoName, index, pr.ID)
			if err != nil {
				return nil, err
			}
			var reviewComments []*base.ReviewComment
			for i := range rcl {
				line := int(rcl[i].LineNum)
				if rcl[i].OldLineNum > 0 {
					line = int(rcl[i].OldLineNum) * -1
				}

				reviewComments = append(reviewComments, &base.ReviewComment{
					ID:        rcl[i].ID,
					Content:   rcl[i].Body,
					TreePath:  rcl[i].Path,
					DiffHunk:  rcl[i].DiffHunk,
					Line:      line,
					CommitID:  rcl[i].CommitID,
					PosterID:  rcl[i].Reviewer.ID,
					CreatedAt: rcl[i].Created,
					UpdatedAt: rcl[i].Updated,
				})
			}

			allReviews = append(allReviews, &base.Review{
				ID:           pr.ID,
				IssueIndex:   index,
				ReviewerID:   pr.Reviewer.ID,
				ReviewerName: pr.Reviewer.UserName,
				Official:     pr.Official,
				CommitID:     pr.CommitID,
				Content:      pr.Body,
				CreatedAt:    pr.Submitted,
				State:        string(pr.State),
				Comments:     reviewComments,
			})
		}

		if len(prl) < g.maxPerPage {
			break
		}
	}
	return allReviews, nil
}
