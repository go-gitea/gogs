// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"context"
	"testing"
	"time"

	api "code.gitea.io/gitea/modules/structs"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestHookContentType_Name(t *testing.T) {
	assert.Equal(t, "json", ContentTypeJSON.Name())
	assert.Equal(t, "form", ContentTypeForm.Name())
}

func TestIsValidHookContentType(t *testing.T) {
	assert.True(t, IsValidHookContentType("json"))
	assert.True(t, IsValidHookContentType("form"))
	assert.False(t, IsValidHookContentType("invalid"))
}

func TestWebhook_History(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	webhook := AssertExistsAndLoadBean(t, &Webhook{ID: 1}).(*Webhook)
	tasks, err := webhook.History(0)
	assert.NoError(t, err)
	if assert.Len(t, tasks, 1) {
		assert.Equal(t, int64(1), tasks[0].ID)
	}

	webhook = AssertExistsAndLoadBean(t, &Webhook{ID: 2}).(*Webhook)
	tasks, err = webhook.History(0)
	assert.NoError(t, err)
	assert.Len(t, tasks, 0)
}

func TestWebhook_UpdateEvent(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	webhook := AssertExistsAndLoadBean(t, &Webhook{ID: 1}).(*Webhook)
	hookEvent := &HookEvent{
		PushOnly:       true,
		SendEverything: false,
		ChooseEvents:   false,
		HookEvents: HookEvents{
			Create:      false,
			Push:        true,
			PullRequest: false,
		},
	}
	webhook.HookEvent = hookEvent
	assert.NoError(t, webhook.UpdateEvent())
	assert.NotEmpty(t, webhook.Events)
	actualHookEvent := &HookEvent{}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	assert.NoError(t, json.Unmarshal([]byte(webhook.Events), actualHookEvent))
	assert.Equal(t, *hookEvent, *actualHookEvent)
}

func TestWebhook_EventsArray(t *testing.T) {
	assert.Equal(t, []string{
		"create", "delete", "fork", "push",
		"issues", "issue_assign", "issue_label", "issue_milestone", "issue_comment",
		"pull_request", "pull_request_assign", "pull_request_label", "pull_request_milestone",
		"pull_request_comment", "pull_request_review_approved", "pull_request_review_rejected",
		"pull_request_review_comment", "pull_request_sync", "repository", "release",
	},
		(&Webhook{
			HookEvent: &HookEvent{SendEverything: true},
		}).EventsArray(),
	)

	assert.Equal(t, []string{"push"},
		(&Webhook{
			HookEvent: &HookEvent{PushOnly: true},
		}).EventsArray(),
	)
}

func TestCreateWebhook(t *testing.T) {
	hook := &Webhook{
		RepoID:      3,
		URL:         "www.example.com/unit_test",
		ContentType: ContentTypeJSON,
		Events:      `{"push_only":false,"send_everything":false,"choose_events":false,"events":{"create":false,"push":true,"pull_request":true}}`,
	}
	AssertNotExistsBean(t, hook)
	assert.NoError(t, CreateWebhook(hook))
	AssertExistsAndLoadBean(t, hook)
}

func TestGetWebhookByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hook, err := GetWebhookByRepoID(1, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), hook.ID)

	_, err = GetWebhookByRepoID(NonexistentID, NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestGetWebhookByOrgID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hook, err := GetWebhookByOrgID(3, 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), hook.ID)

	_, err = GetWebhookByOrgID(NonexistentID, NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestGetActiveWebhooksByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hooks, err := GetActiveWebhooksByRepoID(1)
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(1), hooks[0].ID)
		assert.True(t, hooks[0].IsActive)
	}
}

func TestGetWebhooksByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hooks, err := GetWebhooksByRepoID(1, ListOptions{})
	assert.NoError(t, err)
	if assert.Len(t, hooks, 2) {
		assert.Equal(t, int64(1), hooks[0].ID)
		assert.Equal(t, int64(2), hooks[1].ID)
	}
}

func TestGetActiveWebhooksByOrgID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hooks, err := GetActiveWebhooksByOrgID(3)
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(3), hooks[0].ID)
		assert.True(t, hooks[0].IsActive)
	}
}

func TestGetWebhooksByOrgID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hooks, err := GetWebhooksByOrgID(3, ListOptions{})
	assert.NoError(t, err)
	if assert.Len(t, hooks, 1) {
		assert.Equal(t, int64(3), hooks[0].ID)
		assert.True(t, hooks[0].IsActive)
	}
}

func TestUpdateWebhook(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hook := AssertExistsAndLoadBean(t, &Webhook{ID: 2}).(*Webhook)
	hook.IsActive = true
	hook.ContentType = ContentTypeForm
	AssertNotExistsBean(t, hook)
	assert.NoError(t, UpdateWebhook(hook))
	AssertExistsAndLoadBean(t, hook)
}

