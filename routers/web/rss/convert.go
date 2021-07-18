package rss

import (
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/context"

	"github.com/gorilla/feeds"
)

func FeedActionsToFeedItems(ctx *context.Context, actions []*models.Action) (items []*feeds.Item) {
	for _, act := range actions {
		act.LoadActUser()

		content, desc, title := "", "", ""

		link := &feeds.Link{Href: act.GetCommentLink()}

		title = tmpTypeName(act.OpType)

		// title
		{
			switch act.OpType {

			case models.ActionCreateRepo:
				title = ctx.Tr("action.create_repo", act.GetRepoLink(), act.ShortRepoPath())
			case models.ActionRenameRepo:
				title = ctx.Tr("action.rename_repo", act.GetContent(), act.GetRepoLink(), act.ShortRepoPath())
			case models.ActionCommitRepo:
				branchLink := act.GetBranch()
				if len(act.Content) != 0 {
					title = ctx.Tr("action.commit_repo", act.GetRepoLink(), branchLink, act.GetBranch(), act.ShortRepoPath())
				} else {
					title = ctx.Tr("action.create_branch", act.GetRepoLink(), branchLink, act.GetBranch(), act.ShortRepoPath())
				}
			case models.ActionCreateIssue:
				title = `	{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.create_issue" .GetRepoLink $index .ShortRepoPath | Str2html}}`
			case models.ActionCreatePullRequest:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.create_pull_request" .GetRepoLink $index .ShortRepoPath | Str2html}}`
			case models.ActionTransferRepo:
				title = `{{$.i18n.Tr "action.transfer_repo" .GetContent .GetRepoLink .ShortRepoPath | Str2html}}`
			case models.ActionPushTag:
				title = `{{ $tagLink := .GetTag | EscapePound | Escape}}
			{{$.i18n.Tr "action.push_tag" .GetRepoLink $tagLink .ShortRepoPath | Str2html}}`
			case models.ActionCommentIssue:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.comment_issue" .GetRepoLink $index .ShortRepoPath | Str2html}}`
			case models.ActionMergePullRequest:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.merge_pull_request" .GetRepoLink $index .ShortRepoPath | Str2html}}`
			case models.ActionCloseIssue:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.close_issue" .GetRepoLink $index .ShortRepoPath | Str2html}}`
			case models.ActionReopenIssue:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.reopen_issue" .GetRepoLink $index .ShortRepoPath | Str2html}}`
			case models.ActionClosePullRequest:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.close_pull_request" .GetRepoLink $index .ShortRepoPath | Str2html}}`
			case models.ActionReopenPullRequest:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.reopen_pull_request" .GetRepoLink $index .ShortRepoPath | Str2html}}`
			case models.ActionDeleteTag:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.delete_tag" .GetRepoLink (.GetTag|Escape) .ShortRepoPath | Str2html}}`
			case models.ActionDeleteBranch:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.delete_branch" .GetRepoLink (.GetBranch|Escape) .ShortRepoPath | Str2html}}`
			case models.ActionMirrorSyncPush:
				title = `{{ $branchLink := .GetBranch | EscapePound}}
			{{$.i18n.Tr "action.mirror_sync_push" .GetRepoLink $branchLink (.GetBranch|Escape) .ShortRepoPath | Str2html}}`
			case models.ActionMirrorSyncCreate:
				title = `{{$.i18n.Tr "action.mirror_sync_create" .GetRepoLink (.GetBranch|Escape) .ShortRepoPath | Str2html}}`
			case models.ActionMirrorSyncDelete:
				title = `{{$.i18n.Tr "action.mirror_sync_delete" .GetRepoLink (.GetBranch|Escape) .ShortRepoPath | Str2html}}`
			case models.ActionApprovePullRequest:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.approve_pull_request" .GetRepoLink $index .ShortRepoPath | Str2html}}`
			case models.ActionRejectPullRequest:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.reject_pull_request" .GetRepoLink $index .ShortRepoPath | Str2html}}`
			case models.ActionCommentPull:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{$.i18n.Tr "action.comment_pull" .GetRepoLink $index .ShortRepoPath | Str2html}}`
			case models.ActionPublishRelease:
				title = `{{ $branchLink := .GetBranch | EscapePound | Escape}}
			{{ $linkText := .Content | RenderEmoji }}
			{{$.i18n.Tr "action.publish_release" .GetRepoLink $branchLink .ShortRepoPath $linkText | Str2html}}`
			case models.ActionPullReviewDismissed:
				title = `{{ $index := index .GetIssueInfos 0}}
			{{ $reviewer := index .GetIssueInfos 1}}
			{{$.i18n.Tr "action.review_dismissed" .GetRepoLink $index .ShortRepoPath $reviewer | Str2html}}`
			}
		}

		// description & content
		{
			switch act.OpType {
			case models.ActionCommitRepo, models.ActionMirrorSyncPush:
				desc = `<div class="content">
							<ul>
								{{ $push := ActionContent2Commits .}}
								{{ $repoLink := .GetRepoLink}}
								{{if $push.Commits}}
									{{range $push.Commits}}
										{{ $commitLink := printf "%s/commit/%s" $repoLink .Sha1}}
										<li>
											{{avatarHTML ($push.AvatarLink .AuthorEmail) 16 "mr-2" .AuthorName}}
											<a class="commit-id mr-2" href="{{$commitLink}}">{{ShortSha .Sha1}}</a>
											<span class="text truncate light grey">
												{{RenderCommitMessage .Message $repoLink $.ComposeMetas}}
											</span>
										</li>
									{{end}}
								{{end}}
								{{if and (gt $push.Len 1) $push.CompareURL}}<li><a href="{{AppSubUrl}}/{{$push.CompareURL}}">{{$.i18n.Tr "action.compare_commits" $push.Len}} »</a></li>{{end}}
							</ul>
						</div>`

			case models.ActionCreateIssue, models.ActionCreatePullRequest:
				desc = strings.Join(act.GetIssueInfos(), "#") // index .GetIssueInfos 1 | RenderEmoji
				content = act.GetIssueContent()
			case models.ActionCommentIssue, models.ActionApprovePullRequest, models.ActionRejectPullRequest, models.ActionCommentPull:
				desc = act.GetIssueTitle()        // class="text truncate issue title" | RenderEmoji
				comment := act.GetIssueInfos()[1] // class="text light grey" | RenderEmoji
				if len(comment) != 0 {
					desc += "\n\n" + comment
				}
			case models.ActionMergePullRequest:
				desc = act.GetIssueInfos()[1] // class="text light grey"
			case models.ActionCloseIssue, models.ActionReopenIssue, models.ActionClosePullRequest, models.ActionReopenPullRequest:
				desc = `<span class="text truncate issue title">{{.GetIssueTitle | RenderEmoji}}</span>`
			case models.ActionPullReviewDismissed:
				desc = `<p class="text light grey">{{$.i18n.Tr "action.review_dismissed_reason"}}</p>
		<p class="text light grey">{{index .GetIssueInfos 2 | RenderEmoji}}</p>`
			}
		}
		if len(content) == 0 {
			content = desc
		}

		// img := templates.ActionIcon(act.OpType)

		items = append(items, &feeds.Item{
			Title:       title,
			Link:        link,
			Description: desc,
			Author:      feedsAuthor(act.ActUser),
			Id:          fmt.Sprint(act.ID),
			Created:     time.Now(), // Created:     act.CreatedUnix.AsTime(),
			Content:     content,
		})
	}
	return
}

func feedsAuthor(user *models.User) *feeds.Author {
	return &feeds.Author{
		Name:  user.DisplayName(),
		Email: user.GetEmail(),
	}
}

func tmpTypeName(t models.ActionType) string {
	switch t {
	case models.ActionCreateRepo:
		return "ActionCreateRepo"
	case models.ActionRenameRepo:
		return "ActionRenameRepo"
	case models.ActionStarRepo:
		return "ActionStarRepo"
	case models.ActionWatchRepo:
		return "ActionWatchRepo"
	case models.ActionCommitRepo:
		return "ActionCommitRepo"
	case models.ActionCreateIssue:
		return "ActionCreateIssue"
	case models.ActionCreatePullRequest:
		return "ActionCreatePullRequest"
	case models.ActionTransferRepo:
		return "ActionTransferRepo"
	case models.ActionPushTag:
		return "ActionPushTag"
	case models.ActionCommentIssue:
		return "ActionCommentIssue"
	case models.ActionMergePullRequest:
		return "ActionMergePullRequest"
	case models.ActionCloseIssue:
		return "ActionCloseIssue"
	case models.ActionReopenIssue:
		return "ActionReopenIssue"
	case models.ActionClosePullRequest:
		return "ActionClosePullRequest"
	case models.ActionReopenPullRequest:
		return "ActionReopenPullRequest"
	case models.ActionDeleteTag:
		return "ActionDeleteTag"
	case models.ActionDeleteBranch:
		return "ActionDeleteBranch"
	case models.ActionMirrorSyncPush:
		return "ActionMirrorSyncPush"
	case models.ActionMirrorSyncCreate:
		return "ActionMirrorSyncCreate"
	case models.ActionMirrorSyncDelete:
		return "ActionMirrorSyncDelete"
	case models.ActionApprovePullRequest:
		return "ActionApprovePullRequest"
	case models.ActionRejectPullRequest:
		return "ActionRejectPullRequest"
	case models.ActionCommentPull:
		return "ActionCommentPull"
	case models.ActionPublishRelease:
		return "ActionPublishRelease"
	case models.ActionPullReviewDismissed:
		return "ActionPullReviewDismissed"
	}
	return ""
}

/*

 */
