// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	jsoniter "github.com/json-iterator/go"
)

const matrixPayloadSizeLimit = 1024 * 64

// MatrixMeta contains the Matrix metadata
type MatrixMeta struct {
	HomeserverURL string `json:"homeserver_url"`
	Room          string `json:"room_id"`
	AccessToken   string `json:"access_token"`
	MessageType   int    `json:"message_type"`
}

var messageTypeText = map[int]string{
	1: "m.notice",
	2: "m.text",
}

// GetMatrixHook returns Matrix metadata
func GetMatrixHook(w *models.Webhook) *MatrixMeta {
	s := &MatrixMeta{}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetMatrixHook(%d): %v", w.ID, err)
	}
	return s
}

// MatrixPayloadUnsafe contains the (unsafe) payload for a Matrix room
type MatrixPayloadUnsafe struct {
	MatrixPayloadSafe
	AccessToken string `json:"access_token"`
}

var (
	_ PayloadConvertor = &MatrixPayloadUnsafe{}
)

// safePayload "converts" a unsafe payload to a safe payload
func (m *MatrixPayloadUnsafe) safePayload() *MatrixPayloadSafe {
	return &MatrixPayloadSafe{
		Body:          m.Body,
		MsgType:       m.MsgType,
		Format:        m.Format,
		FormattedBody: m.FormattedBody,
		Commits:       m.Commits,
	}
}

// MatrixPayloadSafe contains (safe) payload for a Matrix room
type MatrixPayloadSafe struct {
	Body          string               `json:"body"`
	MsgType       string               `json:"msgtype"`
	Format        string               `json:"format"`
	FormattedBody string               `json:"formatted_body"`
	Commits       []*api.PayloadCommit `json:"io.gitea.commits,omitempty"`
}

// JSONPayload Marshals the MatrixPayloadUnsafe to json
func (m *MatrixPayloadUnsafe) JSONPayload() ([]byte, error) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

// MatrixLinkFormatter creates a link compatible with Matrix
func MatrixLinkFormatter(url string, text string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(url), html.EscapeString(text))
}

// MatrixLinkToRef Matrix-formatter link to a repo ref
func MatrixLinkToRef(repoURL, ref string) string {
	refName := git.RefEndName(ref)
	switch {
	case strings.HasPrefix(ref, git.BranchPrefix):
		return MatrixLinkFormatter(repoURL+"/src/branch/"+refName, refName)
	case strings.HasPrefix(ref, git.TagPrefix):
		return MatrixLinkFormatter(repoURL+"/src/tag/"+refName, refName)
	default:
		return MatrixLinkFormatter(repoURL+"/src/commit/"+refName, refName)
	}
}

// Create implements PayloadConvertor Create method
func (m *MatrixPayloadUnsafe) Create(p *api.CreatePayload) (api.Payloader, error) {
	repoLink := MatrixLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	refLink := MatrixLinkToRef(p.Repo.HTMLURL, p.Ref)
	text := fmt.Sprintf("[%s:%s] %s created by %s", repoLink, refLink, p.RefType, p.Sender.UserName)

	return getMatrixPayloadUnsafe(text, nil, m.AccessToken, m.MsgType), nil
}

// Delete composes Matrix payload for delete a branch or tag.
func (m *MatrixPayloadUnsafe) Delete(p *api.DeletePayload) (api.Payloader, error) {
	refName := git.RefEndName(p.Ref)
	repoLink := MatrixLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("[%s:%s] %s deleted by %s", repoLink, refName, p.RefType, p.Sender.UserName)

	return getMatrixPayloadUnsafe(text, nil, m.AccessToken, m.MsgType), nil
}

// Fork composes Matrix payload for forked by a repository.
func (m *MatrixPayloadUnsafe) Fork(p *api.ForkPayload) (api.Payloader, error) {
	baseLink := MatrixLinkFormatter(p.Forkee.HTMLURL, p.Forkee.FullName)
	forkLink := MatrixLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	text := fmt.Sprintf("%s is forked to %s", baseLink, forkLink)

	return getMatrixPayloadUnsafe(text, nil, m.AccessToken, m.MsgType), nil
}

// Issue implements PayloadConvertor Issue method
func (m *MatrixPayloadUnsafe) Issue(p *api.IssuePayload) (api.Payloader, error) {
	text, _, _, _ := getIssuesPayloadInfo(p, MatrixLinkFormatter, true)

	return getMatrixPayloadUnsafe(text, nil, m.AccessToken, m.MsgType), nil
}

// IssueComment implements PayloadConvertor IssueComment method
func (m *MatrixPayloadUnsafe) IssueComment(p *api.IssueCommentPayload) (api.Payloader, error) {
	text, _, _ := getIssueCommentPayloadInfo(p, MatrixLinkFormatter, true)

	return getMatrixPayloadUnsafe(text, nil, m.AccessToken, m.MsgType), nil
}

// Release implements PayloadConvertor Release method
func (m *MatrixPayloadUnsafe) Release(p *api.ReleasePayload) (api.Payloader, error) {
	text, _ := getReleasePayloadInfo(p, MatrixLinkFormatter, true)

	return getMatrixPayloadUnsafe(text, nil, m.AccessToken, m.MsgType), nil
}

