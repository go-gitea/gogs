// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	jsoniter "github.com/json-iterator/go"
)

// SlackMeta contains the slack metadata
type SlackMeta struct {
	Channel  string `json:"channel"`
	Username string `json:"username"`
	IconURL  string `json:"icon_url"`
	Color    string `json:"color"`
}

// GetSlackHook returns slack metadata
func GetSlackHook(w *models.Webhook) *SlackMeta {
	s := &SlackMeta{}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetSlackHook(%d): %v", w.ID, err)
	}
	return s
}

// SlackPayload contains the information about the slack channel
type SlackPayload struct {
	Channel     string            `json:"channel"`
	Text        string            `json:"text"`
	Color       string            `json:"-"`
	Username    string            `json:"username"`
	IconURL     string            `json:"icon_url"`
	UnfurlLinks int               `json:"unfurl_links"`
	LinkNames   int               `json:"link_names"`
	Attachments []SlackAttachment `json:"attachments"`
}

// SlackAttachment contains the slack message
type SlackAttachment struct {
	Fallback  string `json:"fallback"`
	Color     string `json:"color"`
	Title     string `json:"title"`
	TitleLink string `json:"title_link"`
	Text      string `json:"text"`
}

// JSONPayload Marshals the SlackPayload to json
func (s *SlackPayload) JSONPayload() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// SlackTextFormatter replaces &, <, > with HTML characters
// see: https://api.slack.com/docs/formatting
func SlackTextFormatter(s string) string {
	// replace & < >
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// SlackShortTextFormatter replaces &, <, > with HTML characters
func SlackShortTextFormatter(s string) string {
	s = strings.Split(s, "\n")[0]
	// replace & < >
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// SlackLinkFormatter creates a link compatible with slack
func SlackLinkFormatter(url string, text string) string {
	return fmt.Sprintf("<%s|%s>", url, SlackTextFormatter(text))
}

// SlackLinkToRef slack-formatter link to a repo ref
func SlackLinkToRef(repoURL, ref string) string {
	url := git.RefURL(repoURL, ref)
	refName := git.RefEndName(ref)
	return SlackLinkFormatter(url, refName)
}

var (
	_ PayloadConvertor = &SlackPayload{}
)

// Create implements PayloadConvertor Create method
func (s *SlackPayload) Create(p *api.CreatePayload) (api.Payloader, error) {
	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	refLink := SlackLinkToRef(p.Repo.HTMLURL, p.Ref)
	text := fmt.Sprintf("[%s:%s] %s created by %s", repoLink, refLink, p.RefType, p.Sender.UserName)

	return s.createPayload(text, nil), nil
}

// Delete composes Slack payload for delete a branch or tag.
func (s *SlackPayload) Delete(p *api.DeletePayload) (api.Payloader, error) {
	refName := git.RefEndName(p.Ref)
	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("[%s:%s] %s deleted by %s", repoLink, refName, p.RefType, p.Sender.UserName)

	return s.createPayload(text, nil), nil
}

// Fork composes Slack payload for forked by a repository.
func (s *SlackPayload) Fork(p *api.ForkPayload) (api.Payloader, error) {
	baseLink := SlackLinkFormatter(p.Forkee.HTMLURL, p.Forkee.FullName)
	forkLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("%s is forked to %s", baseLink, forkLink)

	return s.createPayload(text, nil), nil
}

// Issue implements PayloadConvertor Issue method
func (s *SlackPayload) Issue(p *api.IssuePayload) (api.Payloader, error) {
	text, issueTitle, attachmentText, color := getIssuesPayloadInfo(p, SlackLinkFormatter, true)

	var attachments []SlackAttachment
	if attachmentText != "" {
		attachmentText = SlackTextFormatter(attachmentText)
		issueTitle = SlackTextFormatter(issueTitle)
		attachments = append(attachments, SlackAttachment{
			Color:     fmt.Sprintf("%x", color),
			Title:     issueTitle,
			TitleLink: p.Issue.HTMLURL,
			Text:      attachmentText,
		})
	}

	return s.createPayload(text, attachments), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (s *SlackPayload) IssueComment(p *api.IssueCommentPayload) (api.Payloader, error) {
	text, issueTitle, color := getIssueCommentPayloadInfo(p, SlackLinkFormatter, true)

	return s.createPayload(text, []SlackAttachment{{
		Color:     fmt.Sprintf("%x", color),
		Title:     issueTitle,
		TitleLink: p.Comment.HTMLURL,
		Text:      SlackTextFormatter(p.Comment.Body),
	}}), nil
}

// Release implements PayloadConvertor Release method
func (s *SlackPayload) Release(p *api.ReleasePayload) (api.Payloader, error) {
	text, _ := getReleasePayloadInfo(p, SlackLinkFormatter, true)

	return s.createPayload(text, nil), nil
}

// Push implements PayloadConvertor Push method
func (s *SlackPayload) Push(p *api.PushPayload) (api.Payloader, error) {
	// n new commits
	var (
		commitDesc   string
		commitString string
	)

	if len(p.Commits) == 1 {
		commitDesc = "1 new commit"
	} else {
		commitDesc = fmt.Sprintf("%d new commits", len(p.Commits))
	}
	if len(p.CompareURL) > 0 {
		commitString = SlackLinkFormatter(p.CompareURL, commitDesc)
	} else {
		commitString = commitDesc
	}

	repoLink := SlackLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	branchLink := SlackLinkToRef(p.Repo.HTMLURL, p.Ref)
	text := fmt.Sprintf("[%s:%s] %s pushed by %s", repoLink, branchLink, commitString, p.Pusher.UserName)

	var attachmentText string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		attachmentText += fmt.Sprintf("%s: %s - %s", SlackLinkFormatter(commit.URL, commit.ID[:7]), SlackShortTextFormatter(commit.Message), SlackTextFormatter(commit.Author.Name))
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			attachmentText += "\n"
		}
	}

	return s.createPayload(text, []SlackAttachment{{
		Color:     s.Color,
		Title:     p.Repo.HTMLURL,
		TitleLink: p.Repo.HTMLURL,
		Text:      attachmentText,
	}}), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (s *SlackPayload) PullRequest(p *api.PullRequestPayload) (api.Payloader, error) {
	text, issueTitle, attachmentText, color := getPullRequestPayloadInfo(p, SlackLinkFormatter, true)

	var attachments []SlackAttachment
	if attachmentText != "" {
		attachmentText = SlackTextFormatter(p.PullRequest.Body)
		issueTitle = SlackTextFormatter(issueTitle)
		attachments = append(attachments, SlackAttachment{
			Color:     fmt.Sprintf("%x", color),
			Title:     issueTitle,
			TitleLink: p.PullRequest.URL,
			Text:      attachmentText,
		})
	}

	return s.createPayload(text, attachments), nil
}

// Review implements PayloadConvertor Review method
func (s *SlackPayload) Review(p *api.PullRequestPayload, event models.HookEventType) (api.Payloader, error) {
	senderLink := SlackLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	title := fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title)
	titleLink := fmt.Sprintf("%s/pulls/%d", p.Repository.HTMLURL, p.Index)
	repoLink := SlackLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text string

	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		text = fmt.Sprintf("[%s] Pull request review %s: [%s](%s) by %s", repoLink, action, title, titleLink, senderLink)
	}

	return s.createPayload(text, nil), nil
}

