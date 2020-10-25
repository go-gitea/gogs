// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	for _, e := range w.EventCheckers() {
		if event == e.Type {
			if !e.Has() {
				return nil
			}
		}
	}

	// Avoid sending "0 new commits" to non-integration relevant webhooks (e.g. slack, discord, etc.).
	// Integration webhooks (e.g. drone) still receive the required data.
	if pushEvent, ok := p.(*api.PushPayload); ok &&
		w.HookTaskType != models.GITEA && w.HookTaskType != models.GOGS &&
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
	// Use separate objects so modifications won't be made on payload on non-Gogs/Gitea type hooks.
	switch w.HookTaskType {
	case models.SLACK:
		payloader, err = GetSlackPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetSlackPayload: %v", err)
		}
	case models.DISCORD:
		payloader, err = GetDiscordPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetDiscordPayload: %v", err)
		}
	case models.DINGTALK:
		payloader, err = GetDingtalkPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetDingtalkPayload: %v", err)
		}
	case models.TELEGRAM:
		payloader, err = GetTelegramPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetTelegramPayload: %v", err)
		}
	case models.MSTEAMS:
		payloader, err = GetMSTeamsPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetMSTeamsPayload: %v", err)
		}
	case models.FEISHU:
		payloader, err = GetFeishuPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetFeishuPayload: %v", err)
		}
	case models.MATRIX:
		payloader, err = GetMatrixPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetMatrixPayload: %v", err)
		}
	case models.WORKWECHAT:
		payloader, err = GetWorkwechatPayload(p, event, w.Meta)
		if err != nil {
			return fmt.Errorf("GetWorkwechatPayload: %v", err)
		}
	default:
		p.SetSecret(w.Secret)
		payloader = p
	}

	var signature string
	if len(w.Secret) > 0 {
		data, err := payloader.JSONPayload()
		if err != nil {
			log.Error("prepareWebhooks.JSONPayload: %v", err)
		}
		sig := hmac.New(sha256.New, []byte(w.Secret))
		_, err = sig.Write(data)
		if err != nil {
			log.Error("prepareWebhooks.sigWrite: %v", err)
		}
		signature = hex.EncodeToString(sig.Sum(nil))
	}

	if err = models.CreateHookTask(&models.HookTask{
		RepoID:      repo.ID,
		HookID:      w.ID,
		Type:        w.HookTaskType,
		URL:         w.URL,
		Signature:   signature,
		Payloader:   payloader,
		HTTPMethod:  w.HTTPMethod,
		ContentType: w.ContentType,
		EventType:   event,
		IsSSL:       w.IsSSL,
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