// Push implements PayloadConvertor Push method
func (m *MatrixPayloadUnsafe) Push(p *api.PushPayload) (api.Payloader, error) {
	var commitDesc string

	if len(p.Commits) == 1 {
		commitDesc = "1 commit"
	} else {
		commitDesc = fmt.Sprintf("%d commits", len(p.Commits))
	}

	repoLink := MatrixLinkFormatter(p.Repo.HTMLURL, p.Repo.FullName)
	branchLink := MatrixLinkToRef(p.Repo.HTMLURL, p.Ref)
	text := fmt.Sprintf("[%s] %s pushed %s to %s:<br>", repoLink, p.Pusher.UserName, commitDesc, branchLink)

	// for each commit, generate a new line text
	for i, commit := range p.Commits {
		text += fmt.Sprintf("%s: %s - %s", MatrixLinkFormatter(commit.URL, commit.ID[:7]), commit.Message, commit.Author.Name)
		// add linebreak to each commit but the last
		if i < len(p.Commits)-1 {
			text += "<br>"
		}

	}

	return getMatrixPayloadUnsafe(text, p.Commits, m.AccessToken, m.MsgType), nil
}

// PullRequest implements PayloadConvertor PullRequest method
func (m *MatrixPayloadUnsafe) PullRequest(p *api.PullRequestPayload) (api.Payloader, error) {
	text, _, _, _ := getPullRequestPayloadInfo(p, MatrixLinkFormatter, true)

	return getMatrixPayloadUnsafe(text, nil, m.AccessToken, m.MsgType), nil
}

// Review implements PayloadConvertor Review method
func (m *MatrixPayloadUnsafe) Review(p *api.PullRequestPayload, event models.HookEventType) (api.Payloader, error) {
	senderLink := MatrixLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	title := fmt.Sprintf("#%d %s", p.Index, p.PullRequest.Title)
	titleLink := fmt.Sprintf("%s/pulls/%d", p.Repository.HTMLURL, p.Index)
	repoLink := MatrixLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text string

	switch p.Action {
	case api.HookIssueReviewed:
		action, err := parseHookPullRequestEventType(event)
		if err != nil {
			return nil, err
		}

		text = fmt.Sprintf("[%s] Pull request review %s: [%s](%s) by %s", repoLink, action, title, titleLink, senderLink)
	}

	return getMatrixPayloadUnsafe(text, nil, m.AccessToken, m.MsgType), nil
}

// Repository implements PayloadConvertor Repository method
func (m *MatrixPayloadUnsafe) Repository(p *api.RepositoryPayload) (api.Payloader, error) {
	senderLink := MatrixLinkFormatter(setting.AppURL+p.Sender.UserName, p.Sender.UserName)
	repoLink := MatrixLinkFormatter(p.Repository.HTMLURL, p.Repository.FullName)
	var text string

	switch p.Action {
	case api.HookRepoCreated:
		text = fmt.Sprintf("[%s] Repository created by %s", repoLink, senderLink)
	case api.HookRepoDeleted:
		text = fmt.Sprintf("[%s] Repository deleted by %s", repoLink, senderLink)
	}

	return getMatrixPayloadUnsafe(text, nil, m.AccessToken, m.MsgType), nil
}

// GetMatrixPayload converts a Matrix webhook into a MatrixPayloadUnsafe
func GetMatrixPayload(p api.Payloader, event models.HookEventType, meta string) (api.Payloader, error) {
	s := new(MatrixPayloadUnsafe)

	matrix := &MatrixMeta{}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(meta), &matrix); err != nil {
		return s, errors.New("GetMatrixPayload meta json:" + err.Error())
	}

	s.AccessToken = matrix.AccessToken
	s.MsgType = messageTypeText[matrix.MessageType]

	return convertPayloader(s, p, event)
}

func getMatrixPayloadUnsafe(text string, commits []*api.PayloadCommit, accessToken, msgType string) *MatrixPayloadUnsafe {
	p := MatrixPayloadUnsafe{}
	p.AccessToken = accessToken
	p.FormattedBody = text
	p.Body = getMessageBody(text)
	p.Format = "org.matrix.custom.html"
	p.MsgType = msgType
	p.Commits = commits
	return &p
}

var urlRegex = regexp.MustCompile(`<a [^>]*?href="([^">]*?)">(.*?)</a>`)

func getMessageBody(htmlText string) string {
	htmlText = urlRegex.ReplaceAllString(htmlText, "[$2]($1)")
	htmlText = strings.ReplaceAll(htmlText, "<br>", "\n")
	return htmlText
}

// getMatrixHookRequest creates a new request which contains an Authorization header.
// The access_token is removed from t.PayloadContent
func getMatrixHookRequest(w *models.Webhook, t *models.HookTask) (*http.Request, error) {
	payloadunsafe := MatrixPayloadUnsafe{}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal([]byte(t.PayloadContent), &payloadunsafe); err != nil {
		log.Error("Matrix Hook delivery failed: %v", err)
		return nil, err
	}

	payloadsafe := payloadunsafe.safePayload()

	var payload []byte
	var err error
	if payload, err = json.MarshalIndent(payloadsafe, "", "  "); err != nil {
		return nil, err
	}
	if len(payload) >= matrixPayloadSizeLimit {
		return nil, fmt.Errorf("getMatrixHookRequest: payload size %d > %d", len(payload), matrixPayloadSizeLimit)
	}
	t.PayloadContent = string(payload)

	txnID, err := getMatrixTxnID(payload)
	if err != nil {
		return nil, fmt.Errorf("getMatrixHookRequest: unable to hash payload: %+v", err)
	}

	url := fmt.Sprintf("%s/%s", w.URL, txnID)

	req, err := http.NewRequest(w.HTTPMethod, url, strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+payloadunsafe.AccessToken)

	return req, nil
}

// getMatrixTxnID creates a txnID based on the payload to ensure idempotency
func getMatrixTxnID(payload []byte) (string, error) {
	h := sha1.New()
	_, err := h.Write(payload)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
