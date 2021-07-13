// Copyright 2019 The Gitea Authors. All rights reserved.
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
	// MSTeamsFact for Fact Structure
	MSTeamsFact struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	// MSTeamsSection is a MessageCard section
	MSTeamsSection struct {
		ActivityTitle    string        `json:"activityTitle"`
		ActivitySubtitle string        `json:"activitySubtitle"`
		ActivityImage    string        `json:"activityImage"`
		Facts            []MSTeamsFact `json:"facts"`
		Text             string        `json:"text"`
	}

	// MSTeamsAction is an action (creates buttons, links etc)
	MSTeamsAction struct {
		Type    string                `json:"@type"`
		Name    string                `json:"name"`
		Targets []MSTeamsActionTarget `json:"targets,omitempty"`
	}

	// MSTeamsActionTarget is the actual link to follow, etc
	MSTeamsActionTarget struct {
		Os  string `json:"os"`
		URI string `json:"uri"`
	}

	// MSTeamsPayload is the parent object
	MSTeamsPayload struct {
		Type            string           `json:"@type"`
		Context         string           `json:"@context"`
		ThemeColor      string           `json:"themeColor"`
		Title           string           `json:"title"`
		Summary         string           `json:"summary"`
		Sections        []MSTeamsSection `json:"sections"`
		PotentialAction []MSTeamsAction  `json:"potentialAction"`
	}
)

// JSONPayload Marshals the MSTeamsPayload to json
func (m *MSTeamsPayload) JSONPayload() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

var (
	_ PayloadConvertor = &MSTeamsPayload{}
)

// Create implements PayloadConvertor Create method
func (m *MSTeamsPayload) Create(p *api.CreatePayload) (api.Payloader, error) {
	// created tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s created", p.Repo.FullName, p.RefType, refName)

	return createMSTeamsPayload(
		p.Repo,
		p.Sender,
		title,
		"",
		p.Repo.HTMLURL+"/src/"+refName,
		greenColor,
		&MSTeamsFact{fmt.Sprintf("%s:", p.RefType), refName},
	), nil
}

// Delete implements PayloadConvertor Delete method
func (m *MSTeamsPayload) Delete(p *api.DeletePayload) (api.Payloader, error) {
	// deleted tag/branch
	refName := git.RefEndName(p.Ref)
	title := fmt.Sprintf("[%s] %s %s deleted", p.Repo.FullName, p.RefType, refName)

	return createMSTeamsPayload(
		p.Repo,
		p.Sender,
		title,
		"",
		p.Repo.HTMLURL+"/src/"+refName,
		yellowColor,
		&MSTeamsFact{fmt.Sprintf("%s:", p.RefType), refName},
	), nil
}

// Fork implements PayloadConvertor Fork method
func (m *MSTeamsPayload) Fork(p *api.ForkPayload) (api.Payloader, error) {
	title := fmt.Sprintf("%s is forked to %s", p.Forkee.FullName, p.Repo.FullName)

	return createMSTeamsPayload(
		p.Repo,
		p.Sender,
		title,
		"",
		p.Repo.HTMLURL,
		greenColor,
		&MSTeamsFact{"Forkee:", p.Forkee.FullName},
	), nil
}

// Push implements PayloadConvertor Push method
func (m *MSTeamsPayload) Push(p *api.PushPayload) (api.Payloader, error) {
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
			text += "\n\n"
		}
	}

	return createMSTeamsPayload(
		p.Repo,
		p.Sender,
		title,
		text,
		titleLink,
		greenColor,
		&MSTeamsFact{"Commit count:", fmt.Sprintf("%d", len(p.Commits))},
	), nil
}

// Issue implements PayloadConvertor Issue method
func (m *MSTeamsPayload) Issue(p *api.IssuePayload) (api.Payloader, error) {
	title, _, attachmentText, color := getIssuesPayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		attachmentText,
		p.Issue.HTMLURL,
		color,
		&MSTeamsFact{"Issue #:", fmt.Sprintf("%d", p.Issue.ID)},
	), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (m *MSTeamsPayload) IssueComment(p *api.IssueCommentPayload) (api.Payloader, error) {
	title, _, color := getIssueCommentPayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		p.Comment.Body,
		p.Comment.HTMLURL,
		color,
		&MSTeamsFact{"Issue #:", fmt.Sprintf("%d", p.Issue.ID)},
	), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (m *MSTeamsPayload) PullRequest(p *api.PullRequestPayload) (api.Payloader, error) {
	title, _, attachmentText, color := getPullRequestPayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		attachmentText,
		p.PullRequest.HTMLURL,
		color,
		&MSTeamsFact{"Pull request #:", fmt.Sprintf("%d", p.PullRequest.ID)},
	), nil
}

// Review implements PayloadConvertor Review method
func (m *MSTeamsPayload) Review(p *api.PullRequestPayload, event models.HookEventType) (api.Payloader, error) {
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

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		text,
		p.PullRequest.HTMLURL,
		color,
		&MSTeamsFact{"Pull request #:", fmt.Sprintf("%d", p.PullRequest.ID)},
	), nil
}

// Repository implements PayloadConvertor Repository method
func (m *MSTeamsPayload) Repository(p *api.RepositoryPayload) (api.Payloader, error) {
	var title, url string
	var color int
	switch p.Action {
	case api.HookRepoCreated:
		title = fmt.Sprintf("[%s] Repository created", p.Repository.FullName)
		url = p.Repository.HTMLURL
		color = greenColor
	case api.HookRepoDeleted:
		title = fmt.Sprintf("[%s] Repository deleted", p.Repository.FullName)
		color = yellowColor
	}

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		"",
		url,
		color,
		nil,
	), nil
}

// Release implements PayloadConvertor Release method
func (m *MSTeamsPayload) Release(p *api.ReleasePayload) (api.Payloader, error) {
	title, color := getReleasePayloadInfo(p, noneLinkFormatter, false)

	return createMSTeamsPayload(
		p.Repository,
		p.Sender,
		title,
		"",
		p.Release.URL,
		color,
		&MSTeamsFact{"Tag:", p.Release.TagName},
	), nil
}

// GetMSTeamsPayload converts a MSTeams webhook into a MSTeamsPayload
func GetMSTeamsPayload(p api.Payloader, event models.HookEventType, meta string) (api.Payloader, error) {
	return convertPayloader(new(MSTeamsPayload), p, event)
}

func createMSTeamsPayload(r *api.Repository, s *api.User, title, text, actionTarget string, color int, fact *MSTeamsFact) *MSTeamsPayload {
	facts := []MSTeamsFact{
		{
			Name:  "Repository:",
			Value: r.FullName,
		},
	}
	if fact != nil {
		facts = append(facts, *fact)
	}

	return &MSTeamsPayload{
		Type:       "MessageCard",
		Context:    "https://schema.org/extensions",
		ThemeColor: fmt.Sprintf("%x", color),
		Title:      title,
		Summary:    title,
		Sections: []MSTeamsSection{
			{
				ActivityTitle:    s.FullName,
				ActivitySubtitle: s.UserName,
				ActivityImage:    s.AvatarURL,
				Text:             text,
				Facts:            facts,
			},
		},
		PotentialAction: []MSTeamsAction{
			{
				Type: "OpenUri",
				Name: "View in Gitea",
				Targets: []MSTeamsActionTarget{
					{
						Os:  "default",
						URI: actionTarget,
					},
				},
			},
		},
	}
}
