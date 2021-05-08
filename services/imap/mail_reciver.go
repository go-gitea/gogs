// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package imap

import (
	"fmt"
	"strconv"
	"strings"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/queue"
	"code.gitea.io/gitea/modules/setting"
	comment_service "code.gitea.io/gitea/services/comments"
)

// mail read query
var mailReadQueue queue.Queue

// NewContext start received mail read queue service
func NewContext() {
	if setting.MailReciveService == nil || mailReadQueue != nil {
		return
	}

	mailReadQueue = queue.CreateQueue("mail_recive", func(data ...queue.Data) {
		for _, datum := range data {
			mail := datum.(*Mail)
			if err := mail.LoadHeader([]string{"From", "To", "In-Reply-To", "References"}); err != nil {
				log.Error("fetch mail header failed: %v", err)
				continue
			}

			if len(mail.Heads["To"]) == 0 ||
				len(mail.Heads["From"]) == 0 {
				continue
			}

			if mail.Heads["To"][0].Address != setting.MailReciveService.ReciveEmail {
				continue
			}

			log.Trace("start read email from %v", mail.Heads["From"][0].String())
			if err := handleReciveEmail(mail); err != nil {
				log.Error("handleReciveEmail(): %v", err)
				continue
			}
			log.Trace("finished read email from %v", mail.Heads["From"][0].String())
		}
	}, &Mail{})

	go graceful.GetManager().RunWithShutdownFns(mailReadQueue.Run)
}

func handleReciveEmail(m *Mail) error {
	fromEmail, ok := m.Heads["From"]
	if !ok || len(fromEmail) < 1 {
		return nil
	}
	from := fromEmail[0].Address
	doer, err := models.GetUserByEmail(from)
	if err != nil {
		if models.IsErrUserNotExist(err) {
			return nil
		}
		return fmt.Errorf("models.GetUserByEmail(%v): %v", from, err)
	}

	checkLink := ""

	// check `In-Reply-To`
	if links, ok := m.Heads["In-Reply-To"]; ok && links != nil {
		for _, link := range links {
			if strings.Contains(link.Address, setting.Domain) {
				checkLink = link.Address
				break
			}
		}
	}

	if len(checkLink) == 0 {
		// check `References`
		if links, ok := m.Heads["References"]; ok && links != nil {
			for _, link := range links {
				if strings.Contains(link.Address, setting.Domain) {
					checkLink = link.Address
					break
				}
			}
		}
	}

	if len(checkLink) == 0 {
		_ = m.SetRead(true)
		return nil
	}

	splitLink := strings.SplitN(checkLink, "@", 2)
	if len(splitLink) != 2 || splitLink[1] != setting.Domain {
		_ = m.SetRead(true)
		return nil
	}

	splitLink = strings.SplitN(splitLink[0], "?", 2)
	if len(splitLink) != 2 {
		_ = m.SetRead(true)
		return nil
	}

	checkKey := splitLink[1]

	splitLink = strings.SplitN(splitLink[0], "#", 2)
	if len(splitLink) == 0 {
		_ = m.SetRead(true)
		return nil
	}

	splitLink = strings.SplitN(splitLink[0], "/", 4)
	if len(splitLink) != 4 {
		_ = m.SetRead(true)
		return nil
	}

	if len(splitLink) != 4 ||
		(splitLink[2] != "pulls" && splitLink[2] != "issues") {
		_ = m.SetRead(true)
		return nil
	}

	repoOwner := splitLink[0]
	repoName := splitLink[1]
	issueIndex, err := strconv.ParseInt(splitLink[3], 0, 64)
	if err != nil {
		_ = m.SetRead(true)
		return nil
	}
	if issueIndex <= 0 {
		_ = m.SetRead(true)
		return nil
	}

	repo, err := models.GetRepositoryByOwnerAndName(repoOwner, repoName)
	if err != nil {
		if models.IsErrRepoNotExist(err) {
			_ = m.SetRead(true)
			return nil
		}

		return fmt.Errorf("models.GetRepositoryByOwnerAndName(%v,%v): %v", repoOwner, repoName, err)
	}

	if repo.IsArchived {
		_ = m.SetRead(true)
		return nil
	}

	perm, err := models.GetUserRepoPermission(repo, doer)
	if err != nil {
		return fmt.Errorf("models.GetUserRepoPermission(): %v", err)
	}

	issue, err := models.GetIssueWithAttrsByIndex(repo.ID, issueIndex)
	if err != nil {
		if models.IsErrIssueNotExist(err) {
			_ = m.SetRead(true)
			return nil
		}

		return fmt.Errorf("models.GetIssueWithAttrsByIndex(%v,%v): %v", repo.ID, issueIndex, err)
	}

	// check key
	cmp := base.EncodeSha256(fmt.Sprintf("%d:%s/%s", issue.ID, from, doer.Rands))
	if cmp != checkKey {
		_ = m.SetRead(true)
		return nil
	}

	// check permission
	permUnit := models.UnitTypeIssues
	if issue.IsPull {
		permUnit = models.UnitTypePullRequests
	}

	if issue.IsLocked && !perm.CanWrite(permUnit) {
		_ = m.SetRead(true)
		return nil
	}

	if !issue.IsLocked && !perm.CanRead(permUnit) {
		_ = m.SetRead(true)
		return nil
	}

	if err := m.LoadBody(); err != nil {
		return fmt.Errorf("m.LoadBody(): %v", err)
	}

	_, err = comment_service.CreateIssueComment(doer,
		repo,
		issue,
		m.ContentText, nil)
	if err != nil {
		return fmt.Errorf("comment_service.CreateIssueComment(): %v", err)
	}

	_ = m.SetRead(true)

	if setting.MailReciveService.DeleteRodeMail {
		_ = m.Delete()
	}

	return nil
}
