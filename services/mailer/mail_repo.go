// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"bytes"
	"fmt"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/translation"
)

// SendRepoTransferNotifyMail triggers a notification e-mail when a pending repository transfer was created
func SendRepoTransferNotifyMail(doer, newOwner *models.User, repo *models.Repository) error {
	if newOwner.IsOrganization() {
		users, err := models.GetUsersWhoCanCreateOrgRepo(newOwner.ID)
		if err != nil {
			return err
		}

		langMap := make(map[string][]string)
		for _, user := range users {
			langMap[user.Language] = append(langMap[user.Language], user.Email)
		}

		for lang, tos := range langMap {
			if err := sendRepoTransferNotifyMailPerLang(lang, newOwner, doer, tos, repo); err != nil {
				return err
			}
		}

		return nil
	}

	return sendRepoTransferNotifyMailPerLang(newOwner.Language, newOwner, doer, []string{newOwner.Email}, repo)
}

// sendRepoTransferNotifyMail triggers a notification e-mail when a pending repository transfer was created for each language
func sendRepoTransferNotifyMailPerLang(lang string, newOwner, doer *models.User, emails []string, repo *models.Repository) error {
	var (
		locale  = translation.NewLocale(lang)
		content bytes.Buffer
	)

	destination := locale.Tr("mail.repo.transfer.to_you")
	subject := locale.Tr("mail.repo.transfer.subject_to_you", doer.DisplayName(), repo.FullName())
	if newOwner.IsOrganization() {
		destination = newOwner.DisplayName()
		subject = locale.Tr("mail.repo.transfer.subject_to", doer.DisplayName(), repo.FullName(), destination)
	}

	data := map[string]interface{}{
		"Doer":        doer,
		"User":        repo.Owner,
		"Repo":        repo.FullName(),
		"Link":        repo.HTMLURL(),
		"Subject":     subject,
		"Language":    locale.Language(),
		"Destination": destination,
		// helper
		"i18n":     locale,
		"Str2html": templates.Str2html,
	}

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailRepoTransferNotify), data); err != nil {
		return err
	}

	msg := NewMessage(emails, subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, repository pending transfer notification", newOwner.ID)

	SendAsync(msg)
	return nil
}
