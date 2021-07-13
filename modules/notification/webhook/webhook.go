// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/notification/base"
	"code.gitea.io/gitea/modules/repository"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	webhook_services "code.gitea.io/gitea/services/webhook"
)

type webhookNotifier struct {
	base.NullNotifier
}

var (
	_ base.Notifier = &webhookNotifier{}
)

// NewNotifier create a new webhookNotifier notifier
func NewNotifier() base.Notifier {
	return &webhookNotifier{}
}

func (m *webhookNotifier) NotifyIssueClearLabels(doer *models.User, issue *models.Issue) {
	if err := issue.LoadPoster(); err != nil {
		log.Error("loadPoster: %v", err)
		return
	}

	if err := issue.LoadRepo(); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}

		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventPullRequestLabel, &api.PullRequestPayload{
			Action:      api.HookIssueLabelCleared,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(issue.PullRequest),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventIssueLabel, &api.IssuePayload{
			Action:     api.HookIssueLabelCleared,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyForkRepository(doer *models.User, oldRepo, repo *models.Repository) {
	oldMode, _ := models.AccessLevel(doer, oldRepo)
	mode, _ := models.AccessLevel(doer, repo)

	// forked webhook
	if err := webhook_services.PrepareWebhooks(oldRepo, models.HookEventFork, &api.ForkPayload{
		Forkee: convert.ToRepo(oldRepo, oldMode),
		Repo:   convert.ToRepo(repo, mode),
		Sender: convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", oldRepo.ID, err)
	}

	u := repo.MustOwner()

	// Add to hook queue for created repo after session commit.
	if u.IsOrganization() {
		if err := webhook_services.PrepareWebhooks(repo, models.HookEventRepository, &api.RepositoryPayload{
			Action:       api.HookRepoCreated,
			Repository:   convert.ToRepo(repo, models.AccessModeOwner),
			Organization: convert.ToUser(u, nil),
			Sender:       convert.ToUser(doer, nil),
		}); err != nil {
			log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
		}
	}
}

func (m *webhookNotifier) NotifyCreateRepository(doer *models.User, u *models.User, repo *models.Repository) {
	// Add to hook queue for created repo after session commit.
	if err := webhook_services.PrepareWebhooks(repo, models.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoCreated,
		Repository:   convert.ToRepo(repo, models.AccessModeOwner),
		Organization: convert.ToUser(u, nil),
		Sender:       convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyDeleteRepository(doer *models.User, repo *models.Repository) {
	u := repo.MustOwner()

	if err := webhook_services.PrepareWebhooks(repo, models.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoDeleted,
		Repository:   convert.ToRepo(repo, models.AccessModeOwner),
		Organization: convert.ToUser(u, nil),
		Sender:       convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyMigrateRepository(doer *models.User, u *models.User, repo *models.Repository) {
	// Add to hook queue for created repo after session commit.
	if err := webhook_services.PrepareWebhooks(repo, models.HookEventRepository, &api.RepositoryPayload{
		Action:       api.HookRepoCreated,
		Repository:   convert.ToRepo(repo, models.AccessModeOwner),
		Organization: convert.ToUser(u, nil),
		Sender:       convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [repo_id: %d]: %v", repo.ID, err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeAssignee(doer *models.User, issue *models.Issue, assignee *models.User, removed bool, comment *models.Comment) {
	if issue.IsPull {
		mode, _ := models.AccessLevelUnit(doer, issue.Repo, models.UnitTypePullRequests)

		if err := issue.LoadPullRequest(); err != nil {
			log.Error("LoadPullRequest failed: %v", err)
			return
		}
		issue.PullRequest.Issue = issue
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(issue.PullRequest),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		}
		if removed {
			apiPullRequest.Action = api.HookIssueUnassigned
		} else {
			apiPullRequest.Action = api.HookIssueAssigned
		}
		// Assignee comment triggers a webhook
		if err := webhook_services.PrepareWebhooks(issue.Repo, models.HookEventPullRequestAssign, apiPullRequest); err != nil {
			log.Error("PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, removed, err)
			return
		}
	} else {
		mode, _ := models.AccessLevelUnit(doer, issue.Repo, models.UnitTypeIssues)
		apiIssue := &api.IssuePayload{
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		}
		if removed {
			apiIssue.Action = api.HookIssueUnassigned
		} else {
			apiIssue.Action = api.HookIssueAssigned
		}
		// Assignee comment triggers a webhook
		if err := webhook_services.PrepareWebhooks(issue.Repo, models.HookEventIssueAssign, apiIssue); err != nil {
			log.Error("PrepareWebhooks [is_pull: %v, remove_assignee: %v]: %v", issue.IsPull, removed, err)
			return
		}
	}
}

func (m *webhookNotifier) NotifyIssueChangeTitle(doer *models.User, issue *models.Issue, oldTitle string) {
	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error("LoadPullRequest failed: %v", err)
			return
		}
		issue.PullRequest.Issue = issue
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Title: &api.ChangesFromPayload{
					From: oldTitle,
				},
			},
			PullRequest: convert.ToAPIPullRequest(issue.PullRequest),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventIssues, &api.IssuePayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Title: &api.ChangesFromPayload{
					From: oldTitle,
				},
			},
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(issue.Poster, nil),
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeStatus(doer *models.User, issue *models.Issue, actionComment *models.Comment, isClosed bool) {
	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error("LoadPullRequest: %v", err)
			return
		}
		// Merge pull request calls issue.changeStatus so we need to handle separately.
		apiPullRequest := &api.PullRequestPayload{
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(issue.PullRequest),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		}
		if isClosed {
			apiPullRequest.Action = api.HookIssueClosed
		} else {
			apiPullRequest.Action = api.HookIssueReOpened
		}
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, apiPullRequest)
	} else {
		apiIssue := &api.IssuePayload{
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		}
		if isClosed {
			apiIssue.Action = api.HookIssueClosed
		} else {
			apiIssue.Action = api.HookIssueReOpened
		}
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventIssues, apiIssue)
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v, is_closed: %v]: %v", issue.IsPull, isClosed, err)
	}
}

func (m *webhookNotifier) NotifyNewIssue(issue *models.Issue, mentions []*models.User) {
	if err := issue.LoadRepo(); err != nil {
		log.Error("issue.LoadRepo: %v", err)
		return
	}
	if err := issue.LoadPoster(); err != nil {
		log.Error("issue.LoadPoster: %v", err)
		return
	}

	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	if err := webhook_services.PrepareWebhooks(issue.Repo, models.HookEventIssues, &api.IssuePayload{
		Action:     api.HookIssueOpened,
		Index:      issue.Index,
		Issue:      convert.ToAPIIssue(issue),
		Repository: convert.ToRepo(issue.Repo, mode),
		Sender:     convert.ToUser(issue.Poster, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyNewPullRequest(pull *models.PullRequest, mentions []*models.User) {
	if err := pull.LoadIssue(); err != nil {
		log.Error("pull.LoadIssue: %v", err)
		return
	}
	if err := pull.Issue.LoadRepo(); err != nil {
		log.Error("pull.Issue.LoadRepo: %v", err)
		return
	}
	if err := pull.Issue.LoadPoster(); err != nil {
		log.Error("pull.Issue.LoadPoster: %v", err)
		return
	}

	mode, _ := models.AccessLevel(pull.Issue.Poster, pull.Issue.Repo)
	if err := webhook_services.PrepareWebhooks(pull.Issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
		Action:      api.HookIssueOpened,
		Index:       pull.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(pull),
		Repository:  convert.ToRepo(pull.Issue.Repo, mode),
		Sender:      convert.ToUser(pull.Issue.Poster, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeContent(doer *models.User, issue *models.Issue, oldContent string) {
	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	var err error
	if issue.IsPull {
		issue.PullRequest.Issue = issue
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			PullRequest: convert.ToAPIPullRequest(issue.PullRequest),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventIssues, &api.IssuePayload{
			Action: api.HookIssueEdited,
			Index:  issue.Index,
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyUpdateComment(doer *models.User, c *models.Comment, oldContent string) {
	var err error

	if err = c.LoadPoster(); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}
	if err = c.LoadIssue(); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	if err = c.Issue.LoadAttributes(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	mode, _ := models.AccessLevel(doer, c.Issue.Repo)
	if c.Issue.IsPull {
		err = webhook_services.PrepareWebhooks(c.Issue.Repo, models.HookEventPullRequestComment, &api.IssueCommentPayload{
			Action:  api.HookIssueCommentEdited,
			Issue:   convert.ToAPIIssue(c.Issue),
			Comment: convert.ToComment(c),
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			Repository: convert.ToRepo(c.Issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     true,
		})
	} else {
		err = webhook_services.PrepareWebhooks(c.Issue.Repo, models.HookEventIssueComment, &api.IssueCommentPayload{
			Action:  api.HookIssueCommentEdited,
			Issue:   convert.ToAPIIssue(c.Issue),
			Comment: convert.ToComment(c),
			Changes: &api.ChangesPayload{
				Body: &api.ChangesFromPayload{
					From: oldContent,
				},
			},
			Repository: convert.ToRepo(c.Issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     false,
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", c.ID, err)
	}
}

func (m *webhookNotifier) NotifyCreateIssueComment(doer *models.User, repo *models.Repository,
	issue *models.Issue, comment *models.Comment, mentions []*models.User) {
	mode, _ := models.AccessLevel(doer, repo)

	var err error
	if issue.IsPull {
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventPullRequestComment, &api.IssueCommentPayload{
			Action:     api.HookIssueCommentCreated,
			Issue:      convert.ToAPIIssue(issue),
			Comment:    convert.ToComment(comment),
			Repository: convert.ToRepo(repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     true,
		})
	} else {
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventIssueComment, &api.IssueCommentPayload{
			Action:     api.HookIssueCommentCreated,
			Issue:      convert.ToAPIIssue(issue),
			Comment:    convert.ToComment(comment),
			Repository: convert.ToRepo(repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     false,
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	}
}

func (m *webhookNotifier) NotifyDeleteComment(doer *models.User, comment *models.Comment) {
	var err error

	if err = comment.LoadPoster(); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}
	if err = comment.LoadIssue(); err != nil {
		log.Error("LoadIssue: %v", err)
		return
	}

	if err = comment.Issue.LoadAttributes(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	mode, _ := models.AccessLevel(doer, comment.Issue.Repo)

	if comment.Issue.IsPull {
		err = webhook_services.PrepareWebhooks(comment.Issue.Repo, models.HookEventPullRequestComment, &api.IssueCommentPayload{
			Action:     api.HookIssueCommentDeleted,
			Issue:      convert.ToAPIIssue(comment.Issue),
			Comment:    convert.ToComment(comment),
			Repository: convert.ToRepo(comment.Issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     true,
		})
	} else {
		err = webhook_services.PrepareWebhooks(comment.Issue.Repo, models.HookEventIssueComment, &api.IssueCommentPayload{
			Action:     api.HookIssueCommentDeleted,
			Issue:      convert.ToAPIIssue(comment.Issue),
			Comment:    convert.ToComment(comment),
			Repository: convert.ToRepo(comment.Issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
			IsPull:     false,
		})
	}

	if err != nil {
		log.Error("PrepareWebhooks [comment_id: %d]: %v", comment.ID, err)
	}

}

func (m *webhookNotifier) NotifyIssueChangeLabels(doer *models.User, issue *models.Issue,
	addedLabels []*models.Label, removedLabels []*models.Label) {
	var err error

	if err = issue.LoadRepo(); err != nil {
		log.Error("LoadRepo: %v", err)
		return
	}

	if err = issue.LoadPoster(); err != nil {
		log.Error("LoadPoster: %v", err)
		return
	}

	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	if issue.IsPull {
		if err = issue.LoadPullRequest(); err != nil {
			log.Error("loadPullRequest: %v", err)
			return
		}
		if err = issue.PullRequest.LoadIssue(); err != nil {
			log.Error("LoadIssue: %v", err)
			return
		}
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventPullRequestLabel, &api.PullRequestPayload{
			Action:      api.HookIssueLabelUpdated,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(issue.PullRequest),
			Repository:  convert.ToRepo(issue.Repo, models.AccessModeNone),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventIssueLabel, &api.IssuePayload{
			Action:     api.HookIssueLabelUpdated,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyIssueChangeMilestone(doer *models.User, issue *models.Issue, oldMilestoneID int64) {
	var hookAction api.HookIssueAction
	var err error
	if issue.MilestoneID > 0 {
		hookAction = api.HookIssueMilestoned
	} else {
		hookAction = api.HookIssueDemilestoned
	}

	if err = issue.LoadAttributes(); err != nil {
		log.Error("issue.LoadAttributes failed: %v", err)
		return
	}

	mode, _ := models.AccessLevel(doer, issue.Repo)
	if issue.IsPull {
		err = issue.PullRequest.LoadIssue()
		if err != nil {
			log.Error("LoadIssue: %v", err)
			return
		}
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventPullRequestMilestone, &api.PullRequestPayload{
			Action:      hookAction,
			Index:       issue.Index,
			PullRequest: convert.ToAPIPullRequest(issue.PullRequest),
			Repository:  convert.ToRepo(issue.Repo, mode),
			Sender:      convert.ToUser(doer, nil),
		})
	} else {
		err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventIssueMilestone, &api.IssuePayload{
			Action:     hookAction,
			Index:      issue.Index,
			Issue:      convert.ToAPIIssue(issue),
			Repository: convert.ToRepo(issue.Repo, mode),
			Sender:     convert.ToUser(doer, nil),
		})
	}
	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyPushCommits(pusher *models.User, repo *models.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	if err := webhook_services.PrepareWebhooks(repo, models.HookEventPush, &api.PushPayload{
		Ref:        opts.RefFullName,
		Before:     opts.OldCommitID,
		After:      opts.NewCommitID,
		CompareURL: setting.AppURL + commits.CompareURL,
		Commits:    apiCommits,
		HeadCommit: apiHeadCommit,
		Repo:       convert.ToRepo(repo, models.AccessModeOwner),
		Pusher:     apiPusher,
		Sender:     apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (*webhookNotifier) NotifyMergePullRequest(pr *models.PullRequest, doer *models.User) {
	// Reload pull request information.
	if err := pr.LoadAttributes(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.LoadIssue(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := pr.Issue.LoadRepo(); err != nil {
		log.Error("pr.Issue.LoadRepo: %v", err)
		return
	}

	mode, err := models.AccessLevel(doer, pr.Issue.Repo)
	if err != nil {
		log.Error("models.AccessLevel: %v", err)
		return
	}

	// Merge pull request calls issue.changeStatus so we need to handle separately.
	apiPullRequest := &api.PullRequestPayload{
		Index:       pr.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(pr),
		Repository:  convert.ToRepo(pr.Issue.Repo, mode),
		Sender:      convert.ToUser(doer, nil),
		Action:      api.HookIssueClosed,
	}

	err = webhook_services.PrepareWebhooks(pr.Issue.Repo, models.HookEventPullRequest, apiPullRequest)
	if err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyPullRequestChangeTargetBranch(doer *models.User, pr *models.PullRequest, oldBranch string) {
	issue := pr.Issue
	if !issue.IsPull {
		return
	}
	var err error

	if err = issue.LoadPullRequest(); err != nil {
		log.Error("LoadPullRequest failed: %v", err)
		return
	}
	issue.PullRequest.Issue = issue
	mode, _ := models.AccessLevel(issue.Poster, issue.Repo)
	err = webhook_services.PrepareWebhooks(issue.Repo, models.HookEventPullRequest, &api.PullRequestPayload{
		Action: api.HookIssueEdited,
		Index:  issue.Index,
		Changes: &api.ChangesPayload{
			Ref: &api.ChangesFromPayload{
				From: oldBranch,
			},
		},
		PullRequest: convert.ToAPIPullRequest(issue.PullRequest),
		Repository:  convert.ToRepo(issue.Repo, mode),
		Sender:      convert.ToUser(doer, nil),
	})

	if err != nil {
		log.Error("PrepareWebhooks [is_pull: %v]: %v", issue.IsPull, err)
	}
}

func (m *webhookNotifier) NotifyPullRequestReview(pr *models.PullRequest, review *models.Review, comment *models.Comment, mentions []*models.User) {
	var reviewHookType models.HookEventType

	switch review.Type {
	case models.ReviewTypeApprove:
		reviewHookType = models.HookEventPullRequestReviewApproved
	case models.ReviewTypeComment:
		reviewHookType = models.HookEventPullRequestComment
	case models.ReviewTypeReject:
		reviewHookType = models.HookEventPullRequestReviewRejected
	default:
		// unsupported review webhook type here
		log.Error("Unsupported review webhook type")
		return
	}

	if err := pr.LoadIssue(); err != nil {
		log.Error("pr.LoadIssue: %v", err)
		return
	}

	mode, err := models.AccessLevel(review.Issue.Poster, review.Issue.Repo)
	if err != nil {
		log.Error("models.AccessLevel: %v", err)
		return
	}
	if err := webhook_services.PrepareWebhooks(review.Issue.Repo, reviewHookType, &api.PullRequestPayload{
		Action:      api.HookIssueReviewed,
		Index:       review.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(pr),
		Repository:  convert.ToRepo(review.Issue.Repo, mode),
		Sender:      convert.ToUser(review.Reviewer, nil),
		Review: &api.ReviewPayload{
			Type:    string(reviewHookType),
			Content: review.Content,
		},
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyCreateRef(pusher *models.User, repo *models.Repository, refType, refFullName string) {
	apiPusher := convert.ToUser(pusher, nil)
	apiRepo := convert.ToRepo(repo, models.AccessModeNone)
	refName := git.RefEndName(refFullName)

	gitRepo, err := git.OpenRepository(repo.RepoPath())
	if err != nil {
		log.Error("OpenRepository[%s]: %v", repo.RepoPath(), err)
		return
	}

	shaSum, err := gitRepo.GetRefCommitID(refFullName)
	if err != nil {
		gitRepo.Close()
		log.Error("GetRefCommitID[%s]: %v", refFullName, err)
		return
	}
	gitRepo.Close()

	if err = webhook_services.PrepareWebhooks(repo, models.HookEventCreate, &api.CreatePayload{
		Ref:     refName,
		Sha:     shaSum,
		RefType: refType,
		Repo:    apiRepo,
		Sender:  apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyPullRequestSynchronized(doer *models.User, pr *models.PullRequest) {
	if err := pr.LoadIssue(); err != nil {
		log.Error("pr.LoadIssue: %v", err)
		return
	}
	if err := pr.Issue.LoadAttributes(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	if err := webhook_services.PrepareWebhooks(pr.Issue.Repo, models.HookEventPullRequestSync, &api.PullRequestPayload{
		Action:      api.HookIssueSynchronized,
		Index:       pr.Issue.Index,
		PullRequest: convert.ToAPIPullRequest(pr),
		Repository:  convert.ToRepo(pr.Issue.Repo, models.AccessModeNone),
		Sender:      convert.ToUser(doer, nil),
	}); err != nil {
		log.Error("PrepareWebhooks [pull_id: %v]: %v", pr.ID, err)
	}
}

func (m *webhookNotifier) NotifyDeleteRef(pusher *models.User, repo *models.Repository, refType, refFullName string) {
	apiPusher := convert.ToUser(pusher, nil)
	apiRepo := convert.ToRepo(repo, models.AccessModeNone)
	refName := git.RefEndName(refFullName)

	if err := webhook_services.PrepareWebhooks(repo, models.HookEventDelete, &api.DeletePayload{
		Ref:        refName,
		RefType:    refType,
		PusherType: api.PusherTypeUser,
		Repo:       apiRepo,
		Sender:     apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks.(delete %s): %v", refType, err)
	}
}

func sendReleaseHook(doer *models.User, rel *models.Release, action api.HookReleaseAction) {
	if err := rel.LoadAttributes(); err != nil {
		log.Error("LoadAttributes: %v", err)
		return
	}

	mode, _ := models.AccessLevel(rel.Publisher, rel.Repo)
	if err := webhook_services.PrepareWebhooks(rel.Repo, models.HookEventRelease, &api.ReleasePayload{
		Action:     action,
		Release:    convert.ToRelease(rel),
		Repository: convert.ToRepo(rel.Repo, mode),
		Sender:     convert.ToUser(rel.Publisher, nil),
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifyNewRelease(rel *models.Release) {
	sendReleaseHook(rel.Publisher, rel, api.HookReleasePublished)
}

func (m *webhookNotifier) NotifyUpdateRelease(doer *models.User, rel *models.Release) {
	sendReleaseHook(doer, rel, api.HookReleaseUpdated)
}

func (m *webhookNotifier) NotifyDeleteRelease(doer *models.User, rel *models.Release) {
	sendReleaseHook(doer, rel, api.HookReleaseDeleted)
}

func (m *webhookNotifier) NotifySyncPushCommits(pusher *models.User, repo *models.Repository, opts *repository.PushUpdateOptions, commits *repository.PushCommits) {
	apiPusher := convert.ToUser(pusher, nil)
	apiCommits, apiHeadCommit, err := commits.ToAPIPayloadCommits(repo.RepoPath(), repo.HTMLURL())
	if err != nil {
		log.Error("commits.ToAPIPayloadCommits failed: %v", err)
		return
	}

	if err := webhook_services.PrepareWebhooks(repo, models.HookEventPush, &api.PushPayload{
		Ref:        opts.RefFullName,
		Before:     opts.OldCommitID,
		After:      opts.NewCommitID,
		CompareURL: setting.AppURL + commits.CompareURL,
		Commits:    apiCommits,
		HeadCommit: apiHeadCommit,
		Repo:       convert.ToRepo(repo, models.AccessModeOwner),
		Pusher:     apiPusher,
		Sender:     apiPusher,
	}); err != nil {
		log.Error("PrepareWebhooks: %v", err)
	}
}

func (m *webhookNotifier) NotifySyncCreateRef(pusher *models.User, repo *models.Repository, refType, refFullName string) {
	m.NotifyCreateRef(pusher, repo, refType, refFullName)
}

func (m *webhookNotifier) NotifySyncDeleteRef(pusher *models.User, repo *models.Repository, refType, refFullName string) {
	m.NotifyDeleteRef(pusher, repo, refType, refFullName)
}
