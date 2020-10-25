// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"encoding/json"
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"

	gouuid "github.com/google/uuid"
)

// HookContentType is the content type of a web hook
type HookContentType int

const (
	// ContentTypeJSON is a JSON payload for web hooks
	ContentTypeJSON HookContentType = iota + 1
	// ContentTypeForm is an url-encoded form payload for web hook
	ContentTypeForm
)

var hookContentTypes = map[string]HookContentType{
	"json": ContentTypeJSON,
	"form": ContentTypeForm,
}

// ToHookContentType returns HookContentType by given name.
func ToHookContentType(name string) HookContentType {
	return hookContentTypes[name]
}

// Name returns the name of a given web hook's content type
func (t HookContentType) Name() string {
	switch t {
	case ContentTypeJSON:
		return "json"
	case ContentTypeForm:
		return "form"
	}
	return ""
}

// IsValidHookContentType returns true if given name is a valid hook content type.
func IsValidHookContentType(name string) bool {
	_, ok := hookContentTypes[name]
	return ok
}

// HookEvents is a set of web hook events
type HookEvents struct {
	Create               bool `json:"create"`
	Delete               bool `json:"delete"`
	Fork                 bool `json:"fork"`
	Issues               bool `json:"issues"`
	IssueAssign          bool `json:"issue_assign"`
	IssueLabel           bool `json:"issue_label"`
	IssueMilestone       bool `json:"issue_milestone"`
	IssueComment         bool `json:"issue_comment"`
	Push                 bool `json:"push"`
	PullRequest          bool `json:"pull_request"`
	PullRequestAssign    bool `json:"pull_request_assign"`
	PullRequestLabel     bool `json:"pull_request_label"`
	PullRequestMilestone bool `json:"pull_request_milestone"`
	PullRequestComment   bool `json:"pull_request_comment"`
	PullRequestReview    bool `json:"pull_request_review"`
	PullRequestSync      bool `json:"pull_request_sync"`
	Repository           bool `json:"repository"`
	Release              bool `json:"release"`
}

// HookEvent represents events that will delivery hook.
type HookEvent struct {
	PushOnly       bool   `json:"push_only"`
	SendEverything bool   `json:"send_everything"`
	ChooseEvents   bool   `json:"choose_events"`
	BranchFilter   string `json:"branch_filter"`

	HookEvents `json:"events"`
}

// HookStatus is the status of a web hook
type HookStatus int

// Possible statuses of a web hook
const (
	HookStatusNone = iota
	HookStatusSucceed
	HookStatusFail
)

// Webhook represents a web hook object.
type Webhook struct {
	ID              int64 `xorm:"pk autoincr"`
	RepoID          int64 `xorm:"INDEX"` // An ID of 0 indicates either a default or system webhook
	OrgID           int64 `xorm:"INDEX"`
	IsSystemWebhook bool
	URL             string `xorm:"url TEXT"`
	Signature       string `xorm:"TEXT"`
	HTTPMethod      string `xorm:"http_method"`
	ContentType     HookContentType
	Secret          string `xorm:"TEXT"`
	Events          string `xorm:"TEXT"`
	*HookEvent      `xorm:"-"`
	IsSSL           bool `xorm:"is_ssl"`
	IsActive        bool `xorm:"INDEX"`
	HookTaskType    HookTaskType
	Meta            string     `xorm:"TEXT"` // store hook-specific attributes
	LastStatus      HookStatus // Last delivery status

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
}

// AfterLoad updates the webhook object upon setting a column
func (w *Webhook) AfterLoad() {
	w.HookEvent = &HookEvent{}
	if err := json.Unmarshal([]byte(w.Events), w.HookEvent); err != nil {
		log.Error("Unmarshal[%d]: %v", w.ID, err)
	}
}

// History returns history of webhook by given conditions.
func (w *Webhook) History(page int) ([]*HookTask, error) {
	return HookTasks(w.ID, page)
}

// UpdateEvent handles conversion from HookEvent to Events.
func (w *Webhook) UpdateEvent() error {
	data, err := json.Marshal(w.HookEvent)
	w.Events = string(data)
	return err
}

