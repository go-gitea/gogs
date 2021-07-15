// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"bytes"
	"fmt"
	"html/template"
	"mime"
	"regexp"
	"strconv"
	"strings"
	texttmpl "text/template"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/translation"

	"gopkg.in/gomail.v2"
)

const (
	mailAuthActivate       base.TplName = "auth/activate"
	mailAuthActivateEmail  base.TplName = "auth/activate_email"
	mailAuthResetPassword  base.TplName = "auth/reset_passwd"
	mailAuthRegisterNotify base.TplName = "auth/register_notify"

	mailNotifyCollaborator base.TplName = "notify/collaborator"

	mailRepoTransferNotify base.TplName = "notify/repo_transfer"

	// There's no actual limit for subject in RFC 5322
	mailMaxSubjectRunes = 256
)

var (
	bodyTemplates       *template.Template
	subjectTemplates    *texttmpl.Template
	subjectRemoveSpaces = regexp.MustCompile(`[\s]+`)
)

// InitMailRender initializes the mail renderer
func InitMailRender(subjectTpl *texttmpl.Template, bodyTpl *template.Template) {
	subjectTemplates = subjectTpl
	bodyTemplates = bodyTpl
}

// SendTestMail sends a test mail
func SendTestMail(email string) error {
	return gomail.Send(Sender, NewMessage([]string{email}, "Gitea Test Email!", "Gitea Test Email!").ToMessage())
}

// sendUserMail sends a mail to the user
func sendUserMail(language string, u *models.User, tpl base.TplName, code, subject, info string) {
	locale := translation.NewLocale(language)
	data := map[string]interface{}{
		"DisplayName":       u.DisplayName(),
		"ActiveCodeLives":   timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, language),
		"ResetPwdCodeLives": timeutil.MinutesToFriendly(setting.Service.ResetPwdCodeLives, language),
		"Code":              code,
		"Language":          locale.Language(),
		// helper
		"i18n":     locale,
		"Str2html": templates.Str2html,
		"TrN":      templates.TrN,
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(tpl), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage([]string{u.Email}, subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, %s", u.ID, info)

	SendAsync(msg)
}

// SendActivateAccountMail sends an activation mail to the user (new user registration)
func SendActivateAccountMail(locale translation.Locale, u *models.User) {
	sendUserMail(locale.Language(), u, mailAuthActivate, u.GenerateEmailActivateCode(u.Email), locale.Tr("mail.activate_account"), "activate account")
}

// SendResetPasswordMail sends a password reset mail to the user
func SendResetPasswordMail(u *models.User) {
	locale := translation.NewLocale(u.Language)
	sendUserMail(u.Language, u, mailAuthResetPassword, u.GenerateEmailActivateCode(u.Email), locale.Tr("mail.reset_password"), "recover account")
}

// SendActivateEmailMail sends confirmation email to confirm new email address
func SendActivateEmailMail(u *models.User, email *models.EmailAddress) {
	locale := translation.NewLocale(u.Language)
	data := map[string]interface{}{
		"DisplayName":     u.DisplayName(),
		"ActiveCodeLives": timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, locale.Language()),
		"Code":            u.GenerateEmailActivateCode(email.Email),
		"Email":           email.Email,
		"Language":        locale.Language(),
		// helper
		"i18n":     locale,
		"Str2html": templates.Str2html,
		"TrN":      templates.TrN,
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailAuthActivateEmail), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage([]string{email.Email}, locale.Tr("mail.activate_email"), content.String())
	msg.Info = fmt.Sprintf("UID: %d, activate email", u.ID)

	SendAsync(msg)
}

// SendRegisterNotifyMail triggers a notify e-mail by admin created a account.
func SendRegisterNotifyMail(u *models.User) {
	locale := translation.NewLocale(u.Language)

	data := map[string]interface{}{
		"DisplayName": u.DisplayName(),
		"Username":    u.Name,
		"Language":    locale.Language(),
		// helper
		"i18n":     locale,
		"Str2html": templates.Str2html,
		"TrN":      templates.TrN,
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailAuthRegisterNotify), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage([]string{u.Email}, locale.Tr("mail.register_notify"), content.String())
	msg.Info = fmt.Sprintf("UID: %d, registration notify", u.ID)

	SendAsync(msg)
}

