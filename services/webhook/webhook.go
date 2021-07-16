// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"fmt"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/sync"
	"github.com/gobwas/glob"
)

type webhook struct {
	name           models.HookType
	payloadCreator func(p api.Payloader, event models.HookEventType, meta string) (api.Payloader, error)
}

var (
	webhooks = map[models.HookType]*webhook{
		models.SLACK: {
			name:           models.SLACK,
			payloadCreator: GetSlackPayload,
		},
		models.DISCORD: {
			name:           models.DISCORD,
			payloadCreator: GetDiscordPayload,
		},
		models.DINGTALK: {
			name:           models.DINGTALK,
			payloadCreator: GetDingtalkPayload,
		},
		models.TELEGRAM: {
			name:           models.TELEGRAM,
			payloadCreator: GetTelegramPayload,
		},
		models.MSTEAMS: {
			name:           models.MSTEAMS,
			payloadCreator: GetMSTeamsPayload,
		},
		models.FEISHU: {
			name:           models.FEISHU,
			payloadCreator: GetFeishuPayload,
		},
		models.MATRIX: {
			name:           models.MATRIX,
			payloadCreator: GetMatrixPayload,
		},
		models.WECHATWORK: {
			name:           models.WECHATWORK,
			payloadCreator: GetWechatworkPayload,
		},
	}
)

// RegisterWebhook registers a webhook
func RegisterWebhook(name string, webhook *webhook) {
	webhooks[models.HookType(name)] = webhook
}

// IsValidHookTaskType returns true if a webhook registered
func IsValidHookTaskType(name string) bool {
	if name == models.GITEA || name == models.GOGS {
		return true
	}
	_, ok := webhooks[models.HookType(name)]
	return ok
}

// hookQueue is a global queue of web hooks
var hookQueue = sync.NewUniqueQueue(setting.Webhook.QueueLength)

// getPayloadBranch returns branch for hook event, if applicable.
func getPayloadBranch(p api.Payloader) string {
	switch pp := p.(type) {
	case *api.CreatePayload:
		if pp.RefType == "branch" {
			return pp.Ref
		}
	case *api.DeletePayload:
		if pp.RefType == "branch" {
			return pp.Ref
		}
	case *api.PushPayload:
		if strings.HasPrefix(pp.Ref, git.BranchPrefix) {
			return pp.Ref[len(git.BranchPrefix):]
		}
	}
	return ""
}

// PrepareWebhook adds special webhook to task queue for given payload.
func PrepareWebhook(w *models.Webhook, repo *models.Repository, event models.HookEventType, p api.Payloader) error {
	if err := prepareWebhook(w, repo, event, p); err != nil {
		return err
	}

	go hookQueue.Add(repo.ID)
	return nil
}

func checkBranch(w *models.Webhook, branch string) bool {
	if w.BranchFilter == "" || w.BranchFilter == "*" {
		return true
	}

	g, err := glob.Compile(w.BranchFilter)
	if err != nil {
		// should not really happen as BranchFilter is validated
		log.Error("CheckBranch failed: %s", err)
		return false
	}

	return g.Match(branch)
}

func prepareWebhook(w *models.Webhook, repo *models.Repository, event models.HookEventType, p api.Payloader) error {
	// Skip sending if webhooks are disabled.
	if setting.DisableWebhooks {
		return nil
	}

	for _, e := range w.EventCheckers() {
		if event == e.Type {
			if !e.Has() {
				return nil
			}

			break
		}
	}

	// Avoid sending "0 new commits" to non-integration relevant webhooks (e.g. slack, discord, etc.).
	// Integration webhooks (e.g. drone) still receive the required data.
	if pushEvent, ok := p.(*api.PushPayload); ok &&
		w.Type != models.GITEA && w.Type != models.GOGS &&
		len(pushEvent.Commits) == 0 {
		return nil
	}

	// If payload has no associated branch (e.g. it's a new tag, issue, etc.),
	// branch filter has no effect.
	if branch := getPayloadBranch(p); branch != "" {
		if !checkBranch(w, branch) {
			log.Info("Branch %q doesn't match branch filter %q, skipping", branch, w.BranchFilter)
			return nil
		}
	}

	var payloader api.Payloader
	var err error
	webhook, ok := webhooks[w.Type]
	if ok {
		payloader, err = webhook.payloadCreator(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("create payload for %s[%s]: %v", w.Type, event, err)
		}
	} else {
		payloader = p
	}

	if err = models.CreateHookTask(&models.HookTask{
		RepoID:    repo.ID,
		HookID:    w.ID,
		Payloader: payloader,
		EventType: event,
	}); err != nil {
		return fmt.Errorf("CreateHookTask: %v", err)
	}
	return nil
}

// PrepareWebhooks adds new webhooks to task queue for given payload.
func PrepareWebhooks(repo *models.Repository, event models.HookEventType, p api.Payloader) error {
	if err := prepareWebhooks(repo, event, p); err != nil {
		return err
	}

	go hookQueue.Add(repo.ID)
	return nil
}

func prepareWebhooks(repo *models.Repository, event models.HookEventType, p api.Payloader) error {
	ws, err := models.GetActiveWebhooksByRepoID(repo.ID)
	if err != nil {
		return fmt.Errorf("GetActiveWebhooksByRepoID: %v", err)
	}

	// check if repo belongs to org and append additional webhooks
	if repo.MustOwner().IsOrganization() {
		// get hooks for org
		orgHooks, err := models.GetActiveWebhooksByOrgID(repo.OwnerID)
		if err != nil {
			return fmt.Errorf("GetActiveWebhooksByOrgID: %v", err)
		}
		ws = append(ws, orgHooks...)
	}

	// Add any admin-defined system webhooks
	systemHooks, err := models.GetSystemWebhooks()
	if err != nil {
		return fmt.Errorf("GetSystemWebhooks: %v", err)
	}
	ws = append(ws, systemHooks...)

	if len(ws) == 0 {
		return nil
	}

	for _, w := range ws {
		if err = prepareWebhook(w, repo, event, p); err != nil {
			return err
		}
	}
	return nil
}