// HasCreateEvent returns true if hook enabled create event.
func (w *Webhook) HasCreateEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Create)
}

// HasDeleteEvent returns true if hook enabled delete event.
func (w *Webhook) HasDeleteEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Delete)
}

// HasForkEvent returns true if hook enabled fork event.
func (w *Webhook) HasForkEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Fork)
}

// HasIssuesEvent returns true if hook enabled issues event.
func (w *Webhook) HasIssuesEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Issues)
}

// HasIssuesAssignEvent returns true if hook enabled issues assign event.
func (w *Webhook) HasIssuesAssignEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.IssueAssign)
}

// HasIssuesLabelEvent returns true if hook enabled issues label event.
func (w *Webhook) HasIssuesLabelEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.IssueLabel)
}

// HasIssuesMilestoneEvent returns true if hook enabled issues milestone event.
func (w *Webhook) HasIssuesMilestoneEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.IssueMilestone)
}

// HasIssueCommentEvent returns true if hook enabled issue_comment event.
func (w *Webhook) HasIssueCommentEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.IssueComment)
}

// HasPushEvent returns true if hook enabled push event.
func (w *Webhook) HasPushEvent() bool {
	return w.PushOnly || w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Push)
}

// HasPullRequestEvent returns true if hook enabled pull request event.
func (w *Webhook) HasPullRequestEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequest)
}

// HasPullRequestAssignEvent returns true if hook enabled pull request assign event.
func (w *Webhook) HasPullRequestAssignEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestAssign)
}

// HasPullRequestLabelEvent returns true if hook enabled pull request label event.
func (w *Webhook) HasPullRequestLabelEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestLabel)
}

// HasPullRequestMilestoneEvent returns true if hook enabled pull request milestone event.
func (w *Webhook) HasPullRequestMilestoneEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestMilestone)
}

// HasPullRequestCommentEvent returns true if hook enabled pull_request_comment event.
func (w *Webhook) HasPullRequestCommentEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestComment)
}

// HasPullRequestApprovedEvent returns true if hook enabled pull request review event.
func (w *Webhook) HasPullRequestApprovedEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestReview)
}

// HasPullRequestRejectedEvent returns true if hook enabled pull request review event.
func (w *Webhook) HasPullRequestRejectedEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestReview)
}

// HasPullRequestReviewCommentEvent returns true if hook enabled pull request review event.
func (w *Webhook) HasPullRequestReviewCommentEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestReview)
}

// HasPullRequestSyncEvent returns true if hook enabled pull request sync event.
func (w *Webhook) HasPullRequestSyncEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.PullRequestSync)
}

// HasReleaseEvent returns if hook enabled release event.
func (w *Webhook) HasReleaseEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Release)
}

// HasRepositoryEvent returns if hook enabled repository event.
func (w *Webhook) HasRepositoryEvent() bool {
	return w.SendEverything ||
		(w.ChooseEvents && w.HookEvents.Repository)
}

// EventCheckers returns event checkers
func (w *Webhook) EventCheckers() []struct {
	Has  func() bool
	Type HookEventType
} {
	return []struct {
		Has  func() bool
		Type HookEventType
	}{
		{w.HasCreateEvent, HookEventCreate},
		{w.HasDeleteEvent, HookEventDelete},
		{w.HasForkEvent, HookEventFork},
		{w.HasPushEvent, HookEventPush},
		{w.HasIssuesEvent, HookEventIssues},
		{w.HasIssuesAssignEvent, HookEventIssueAssign},
		{w.HasIssuesLabelEvent, HookEventIssueLabel},
		{w.HasIssuesMilestoneEvent, HookEventIssueMilestone},
		{w.HasIssueCommentEvent, HookEventIssueComment},
		{w.HasPullRequestEvent, HookEventPullRequest},
		{w.HasPullRequestAssignEvent, HookEventPullRequestAssign},
		{w.HasPullRequestLabelEvent, HookEventPullRequestLabel},
		{w.HasPullRequestMilestoneEvent, HookEventPullRequestMilestone},
		{w.HasPullRequestCommentEvent, HookEventPullRequestComment},
		{w.HasPullRequestApprovedEvent, HookEventPullRequestReviewApproved},
		{w.HasPullRequestRejectedEvent, HookEventPullRequestReviewRejected},
		{w.HasPullRequestCommentEvent, HookEventPullRequestReviewComment},
		{w.HasPullRequestSyncEvent, HookEventPullRequestSync},
		{w.HasRepositoryEvent, HookEventRepository},
		{w.HasReleaseEvent, HookEventRelease},
	}
}