// SendCollaboratorMail sends mail notification to new collaborator.
func SendCollaboratorMail(u, doer *models.User, repo *models.Repository) {
	locale := translation.NewLocale(u.Language)
	repoName := repo.FullName()

	subject := locale.Tr("mail.repo.collaborator.added.subject", doer.DisplayName(), repoName)
	data := map[string]interface{}{
		"Subject":  subject,
		"RepoName": repoName,
		"Link":     repo.HTMLURL(),
		"Language": locale.Language(),
		// helper
		"i18n":     locale,
		"Str2html": templates.Str2html,
		"TrN":      templates.TrN,
	}

	var content bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&content, string(mailNotifyCollaborator), data); err != nil {
		log.Error("Template: %v", err)
		return
	}

	msg := NewMessage([]string{u.Email}, subject, content.String())
	msg.Info = fmt.Sprintf("UID: %d, add collaborator", u.ID)

	SendAsync(msg)
}

func composeIssueCommentMessages(ctx *mailCommentContext, lang string, recipients []*models.User, fromMention bool, info string) ([]*Message, error) {
	var (
		subject string
		link    string
		prefix  string
		// Fall back subject for bad templates, make sure subject is never empty
		fallback       string
		reviewComments []*models.Comment
	)

	commentType := models.CommentTypeComment
	if ctx.Comment != nil {
		commentType = ctx.Comment.Type
		link = ctx.Issue.HTMLURL() + "#" + ctx.Comment.HashTag()
	} else {
		link = ctx.Issue.HTMLURL()
	}

	reviewType := models.ReviewTypeComment
	if ctx.Comment != nil && ctx.Comment.Review != nil {
		reviewType = ctx.Comment.Review.Type
	}

	// This is the body of the new issue or comment, not the mail body
	body, err := markdown.RenderString(&markup.RenderContext{
		URLPrefix: ctx.Issue.Repo.HTMLURL(),
		Metas:     ctx.Issue.Repo.ComposeMetas(),
	}, ctx.Content)
	if err != nil {
		return nil, err
	}

	actType, actName, tplName := actionToTemplate(ctx.Issue, ctx.ActionType, commentType, reviewType)

	if actName != "new" {
		prefix = "Re: "
	}
	fallback = prefix + fallbackMailSubject(ctx.Issue)

	if ctx.Comment != nil && ctx.Comment.Review != nil {
		reviewComments = make([]*models.Comment, 0, 10)
		for _, lines := range ctx.Comment.Review.CodeComments {
			for _, comments := range lines {
				reviewComments = append(reviewComments, comments...)
			}
		}
	}
	locale := translation.NewLocale(lang)

	mailMeta := map[string]interface{}{
		"FallbackSubject": fallback,
		"Body":            body,
		"Link":            link,
		"Issue":           ctx.Issue,
		"Comment":         ctx.Comment,
		"IsPull":          ctx.Issue.IsPull,
		"User":            ctx.Issue.Repo.MustOwner(),
		"Repo":            ctx.Issue.Repo.FullName(),
		"Doer":            ctx.Doer,
		"IsMention":       fromMention,
		"SubjectPrefix":   prefix,
		"ActionType":      actType,
		"ActionName":      actName,
		"ReviewComments":  reviewComments,
		"Language":        locale.Language(),
		// helper
		"i18n":     locale,
		"Str2html": templates.Str2html,
		"TrN":      templates.TrN,
	}

	var mailSubject bytes.Buffer
	if err := subjectTemplates.ExecuteTemplate(&mailSubject, string(tplName), mailMeta); err == nil {
		subject = sanitizeSubject(mailSubject.String())
		if subject == "" {
			subject = fallback
		}
	} else {
		log.Error("ExecuteTemplate [%s]: %v", tplName+"/subject", err)
	}

	subject = emoji.ReplaceAliases(subject)

	mailMeta["Subject"] = subject

	var mailBody bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&mailBody, string(tplName), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", string(tplName)+"/body", err)
	}

	// Make sure to compose independent messages to avoid leaking user emails
	msgs := make([]*Message, 0, len(recipients))
	for _, recipient := range recipients {
		msg := NewMessageFrom([]string{recipient.Email}, ctx.Doer.DisplayName(), setting.MailService.FromEmail, subject, mailBody.String())
		msg.Info = fmt.Sprintf("Subject: %s, %s", subject, info)

		// Set Message-ID on first message so replies know what to reference
		if actName == "new" {
			msg.SetHeader("Message-ID", "<"+ctx.Issue.ReplyReference()+">")
		} else {
			msg.SetHeader("In-Reply-To", "<"+ctx.Issue.ReplyReference()+">")
			msg.SetHeader("References", "<"+ctx.Issue.ReplyReference()+">")
		}

		for key, value := range generateAdditionalHeaders(ctx, actType, recipient) {
			msg.SetHeader(key, value)
		}

		msgs = append(msgs, msg)
	}

	return msgs, nil
}

