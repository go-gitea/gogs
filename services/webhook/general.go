// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"fmt"
	"html"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
)

type linkFormatter = func(string, string) string

// noneLinkFormatter does not create a link but just returns the text
func noneLinkFormatter(url string, text string) string {
	return text
}

// htmlLinkFormatter creates a HTML link
func htmlLinkFormatter(url string, text string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, url, html.EscapeString(text))
}

func getIssuesPayloadInfo(p *api.IssuePayload, linkFormatter linkFormatter, withSender bool) (string, string, string, int) {
	repoLink := linkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	issueTitle := fmt.Sprintf("#%d %s", p.Index, p.Issue.Title)
	titleLink := linkFormatter(fmt.Sprintf("%s/issues/%d", p.Repository.HTMLURL, p.Index), issueTitle)
	var text string
	color := yellowColor

	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("[%s] Issue opened: %s", repoLink, titleLink)
		color = orangeColor
	case api.HookIssueClosed:
		text = fmt.Sprintf("[%s] Issue closed: %s", repoLink, titleLink)
		color = redColor
	case api.HookIssueReOpened:
		text = fmt.Sprintf("[%s] Issue re-opened: %s", repoLink, titleLink)
	case api.HookIssueEdited:
		text = fmt.Sprintf("[%s] Issue edited: %s", repoLink, titleLink)
	case api.HookIssueAssigned:
		list := make([]string, len(p.Issue.Assignees))
		for i, user := range p.Issue.Assignees {
			list[i] = linkFormatter(setting.AppURL+user.UserName, user.UserName)
		}
		text = fmt.Sprintf("[%s] Issue assigned to %s: %s", repoLink, strings.Join(list, ", "), titleLink)
		color = greenColor
	case api.HookIssueUnassigned:
		text = fmt.Sprintf("[%s] Issue unassigned: %s", repoLink, titleLink)
	case api.HookIssueLabelUpdated:
		text = fmt.Sprintf("[%s] Issue labels updated: %s", repoLink, titleLink)
	case api.HookIssueLabelCleared:
		text = fmt.Sprintf("[%s] Issue labels cleared: %s", repoLink, titleLink)
	case api.HookIssueSynchronized:
		text = fmt.Sprintf("[%s] Issue synchronized: %s", repoLink, titleLink)
	case api.HookIssueMilestoned:
		mileStoneLink := fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.Issue.Milestone.ID)
		text = fmt.Sprintf("[%s] Issue milestoned to %s: %s", repoLink,
			linkFormatter(mileStoneLink, p.Issue.Milestone.Title), titleLink)
	case api.HookIssueDemilestoned:
		text = fmt.Sprintf("[%s] Issue milestone cleared: %s", repoLink, titleLink)
	}
	if withSender {
		text += fmt.Sprintf(" by %s", linkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName))
	}

	var attachmentText string
	if p.Action == api.HookIssueOpened || p.Action == api.HookIssueEdited {
		attachmentText = p.Issue.Body
	}

	return text, issueTitle, attachmentText, color
}