// EventsArray returns an array of hook events
func (w *Webhook) EventsArray() []string {
	events := make([]string, 0, 7)

	for _, c := range w.EventCheckers() {
		if c.Has() {
			events = append(events, string(c.Type))
		}
	}
	return events
}

// CreateWebhook creates a new web hook.
func CreateWebhook(w *Webhook) error {
	return createWebhook(x, w)
}

func createWebhook(e Engine, w *Webhook) error {
	_, err := e.Insert(w)
	return err
}

// getWebhook uses argument bean as query condition,
// ID must be specified and do not assign unnecessary fields.
func getWebhook(bean *Webhook) (*Webhook, error) {
	has, err := x.Get(bean)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrWebhookNotExist{bean.ID}
	}
	return bean, nil
}

// GetWebhookByID returns webhook of repository by given ID.
func GetWebhookByID(id int64) (*Webhook, error) {
	return getWebhook(&Webhook{
		ID: id,
	})
}

// GetWebhookByRepoID returns webhook of repository by given ID.
func GetWebhookByRepoID(repoID, id int64) (*Webhook, error) {
	return getWebhook(&Webhook{
		ID:     id,
		RepoID: repoID,
	})
}

// GetWebhookByOrgID returns webhook of organization by given ID.
func GetWebhookByOrgID(orgID, id int64) (*Webhook, error) {
	return getWebhook(&Webhook{
		ID:    id,
		OrgID: orgID,
	})
}

// GetActiveWebhooksByRepoID returns all active webhooks of repository.
func GetActiveWebhooksByRepoID(repoID int64) ([]*Webhook, error) {
	return getActiveWebhooksByRepoID(x, repoID)
}

func getActiveWebhooksByRepoID(e Engine, repoID int64) ([]*Webhook, error) {
	webhooks := make([]*Webhook, 0, 5)
	return webhooks, e.Where("is_active=?", true).
		Find(&webhooks, &Webhook{RepoID: repoID})
}

// GetWebhooksByRepoID returns all webhooks of a repository.
func GetWebhooksByRepoID(repoID int64, listOptions ListOptions) ([]*Webhook, error) {
	if listOptions.Page == 0 {
		webhooks := make([]*Webhook, 0, 5)
		return webhooks, x.Find(&webhooks, &Webhook{RepoID: repoID})
	}

	sess := listOptions.getPaginatedSession()
	webhooks := make([]*Webhook, 0, listOptions.PageSize)

	return webhooks, sess.Find(&webhooks, &Webhook{RepoID: repoID})
}

// GetActiveWebhooksByOrgID returns all active webhooks for an organization.
func GetActiveWebhooksByOrgID(orgID int64) (ws []*Webhook, err error) {
	return getActiveWebhooksByOrgID(x, orgID)
}

func getActiveWebhooksByOrgID(e Engine, orgID int64) (ws []*Webhook, err error) {
	err = e.
		Where("org_id=?", orgID).
		And("is_active=?", true).
		Find(&ws)
	return ws, err
}

// GetWebhooksByOrgID returns paginated webhooks for an organization.
func GetWebhooksByOrgID(orgID int64, listOptions ListOptions) ([]*Webhook, error) {
	if listOptions.Page == 0 {
		ws := make([]*Webhook, 0, 5)
		return ws, x.Find(&ws, &Webhook{OrgID: orgID})
	}

	sess := listOptions.getPaginatedSession()
	ws := make([]*Webhook, 0, listOptions.PageSize)
	return ws, sess.Find(&ws, &Webhook{OrgID: orgID})
}

// GetDefaultWebhook returns admin-default webhook by given ID.
func GetDefaultWebhook(id int64) (*Webhook, error) {
	webhook := &Webhook{ID: id}
	has, err := x.
		Where("repo_id=? AND org_id=? AND is_system_webhook=?", 0, 0, false).
		Get(webhook)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrWebhookNotExist{id}
	}
	return webhook, nil
}