func generateAdditionalHeaders(ctx *mailCommentContext, reason string, recipient *models.User) map[string]string {
	repo := ctx.Issue.Repo

	return map[string]string{
		// https://datatracker.ietf.org/doc/html/rfc2919
		"List-ID": fmt.Sprintf("%s <%s.%s.%s>", repo.FullName(), repo.Name, repo.OwnerName, setting.Domain),

		// https://datatracker.ietf.org/doc/html/rfc2369
		"List-Archive": fmt.Sprintf("<%s>", repo.HTMLURL()),
		//"List-Post": https://github.com/go-gitea/gitea/pull/13585
		//"List-Unsubscribe": https://github.com/go-gitea/gitea/issues/10808, https://github.com/go-gitea/gitea/issues/13283

		"X-Gitea-Reason":            reason,
		"X-Gitea-Sender":            ctx.Doer.DisplayName(),
		"X-Gitea-Recipient":         recipient.DisplayName(),
		"X-Gitea-Recipient-Address": recipient.Email,
		"X-Gitea-Repository":        repo.Name,
		"X-Gitea-Repository-Path":   repo.FullName(),
		"X-Gitea-Repository-Link":   repo.HTMLURL(),
		"X-Gitea-Issue-ID":          strconv.FormatInt(ctx.Issue.Index, 10),
		"X-Gitea-Issue-Link":        ctx.Issue.HTMLURL(),

		"X-GitHub-Reason":            reason,
		"X-GitHub-Sender":            ctx.Doer.DisplayName(),
		"X-GitHub-Recipient":         recipient.DisplayName(),
		"X-GitHub-Recipient-Address": recipient.Email,

		"X-GitLab-NotificationReason": reason,
		"X-GitLab-Project":            repo.Name,
		"X-GitLab-Project-Path":       repo.FullName(),
		"X-GitLab-Issue-IID":          strconv.FormatInt(ctx.Issue.Index, 10),
	}
}

func sanitizeSubject(subject string) string {
	runes := []rune(strings.TrimSpace(subjectRemoveSpaces.ReplaceAllLiteralString(subject, " ")))
	if len(runes) > mailMaxSubjectRunes {
		runes = runes[:mailMaxSubjectRunes]
	}
	// Encode non-ASCII characters
	return mime.QEncoding.Encode("utf-8", string(runes))
}

// SendIssueAssignedMail composes and sends issue assigned email
func SendIssueAssignedMail(issue *models.Issue, doer *models.User, content string, comment *models.Comment, recipients []*models.User) error {
	langMap := make(map[string][]*models.User)
	for _, user := range recipients {
		langMap[user.Language] = append(langMap[user.Language], user)
	}

	for lang, tos := range langMap {
		msgs, err := composeIssueCommentMessages(&mailCommentContext{
			Issue:      issue,
			Doer:       doer,
			ActionType: models.ActionType(0),
			Content:    content,
			Comment:    comment,
		}, lang, tos, false, "issue assigned")
		if err != nil {
			return err
		}
		SendAsyncs(msgs)
	}
	return nil
}

// actionToTemplate returns the type and name of the action facing the user
// (slightly different from models.ActionType) and the name of the template to use (based on availability)
func actionToTemplate(issue *models.Issue, actionType models.ActionType,
	commentType models.CommentType, reviewType models.ReviewType) (typeName, name, template string) {
	if issue.IsPull {
		typeName = "pull"
	} else {
		typeName = "issue"
	}
	switch actionType {
	case models.ActionCreateIssue, models.ActionCreatePullRequest:
		name = "new"
	case models.ActionCommentIssue, models.ActionCommentPull:
		name = "comment"
	case models.ActionCloseIssue, models.ActionClosePullRequest:
		name = "close"
	case models.ActionReopenIssue, models.ActionReopenPullRequest:
		name = "reopen"
	case models.ActionMergePullRequest:
		name = "merge"
	case models.ActionPullReviewDismissed:
		name = "review_dismissed"
	case models.ActionPullRequestReadyForReview:
		name = "ready_for_review"
	default:
		switch commentType {
		case models.CommentTypeReview:
			switch reviewType {
			case models.ReviewTypeApprove:
				name = "approve"
			case models.ReviewTypeReject:
				name = "reject"
			default:
				name = "review"
			}
		case models.CommentTypeCode:
			name = "code"
		case models.CommentTypeAssignees:
			name = "assigned"
		case models.CommentTypePullPush:
			name = "push"
		default:
			name = "default"
		}
	}

	template = typeName + "/" + name
	ok := bodyTemplates.Lookup(template) != nil
	if !ok && typeName != "issue" {
		template = "issue/" + name
		ok = bodyTemplates.Lookup(template) != nil
	}
	if !ok {
		template = typeName + "/default"
		ok = bodyTemplates.Lookup(template) != nil
	}
	if !ok {
		template = "issue/default"
	}
	return
}