func getPullRequestPayloadInfo(p *api.PullRequestPayload, linkFormatter linkFormatter, withSender bool) (string, string, string, int) {
	repoLink := linkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	issueTitle := fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title)
	titleLink := linkFormatter(p.PullRequest.URL, issueTitle)
	var text string
	color := yellowColor

	switch p.Action {
	case api.HookIssueOpened:
		text = fmt.Sprintf("[%s] Pull request opened: %s", repoLink, titleLink)
		color = greenColor
	case api.HookIssueClosed:
		if p.PullRequest.HasMerged {
			text = fmt.Sprintf("[%s] Pull request merged: %s", repoLink, titleLink)
			color = purpleColor
		} else {
			text = fmt.Sprintf("[%s] Pull request closed: %s", repoLink, titleLink)
			color = redColor
		}
	case api.HookIssueReOpened:
		text = fmt.Sprintf("[%s] Pull request re-opened: %s", repoLink, titleLink)
	case api.HookIssueEdited:
		text = fmt.Sprintf("[%s] Pull request edited: %s", repoLink, titleLink)
	case api.HookIssueAssigned:
		list := make([]string, len(p.PullRequest.Assignees))
		for i, user := range p.PullRequest.Assignees {
			list[i] = linkFormatter(setting.AppURL+user.UserName, user.UserName)
		}
		text = fmt.Sprintf("[%s] Pull request assigned to %s: %s", repoLink,
			strings.Join(list, ", "), titleLink)
		color = greenColor
	case api.HookIssueUnassigned:
		text = fmt.Sprintf("[%s] Pull request unassigned: %s", repoLink, titleLink)
	case api.HookIssueLabelUpdated:
		text = fmt.Sprintf("[%s] Pull request labels updated: %s", repoLink, titleLink)
	case api.HookIssueLabelCleared:
		text = fmt.Sprintf("[%s] Pull request labels cleared: %s", repoLink, titleLink)
	case api.HookIssueSynchronized:
		text = fmt.Sprintf("[%s] Pull request synchronized: %s", repoLink, titleLink)
	case api.HookIssueMilestoned:
		mileStoneLink := fmt.Sprintf("%s/milestone/%d", p.Repository.HTMLURL, p.PullRequest.Milestone.ID)
		text = fmt.Sprintf("[%s] Pull request milestoned to %s: %s", repoLink,
			linkFormatter(mileStoneLink, p.PullRequest.Milestone.Title), titleLink)
	case api.HookIssueDemilestoned:
		text = fmt.Sprintf("[%s] Pull request milestone cleared: %s", repoLink, titleLink)
	case api.HookIssueReviewed:
		text = fmt.Sprintf("[%s] Pull request reviewed: %s", repoLink, titleLink)
	}
	if withSender {
		text += fmt.Sprintf(" by %s", linkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName))
	}

	var attachmentText string
	if p.Action == api.HookIssueOpened || p.Action == api.HookIssueEdited {
		attachmentText = p.PullRequest.Body
	}

	return text, issueTitle, attachmentText, color
}

func getReleasePayloadInfo(p *api.ReleasePayload, linkFormatter linkFormatter, withSender bool) (text string, color int) {
	repoLink := linkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	refLink := linkFormatter(p.Repository.HTMLURL+"/src/"+p.Release.TagName, p.Release.TagName)

	switch p.Action {
	case api.HookReleasePublished:
		text = fmt.Sprintf("[%s] Release created: %s", repoLink, refLink)
		color = greenColor
	case api.HookReleaseUpdated:
		text = fmt.Sprintf("[%s] Release updated: %s", repoLink, refLink)
		color = yellowColor
	case api.HookReleaseDeleted:
		text = fmt.Sprintf("[%s] Release deleted: %s", repoLink, refLink)
		color = redColor
	}
	if withSender {
		text += fmt.Sprintf(" by %s", linkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName))
	}

	return text, color
}

func getIssueCommentPayloadInfo(p *api.IssueCommentPayload, linkFormatter linkFormatter, withSender bool) (string, string, int) {
	repoLink := linkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	issueTitle := fmt.Sprintf("#%d %s", p.Issue.Index, p.Issue.Title)

	var text, typ, titleLink string
	color := yellowColor

	if p.IsPull {
		typ = "pull request"
		titleLink = linkFormatter(p.Comment.PRURL, issueTitle)
	} else {
		typ = "issue"
		titleLink = linkFormatter(p.Comment.IssueURL, issueTitle)
	}

	switch p.Action {
	case api.HookIssueCommentCreated:
		text = fmt.Sprintf("[%s] New comment on %s %s", repoLink, typ, titleLink)
		if p.IsPull {
			color = greenColorLight
		} else {
			color = orangeColorLight
		}
	case api.HookIssueCommentEdited:
		text = fmt.Sprintf("[%s] Comment edited on %s %s", repoLink, typ, titleLink)
	case api.HookIssueCommentDeleted:
		text = fmt.Sprintf("[%s] Comment deleted on %s %s", repoLink, typ, titleLink)
		color = redColor
	}
	if withSender {
		text += fmt.Sprintf(" by %s", linkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName))
	}

	return text, issueTitle, color
}