func TestDeleteWebhookByRepoID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	AssertExistsAndLoadBean(t, &Webhook{ID: 2, RepoID: 1})
	assert.NoError(t, DeleteWebhookByRepoID(1, 2))
	AssertNotExistsBean(t, &Webhook{ID: 2, RepoID: 1})

	err := DeleteWebhookByRepoID(NonexistentID, NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestDeleteWebhookByOrgID(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	AssertExistsAndLoadBean(t, &Webhook{ID: 3, OrgID: 3})
	assert.NoError(t, DeleteWebhookByOrgID(3, 3))
	AssertNotExistsBean(t, &Webhook{ID: 3, OrgID: 3})

	err := DeleteWebhookByOrgID(NonexistentID, NonexistentID)
	assert.Error(t, err)
	assert.True(t, IsErrWebhookNotExist(err))
}

func TestHookTasks(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hookTasks, err := HookTasks(1, 1)
	assert.NoError(t, err)
	if assert.Len(t, hookTasks, 1) {
		assert.Equal(t, int64(1), hookTasks[0].ID)
	}

	hookTasks, err = HookTasks(NonexistentID, 1)
	assert.NoError(t, err)
	assert.Len(t, hookTasks, 0)
}

func TestCreateHookTask(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hookTask := &HookTask{
		RepoID:    3,
		HookID:    3,
		Payloader: &api.PushPayload{},
	}
	AssertNotExistsBean(t, hookTask)
	assert.NoError(t, CreateHookTask(hookTask))
	AssertExistsAndLoadBean(t, hookTask)
}

func TestUpdateHookTask(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	hook := AssertExistsAndLoadBean(t, &HookTask{ID: 1}).(*HookTask)
	hook.PayloadContent = "new payload content"
	hook.DeliveredString = "new delivered string"
	hook.IsDelivered = true
	AssertNotExistsBean(t, hook)
	assert.NoError(t, UpdateHookTask(hook))
	AssertExistsAndLoadBean(t, hook)
}

func TestCleanupHookTaskTable_PerWebhook_DeletesDelivered(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hookTask := &HookTask{
		RepoID:      3,
		HookID:      3,
		Payloader:   &api.PushPayload{},
		IsDelivered: true,
		Delivered:   time.Now().UnixNano(),
	}
	AssertNotExistsBean(t, hookTask)
	assert.NoError(t, CreateHookTask(hookTask))
	AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), PerWebhook, 168*time.Hour, 0))
	AssertNotExistsBean(t, hookTask)
}

func TestCleanupHookTaskTable_PerWebhook_LeavesUndelivered(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hookTask := &HookTask{
		RepoID:      2,
		HookID:      4,
		Payloader:   &api.PushPayload{},
		IsDelivered: false,
	}
	AssertNotExistsBean(t, hookTask)
	assert.NoError(t, CreateHookTask(hookTask))
	AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), PerWebhook, 168*time.Hour, 0))
	AssertExistsAndLoadBean(t, hookTask)
}

func TestCleanupHookTaskTable_PerWebhook_LeavesMostRecentTask(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hookTask := &HookTask{
		RepoID:      2,
		HookID:      4,
		Payloader:   &api.PushPayload{},
		IsDelivered: true,
		Delivered:   time.Now().UnixNano(),
	}
	AssertNotExistsBean(t, hookTask)
	assert.NoError(t, CreateHookTask(hookTask))
	AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), PerWebhook, 168*time.Hour, 1))
	AssertExistsAndLoadBean(t, hookTask)
}

func TestCleanupHookTaskTable_OlderThan_DeletesDelivered(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hookTask := &HookTask{
		RepoID:      3,
		HookID:      3,
		Payloader:   &api.PushPayload{},
		IsDelivered: true,
		Delivered:   time.Now().AddDate(0, 0, -8).UnixNano(),
	}
	AssertNotExistsBean(t, hookTask)
	assert.NoError(t, CreateHookTask(hookTask))
	AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), OlderThan, 168*time.Hour, 0))
	AssertNotExistsBean(t, hookTask)
}

func TestCleanupHookTaskTable_OlderThan_LeavesUndelivered(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hookTask := &HookTask{
		RepoID:      2,
		HookID:      4,
		Payloader:   &api.PushPayload{},
		IsDelivered: false,
	}
	AssertNotExistsBean(t, hookTask)
	assert.NoError(t, CreateHookTask(hookTask))
	AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), OlderThan, 168*time.Hour, 0))
	AssertExistsAndLoadBean(t, hookTask)
}

func TestCleanupHookTaskTable_OlderThan_LeavesTaskEarlierThanAgeToDelete(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	hookTask := &HookTask{
		RepoID:      2,
		HookID:      4,
		Payloader:   &api.PushPayload{},
		IsDelivered: true,
		Delivered:   time.Now().AddDate(0, 0, -6).UnixNano(),
	}
	AssertNotExistsBean(t, hookTask)
	assert.NoError(t, CreateHookTask(hookTask))
	AssertExistsAndLoadBean(t, hookTask)

	assert.NoError(t, CleanupHookTaskTable(context.Background(), OlderThan, 168*time.Hour, 0))
	AssertExistsAndLoadBean(t, hookTask)
}
