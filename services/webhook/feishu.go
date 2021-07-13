// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	api "code.gitea.io/gitea/modules/structs"
	jsoniter "github.com/json-iterator/go"
)

type (
	// FeishuPayload represents
	FeishuPayload struct {
		MsgType string `json:"msg_type"` // text / post / image / share_chat / interactive
		Content struct {
			Text string `json:"text"`
		} `json:"content"`
	}
)

func newFeishuTextPayload(text string) *FeishuPayload {
	return &FeishuPayload{
		MsgType: "text",
		Content: struct {
			Text string `json:"text"`
		}{
			Text: strings.TrimSpace(text),
		},
	}
}

// JSONPayload Marshals the FeishuPayload to json
func (f *FeishuPayload) JSONPayload() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

var (
	_ PayloadConvertor = &FeishuPayload{}
)

// Create implements PayloadConvertor Create method
func (f *FeishuPayload) Create(p *api.CreatePayload) (api.Payloader, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	text := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return newFeishuTextPayload(text), nil
}

// Delete implements PayloadConvertor Delete method
func (f *FeishuPayload) Delete(p *api.DeletePayload) (api.Payloader, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	text := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return newFeishuTextPayload(text), nil
}

// Fork implements PayloadConvertor Fork method
func (f *FeishuPayload) Fork(p *api.ForkPayload) (api.Payloader, error) {
	text := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return newFeishuTextPayload(text), nil
}

// Push implements PayloadConvertor Push method
func (f *FeishuPayload) Push(p *api.PushPayload) (api.Payloader, error) {
	var (
		branchName = git.RefEndName(p.Ref)
		commitDesc string
	)

	var text = fmt.Sprintf("[%s:%s] %s\r\n", p.Repo.FullName, branchName, commitDesc)
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		var authorName string
		if commit.Author != nil {
			authorName = " - " + commit.Author.Name
		}
		text += fmt.Sprintf("[%s](%s) %s", commit.ID[:7], commit.URL,
			strings.TrimRight(commit.Message, "\r\n")) + authorName
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "\r\n"
		}
	}

	return newFeishuTextPayload(text), nil
}

// Issue implements PayloadConvertor Issue method
func (f *FeishuPayload) Issue(p *api.IssuePayload) (api.Payloader, error) {
	text, issueTitle, attachmentText, _ := getIssuesPayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(issueTitle + "\r\n" + text + "\r\n\r\n" + attachmentText), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (f *FeishuPayload) IssueComment(p *api.IssueCommentPayload) (api.Payloader, error) {
	text, issueTitle, _ := getIssueCommentPayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(issueTitle + "\r\n" + text + "\r\n\r\n" + p.Comment.Body), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (f *FeishuPayload) PullRequest(p *api.PullRequestPayload) (api.Payloader, error) {
	text, issueTitle, attachmentText, _ := getPullRequestPayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(issueTitle + "\r\n" + text + "\r\n\r\n" + attachmentText), nil
}

// Review implements PayloadConvertor Review method
func (f *FeishuPayload) Review(p *api.PullRequestPayload, event models.HookEventType) (api.Payloader, error) {
	action, err := parseHookPullRequestEventType(event)
	if err != nil {
		return nil, err
	}

	title := fmt.Sprintf("[%s] Pull request review %s : #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
	text := p.Review.Content

	return newFeishuTextPayload(title + "\r\n\r\n" + text), nil
}

// Repository implements PayloadConvertor Repository method
func (f *FeishuPayload) Repository(p *api.RepositoryPayload) (api.Payloader, error) {
	var text string
	switch p.Action {
	case api.HookRepoCreated:
		text = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		return newFeishuTextPayload(text), nil
	case api.HookRepoDeleted:
		text = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		return newFeishuTextPayload(text), nil
	}

	return nil, nil
}

// Release implements PayloadConvertor Release method
func (f *FeishuPayload) Release(p *api.ReleasePayload) (api.Payloader, error) {
	text, _ := getReleasePayloadInfo(p, noneLinkFormatter, true)

	return newFeishuTextPayload(text), nil
}

// GetFeishuPayload converts a ding talk webhook into a FeishuPayload
func GetFeishuPayload(p api.Payloader, event models.HookEventType, meta string) (api.Payloader, error) {
	return convertPayloader(new(FeishuPayload), p, event)
}