// GetDefaultWebhooks returns all admin-default webhooks.
func GetDefaultWebhooks() ([]*Webhook, error) {
	return getDefaultWebhooks(x)
}

func getDefaultWebhooks(e Engine) ([]*Webhook, error) {
	webhooks := make([]*Webhook, 0, 5)
	return webhooks, e.
		Where("repo_id=? AND org_id=? AND is_system_webhook=?", 0, 0, false).
		Find(&webhooks)
}

// GetSystemWebhook returns admin system webhook by given ID.
func GetSystemWebhook(id int64) (*Webhook, error) {
	webhook := &Webhook{ID: id}
	has, err := x.
		Where("repo_id=? AND org_id=? AND is_system_webhook=?", 0, 0, true).
		Get(webhook)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrWebhookNotExist{id}
	}
	return webhook, nil
}

// GetSystemWebhooks returns all admin system webhooks.
func GetSystemWebhooks() ([]*Webhook, error) {
	return getSystemWebhooks(x)
}

func getSystemWebhooks(e Engine) ([]*Webhook, error) {
	webhooks := make([]*Webhook, 0, 5)
	return webhooks, e.
		Where("repo_id=? AND org_id=? AND is_system_webhook=?", 0, 0, true).
		Find(&webhooks)
}

// UpdateWebhook updates information of webhook.
func UpdateWebhook(w *Webhook) error {
	_, err := x.ID(w.ID).AllCols().Update(w)
	return err
}

// UpdateWebhookLastStatus updates last status of webhook.
func UpdateWebhookLastStatus(w *Webhook) error {
	_, err := x.ID(w.ID).Cols("last_status").Update(w)
	return err
}

// deleteWebhook uses argument bean as query condition,
// ID must be specified and do not assign unnecessary fields.
func deleteWebhook(bean *Webhook) (err error) {
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if count, err := sess.Delete(bean); err != nil {
		return err
	} else if count == 0 {
		return ErrWebhookNotExist{ID: bean.ID}
	} else if _, err = sess.Delete(&HookTask{HookID: bean.ID}); err != nil {
		return err
	}

	return sess.Commit()
}

// DeleteWebhookByRepoID deletes webhook of repository by given ID.
func DeleteWebhookByRepoID(repoID, id int64) error {
	return deleteWebhook(&Webhook{
		ID:     id,
		RepoID: repoID,
	})
}

// DeleteWebhookByOrgID deletes webhook of organization by given ID.
func DeleteWebhookByOrgID(orgID, id int64) error {
	return deleteWebhook(&Webhook{
		ID:    id,
		OrgID: orgID,
	})
}

// DeleteDefaultSystemWebhook deletes an admin-configured default or system webhook (where Org and Repo ID both 0)
func DeleteDefaultSystemWebhook(id int64) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	count, err := sess.
		Where("repo_id=? AND org_id=?", 0, 0).
		Delete(&Webhook{ID: id})
	if err != nil {
		return err
	} else if count == 0 {
		return ErrWebhookNotExist{ID: id}
	}

	if _, err := sess.Delete(&HookTask{HookID: id}); err != nil {
		return err
	}

	return sess.Commit()
}

// copyDefaultWebhooksToRepo creates copies of the default webhooks in a new repo
func copyDefaultWebhooksToRepo(e Engine, repoID int64) error {
	ws, err := getDefaultWebhooks(e)
	if err != nil {
		return fmt.Errorf("GetDefaultWebhooks: %v", err)
	}

	for _, w := range ws {
		w.ID = 0
		w.RepoID = repoID
		if err := createWebhook(e, w); err != nil {
			return fmt.Errorf("CreateWebhook: %v", err)
		}
	}
	return nil
}

//   ___ ___                __   ___________              __
//  /   |   \  ____   ____ |  | _\__    ___/____    _____|  | __
// /    ~    \/  _ \ /  _ \|  |/ / |    |  \__  \  /  ___/  |/ /
// \    Y    (  <_> |  <_> )    <  |    |   / __ \_\___ \|    <
//  \___|_  / \____/ \____/|__|_ \ |____|  (____  /____  >__|_ \
//        \/                    \/              \/     \/     \/

