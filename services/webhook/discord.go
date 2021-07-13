// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	jsoniter "github.com/json-iterator/go"
)

type (
	// DiscordEmbedFooter for Embed Footer Structure.
	DiscordEmbedFooter struct {
		Text string `json:"text"`
	}

	// DiscordEmbedAuthor for Embed Author Structure
	DiscordEmbedAuthor struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		IconURL string `json:"icon_url"`
	}

	// DiscordEmbedField for Embed Field Structure
	DiscordEmbedField struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	// DiscordEmbed is for Embed Structure
	DiscordEmbed struct {
		Title       string              `json:"title"`
		Description string              `json:"description"`
		URL         string              `json:"url"`
		Color       int                 `json:"color"`
		Footer      DiscordEmbedFooter  `json:"footer"`
		Author      DiscordEmbedAuthor  `json:"author"`
		Fields      []DiscordEmbedField `json:"fields"`
	}

	// DiscordPayload represents
	DiscordPayload struct {
		Wait      bool           `json:"wait"`
		Content   string         `json:"content"`
		Username  string         `json:"username"`
		AvatarURL string         `json:"avatar_url"`
		TTS       bool           `json:"tts"`
		Embeds    []DiscordEmbed `json:"embeds"`
	}

	// DiscordMeta contains the discord metadata
	DiscordMeta struct {
		Username string `json:"username"`
		IconURL  string `json:"icon_url"`
	}
)

// GetDiscordHook returns discord metadata
func GetDiscordHook(w *models.Webhook) *DiscordMeta {
	s := &DiscordMeta{}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetDiscordHook(%d): %v", w.ID, err)
	}
	return s
}

func color(clr string) int {
	if clr != "" {
		clr = strings.TrimLeft(clr, "#")
		if s, err := strconv.ParseInt(clr, 16, 32); err == nil {
			return int(s)
		}
	}

	return 0
}

var (
	greenColor       = color("1ac600")
	greenColorLight  = color("bfe5bf")
	yellowColor      = color("ffd930")
	greyColor        = color("4f545c")
	purpleColor      = color("7289da")
	orangeColor      = color("eb6420")
	orangeColorLight = color("e68d60")
	redColor         = color("ff3232")
)

// JSONPayload Marshals the DiscordPayload to json
func (d *DiscordPayload) JSONPayload() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

var (
	_ PayloadConvertor = &DiscordPayload{}
)

// Create implements PayloadConvertor Create method
func (d *DiscordPayload) Create(p *api.CreatePayload) (api.Payloader, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return d.createPayload(p.Sender, title, "", p.Repo.HTMLURL+"/src/"+refName, greenColor), nil
}

// Delete implements PayloadConvertor Delete method
func (d *DiscordPayload) Delete(p *api.DeletePayload) (api.Payloader, error) {
	// deleted tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return d.createPayload(p.Sender, title, "", p.Repo.HTMLURL+"/src/"+refName, redColor), nil
}

// Fork implements PayloadConvertor Fork method
func (d *DiscordPayload) Fork(p *api.ForkPayload) (api.Payloader, error) {
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return d.createPayload(p.Sender, title, "", p.Repo.HTMLURL, greenColor), nil
}

// Push implements PayloadConvertor Push method
func (d *DiscordPayload) Push(p *api.PushPayload) (api.Payloader, error) {
	var (
		branchName = git.RefEndName(p.Ref)
		commitDesc string
	)

	var titleLink string
	if len(p.Commits) == 1 {
		commitDesc = "1 new commit"
		titleLink = p.Commits[0].URL
	} else {
		commitDesc = fmt.Sprintf("%d new commits", len(p.Commits))
		titleLink = p.CompareURL
	}
	if titleLink == "" {
		titleLink = p.Repo.HTMLURL + "/src/" + branchName
	}

	title := fmt.Sprintf("[%s:%s] %s", p.Repo.FullName, branchName, commitDesc)

	var text string
	// for each commit, generate attachment text
	for i, commit := range p.Commits {
		text += fmt.Sprintf("[%s](%s) %s - %s", commit.ID[:7], commit.URL,
			strings.TrimRight(commit.Message, "\r\n"), commit.Author.Name)
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "\n"
		}
	}

	return d.createPayload(p.Sender, title, text, titleLink, greenColor), nil
}

