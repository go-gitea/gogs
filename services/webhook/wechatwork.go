// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"encoding/json"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
)

type (
	// WechatworkPayload represents
	WechatworkPayload struct {
		Msgtype string `json:"msgtype"`
		Text    struct {
			Content             string   `json:"content"`
			MentionedList       []string `json:"mentioned_list"`
			MentionedMobileList []string `json:"mentioned_mobile_list"`
		} `json:"text"`
		Markdown struct {
			Content string `json:"content"`
		} `json:"markdown"`
	}
)

// SetSecret sets the Wechatwork secret
func (f *WechatworkPayload) SetSecret(_ string) {}

// JSONPayload Marshals the WechatworkPayload to json
func (f *WechatworkPayload) JSONPayload() ([]byte, error) {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

func newWechatworkMarkdownPayload(title string) *WechatworkPayload {
	return &WechatworkPayload{
		Msgtype: "markdown",
		Markdown: struct {
			Content string `json:"content"`
		}{
			Content: title,
		},
	}
}

var (
	_ PayloadConvertor = &WechatworkPayload{}
)

// Create implements PayloadConvertor Create method
func (f *WechatworkPayload) Create(p *api.CreatePayload) (api.Payloader, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return newWechatworkMarkdownPayload(title), nil
}

// Delete implements PayloadConvertor Delete method
func (f *WechatworkPayload) Delete(p *api.DeletePayload) (api.Payloader, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return newWechatworkMarkdownPayload(title), nil

}

// Fork implements PayloadConvertor Fork method
func (f *WechatworkPayload) Fork(p *api.ForkPayload) (api.Payloader, error) {

	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return newWechatworkMarkdownPayload(title), nil
}

// Push implements PayloadConvertor Push method
func (f *WechatworkPayload) Push(p *api.PushPayload) (api.Payloader, error) {
	var (
		branchName = git.RefEndName(p.Ref)
		commitDesc string
	)

	title := fmt.Sprintf("# %s:%s <font color=\"warning\">  %s  </font>", p.Repo.FullName, branchName, commitDesc)

	var text string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		var authorName string
		if commit.Author != nil {
			authorName = "Author：" + commit.Author.Name
		}

		message := strings.ReplaceAll(commit.Message, "\n\n", "\r\n")
		text += fmt.Sprintf(" > [%s](%s) \r\n ><font color=\"info\">%s</font> \n ><font color=\"warning\">%s</font>", commit.ID[:7], commit.URL,
			message, authorName)

		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "\n"
		}
	}
	return newWechatworkMarkdownPayload(title + "\r\n\r\n" + text), nil

}

// Issue implements PayloadConvertor Issue method
func (f *WechatworkPayload) Issue(p *api.IssuePayload) (api.Payloader, error) {
	text, issueTitle, attachmentText, _ := getIssuesPayloadInfo(p, noneLinkFormatter, true)
	var content string
	content += fmt.Sprintf(" ><font color=\"info\">%s</font>\n >%s \n ><font color=\"warning\"> %s</font>", text, attachmentText, issueTitle)

	return newWechatworkMarkdownPayload(content), nil

}

// IssueComment implements PayloadConvertor IssueComment method
func (f *WechatworkPayload) IssueComment(p *api.IssueCommentPayload) (api.Payloader, error) {
	text, issueTitle, _ := getIssueCommentPayloadInfo(p, noneLinkFormatter, true)
	var content string
	content += fmt.Sprintf(" ><font color=\"info\">%s</font>\n >%s \n ><font color=\"warning\">%s</font>", text, p.Comment.Body, issueTitle)

	return newWechatworkMarkdownPayload(content), nil

}

// PullRequest implements PayloadConvertor PullRequest method
func (f *WechatworkPayload) PullRequest(p *api.PullRequestPayload) (api.Payloader, error) {
	text, issueTitle, attachmentText, _ := getPullRequestPayloadInfo(p, noneLinkFormatter, true)
	pr := fmt.Sprintf("> <font color=\"info\"> %s </font> \r\n > <font color=\"comment\">%s </font> \r\n > <font color=\"comment\">%s </font> \r\n",
		text, issueTitle, attachmentText)

	return newWechatworkMarkdownPayload(pr), nil

}

// Review implements PayloadConvertor Review method
func (f *WechatworkPayload) Review(p *api.PullRequestPayload, event models.HookEventType) (api.Payloader, error) {
	var text, title string
	switch p.Action {
	case api.HookIssueSynchronized:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}
		title = fmt.Sprintf("[%s] Pull request review %s : #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.Review.Content
	}

	return newWechatworkMarkdownPayload("# " + title + "\r\n\r\n >" + text), nil

}

// Repository implements PayloadConvertor Repository method
func (f *WechatworkPayload) Repository(p *api.RepositoryPayload) (api.Payloader, error) {
	var title string
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		return newWechatworkMarkdownPayload(title), nil
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return newWechatworkMarkdownPayload(title), nil
	}

	return nil, nil
}

// Release implements PayloadConvertor Release method
func (f *WechatworkPayload) Release(p *api.ReleasePayload) (api.Payloader, error) {
	text, _ := getReleasePayloadInfo(p, noneLinkFormatter, true)

	return newWechatworkMarkdownPayload(text), nil
}

// GetWechatworkPayload GetWechatworkPayload converts a ding talk webhook into a WechatworkPayload
func GetWechatworkPayload(p api.Payloader, event models.HookEventType, meta string) (api.Payloader, error) {
	return convertPayloader(new(WechatworkPayload), p, event)
}