// HookTaskType is the type of an hook task
type HookTaskType int

// Types of hook tasks
const (
	GOGS HookTaskType = iota + 1
	SLACK
	GITEA
	DISCORD
	DINGTALK
	TELEGRAM
	MSTEAMS
	FEISHU
	MATRIX
	WORKWECHAT
)

var hookTaskTypes = map[string]HookTaskType{
	"gitea":      GITEA,
	"gogs":       GOGS,
	"slack":      SLACK,
	"discord":    DISCORD,
	"dingtalk":   DINGTALK,
	"telegram":   TELEGRAM,
	"msteams":    MSTEAMS,
	"feishu":     FEISHU,
	"matrix":     MATRIX,
	"workwechat": WORKWECHAT,
}

// ToHookTaskType returns HookTaskType by given name.
func ToHookTaskType(name string) HookTaskType {
	return hookTaskTypes[name]
}

// Name returns the name of an hook task type
func (t HookTaskType) Name() string {
	switch t {
	case GITEA:
		return "gitea"
	case GOGS:
		return "gogs"
	case SLACK:
		return "slack"
	case DISCORD:
		return "discord"
	case DINGTALK:
		return "dingtalk"
	case TELEGRAM:
		return "telegram"
	case MSTEAMS:
		return "msteams"
	case FEISHU:
		return "feishu"
	case MATRIX:
		return "matrix"
	case WORKWECHAT:
		return "workwechat"
	}
	return ""
}

// IsValidHookTaskType returns true if given name is a valid hook task type.
func IsValidHookTaskType(name string) bool {
	_, ok := hookTaskTypes[name]
	return ok
}

// HookEventType is the type of an hook event
type HookEventType string

// Types of hook events
const (
	HookEventCreate                    HookEventType = "create"
	HookEventDelete                    HookEventType = "delete"
	HookEventFork                      HookEventType = "fork"
	HookEventPush                      HookEventType = "push"
	HookEventIssues                    HookEventType = "issues"
	HookEventIssueAssign               HookEventType = "issue_assign"
	HookEventIssueLabel                HookEventType = "issue_label"
	HookEventIssueMilestone            HookEventType = "issue_milestone"
	HookEventIssueComment              HookEventType = "issue_comment"
	HookEventPullRequest               HookEventType = "pull_request"
	HookEventPullRequestAssign         HookEventType = "pull_request_assign"
	HookEventPullRequestLabel          HookEventType = "pull_request_label"
	HookEventPullRequestMilestone      HookEventType = "pull_request_milestone"
	HookEventPullRequestComment        HookEventType = "pull_request_comment"
	HookEventPullRequestReviewApproved HookEventType = "pull_request_review_approved"
	HookEventPullRequestReviewRejected HookEventType = "pull_request_review_rejected"
	HookEventPullRequestReviewComment  HookEventType = "pull_request_review_comment"
	HookEventPullRequestSync           HookEventType = "pull_request_sync"
	HookEventRepository                HookEventType = "repository"
	HookEventRelease                   HookEventType = "release"
)

// Event returns the HookEventType as an event string
func (h HookEventType) Event() string {
	switch h {
	case HookEventCreate:
		return "create"
	case HookEventDelete:
		return "delete"
	case HookEventFork:
		return "fork"
	case HookEventPush:
		return "push"
	case HookEventIssues, HookEventIssueAssign, HookEventIssueLabel, HookEventIssueMilestone:
		return "issues"
	case HookEventPullRequest, HookEventPullRequestAssign, HookEventPullRequestLabel, HookEventPullRequestMilestone,
		HookEventPullRequestSync:
		return "pull_request"
	case HookEventIssueComment, HookEventPullRequestComment:
		return "issue_comment"
	case HookEventPullRequestReviewApproved:
		return "pull_request_approved"
	case HookEventPullRequestReviewRejected:
		return "pull_request_rejected"
	case HookEventPullRequestReviewComment:
		return "pull_request_comment"
	case HookEventRepository:
		return "repository"
	case HookEventRelease:
		return "release"
	}
	return ""
}