// Issue implements PayloadConvertor Issue method
func (d *DiscordPayload) Issue(p *api.IssuePayload) (api.Payloader, error) {
	title, _, text, color := getIssuesPayloadInfo(p, noneLinkFormatter, false)

	return d.createPayload(p.Sender, title, text, p.Issue.HTMLURL, color), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (d *DiscordPayload) IssueComment(p *api.IssueCommentPayload) (api.Payloader, error) {
	title, _, color := getIssueCommentPayloadInfo(p, noneLinkFormatter, false)

	return d.createPayload(p.Sender, title, p.Comment.Body, p.Comment.HTMLURL, color), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (d *DiscordPayload) PullRequest(p *api.PullRequestPayload) (api.Payloader, error) {
	title, _, text, color := getPullRequestPayloadInfo(p, noneLinkFormatter, false)

	return d.createPayload(p.Sender, title, text, p.PullRequest.HTMLURL, color), nil
}

// Review implements PayloadConvertor Review method
func (d *DiscordPayload) Review(p *api.PullRequestPayload, event models.HookEventType) (api.Payloader, error) {
	var text, title string
	var color int
	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		title = fmt.Sprintf("[%s] Pull request review %s: #%d %s", p.Repository.FullName, action, p.Index, p.PullRequest.Title)
		text = p.Review.Content

		switch event {
		case models.HookEventPullRequestReviewApproved:
			color = greenColor
		case models.HookEventPullRequestReviewRejected:
			color = redColor
		case models.HookEventPullRequestComment:
			color = greyColor
		default:
			color = yellowColor
		}
	}

	return d.createPayload(p.Sender, title, text, p.PullRequest.HTMLURL, color), nil
}

// Repository implements PayloadConvertor Repository method
func (d *DiscordPayload) Repository(p *api.RepositoryPayload) (api.Payloader, error) {
	var title, url string
	var color int
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		url = p.Repository.HTMLURL
		color = greenColor
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		color = redColor
	}

	return d.createPayload(p.Sender, title, "", url, color), nil
}

// Release implements PayloadConvertor Release method
func (d *DiscordPayload) Release(p *api.ReleasePayload) (api.Payloader, error) {
	text, color := getReleasePayloadInfo(p, noneLinkFormatter, false)

	return d.createPayload(p.Sender, text, p.Release.Note, p.Release.URL, color), nil
}

// GetDiscordPayload converts a discord webhook into a DiscordPayload
func GetDiscordPayload(p api.Payloader, event models.HookEventType, meta string) (api.Payloader, error) {
	s := new(DiscordPayload)

	discord := &DiscordMeta{}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(meta), &discord); err != nil {
		return s, errors.New("GetDiscordPayload meta json:" + err.Error())
	}
	s.Username = discord.Username
	s.AvatarURL = discord.IconURL

	return convertPayloader(s, p, event)
}

func parseHookPullRequestEventType(event models.HookEventType) (string, error) {
	switch event {

	case models.HookEventPullRequestReviewApproved:
		return "approved", nil
	case models.HookEventPullRequestReviewRejected:
		return "rejected", nil
	case models.HookEventPullRequestComment:
		return "comment", nil

	default:
		return "", errors.New("unknown event type")
	}
}

func (d *DiscordPayload) createPayload(s *api.User, title, text, url string, color int) *DiscordPayload {
	return &DiscordPayload{
		Username:  d.Username,
		AvatarURL: d.AvatarURL,
		Embeds: []DiscordEmbed{
			{
				Title:       title,
				Description: text,
				URL:         url,
				Color:       color,
				Author: DiscordEmbedAuthor{
					Name:    s.UserName,
					URL:     setting.AppURL + s.UserName,
					IconURL: s.AvatarURL,
				},
			},
		},
	}
}