// Repository implements PayloadConvertor Repository method
func (s *SlackPayload) Repository(p *api.RepositoryPayload) (api.Payloader, error) {
	senderLink := SlackLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	repoLink := SlackLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text string

	switch p.Action {
	case api.HookRepoCreated:
		text = fmt.Sprintf("[%s] Repository created by %s", repoLink, senderLink)
	case api.HookRepoDeleted:
		text = fmt.Sprintf("[%s] Repository deleted by %s", repoLink, senderLink)
	}

	return s.createPayload(text, nil), nil
}

func (s *SlackPayload) createPayload(text string, attachments []SlackAttachment) *SlackPayload {
	return &SlackPayload{
		Channel:     s.Channel,
		Text:        text,
		Username:    s.Username,
		IconURL:     s.IconURL,
		Attachments: attachments,
	}
}

// GetSlackPayload converts a slack webhook into a SlackPayload
func GetSlackPayload(p api.Payloader, event models.HookEventType, meta string) (api.Payloader, error) {
	s := new(SlackPayload)

	slack := &SlackMeta{}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(meta), &slack); err != nil {
		return s, errors.New("GetSlackPayload meta json:" + err.Error())
	}

	s.Channel = slack.Channel
	s.Username = slack.Username
	s.IconURL = slack.IconURL
	s.Color = slack.Color

	return convertPayloader(s, p, event)
}