// HookRequest represents hook task request information.
type HookRequest struct {
	Headers map[string]string `json:"headers"`
}

// HookResponse represents hook task response information.
type HookResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// HookTask represents a hook task.
type HookTask struct {
	ID              int64 `xorm:"pk autoincr"`
	RepoID          int64 `xorm:"INDEX"`
	HookID          int64
	UUID            string
	Type            HookTaskType
	URL             string `xorm:"TEXT"`
	Signature       string `xorm:"TEXT"`
	api.Payloader   `xorm:"-"`
	PayloadContent  string `xorm:"TEXT"`
	HTTPMethod      string `xorm:"http_method"`
	ContentType     HookContentType
	EventType       HookEventType
	IsSSL           bool
	IsDelivered     bool
	Delivered       int64
	DeliveredString string `xorm:"-"`

	// History info.
	IsSucceed       bool
	RequestContent  string        `xorm:"TEXT"`
	RequestInfo     *HookRequest  `xorm:"-"`
	ResponseContent string        `xorm:"TEXT"`
	ResponseInfo    *HookResponse `xorm:"-"`
}

// BeforeUpdate will be invoked by XORM before updating a record
// representing this object
func (t *HookTask) BeforeUpdate() {
	if t.RequestInfo != nil {
		t.RequestContent = t.simpleMarshalJSON(t.RequestInfo)
	}
	if t.ResponseInfo != nil {
		t.ResponseContent = t.simpleMarshalJSON(t.ResponseInfo)
	}
}

// AfterLoad updates the webhook object upon setting a column
func (t *HookTask) AfterLoad() {
	t.DeliveredString = time.Unix(0, t.Delivered).Format("2006-01-02 15:04:05 MST")

	if len(t.RequestContent) == 0 {
		return
	}

	t.RequestInfo = &HookRequest{}
	if err := json.Unmarshal([]byte(t.RequestContent), t.RequestInfo); err != nil {
		log.Error("Unmarshal RequestContent[%d]: %v", t.ID, err)
	}

	if len(t.ResponseContent) > 0 {
		t.ResponseInfo = &HookResponse{}
		if err := json.Unmarshal([]byte(t.ResponseContent), t.ResponseInfo); err != nil {
			log.Error("Unmarshal ResponseContent[%d]: %v", t.ID, err)
		}
	}
}

func (t *HookTask) simpleMarshalJSON(v interface{}) string {
	p, err := json.Marshal(v)
	if err != nil {
		log.Error("Marshal [%d]: %v", t.ID, err)
	}
	return string(p)
}

// HookTasks returns a list of hook tasks by given conditions.
func HookTasks(hookID int64, page int) ([]*HookTask, error) {
	tasks := make([]*HookTask, 0, setting.Webhook.PagingNum)
	return tasks, x.
		Limit(setting.Webhook.PagingNum, (page-1)*setting.Webhook.PagingNum).
		Where("hook_id=?", hookID).
		Desc("id").
		Find(&tasks)
}

// CreateHookTask creates a new hook task,
// it handles conversion from Payload to PayloadContent.
func CreateHookTask(t *HookTask) error {
	return createHookTask(x, t)
}

func createHookTask(e Engine, t *HookTask) error {
	data, err := t.Payloader.JSONPayload()
	if err != nil {
		return err
	}
	t.UUID = gouuid.New().String()
	t.PayloadContent = string(data)
	_, err = e.Insert(t)
	return err
}

// UpdateHookTask updates information of hook task.
func UpdateHookTask(t *HookTask) error {
	_, err := x.ID(t.ID).AllCols().Update(t)
	return err
}

// FindUndeliveredHookTasks represents find the undelivered hook tasks
func FindUndeliveredHookTasks() ([]*HookTask, error) {
	tasks := make([]*HookTask, 0, 10)
	if err := x.Where("is_delivered=?", false).Find(&tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// FindRepoUndeliveredHookTasks represents find the undelivered hook tasks of one repository
func FindRepoUndeliveredHookTasks(repoID int64) ([]*HookTask, error) {
	tasks := make([]*HookTask, 0, 5)
	if err := x.Where("repo_id=? AND is_delivered=?", repoID, false).Find(&tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}
