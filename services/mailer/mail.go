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
	"strings"
	texttmpl "text/template"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/emoji"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

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

// SendUserMail sends a mail to the user
func SendUserMail(language string, u *models.User, tpl base.TplName, code, subject, info string) {
	data := map[string]interface{}{
		"DisplayName":       u.DisplayName(),
		"ActiveCodeLives":   timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, language),
		"ResetPwdCodeLives": timeutil.MinutesToFriendly(setting.Service.ResetPwdCodeLives, language),
		"Code":              code,
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

// Locale represents an interface to translation
type Locale interface {
	Language() string
	Tr(string, ...interface{}) string
}

// SendActivateAccountMail sends an activation mail to the user (new user registration)
func SendActivateAccountMail(locale Locale, u *models.User) {
	SendUserMail(locale.Language(), u, mailAuthActivate, u.GenerateActivateCode(), locale.Tr("mail.activate_account"), "activate account")
}

// SendResetPasswordMail sends a password reset mail to the user
func SendResetPasswordMail(locale Locale, u *models.User) {
	SendUserMail(locale.Language(), u, mailAuthResetPassword, u.GenerateActivateCode(), locale.Tr("mail.reset_password"), "recover account")
}

// SendActivateEmailMail sends confirmation email to confirm new email address
func SendActivateEmailMail(locale Locale, u *models.User, email *models.EmailAddress) {
	data := map[string]interface{}{
		"DisplayName":     u.DisplayName(),
		"ActiveCodeLives": timeutil.MinutesToFriendly(setting.Service.ActiveCodeLives, locale.Language()),
		"Code":            u.GenerateEmailActivateCode(email.Email),
		"Email":           email.Email,
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
func SendRegisterNotifyMail(locale Locale, u *models.User) {
	if setting.MailService == nil {
		log.Warn("SendRegisterNotifyMail is being invoked but mail service hasn't been initialized")
		return
	}

	data := map[string]interface{}{
		"DisplayName": u.DisplayName(),
		"Username":    u.Name,
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
	repoName := repo.FullName()
	subject := fmt.Sprintf("%s added you to %s", doer.DisplayName(), repoName)

	data := map[string]interface{}{
		"Subject":  subject,
		"RepoName": repoName,
		"Link":     repo.HTMLURL(),
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

func composeIssueCommentMessages(ctx *mailCommentContext, tos []string, fromMention bool, info string) []*Message {
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
	body := string(markup.RenderByType(markdown.MarkupName, []byte(ctx.Content), ctx.Issue.Repo.HTMLURL(), ctx.Issue.Repo.ComposeMetas()))

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
	}

	var mailSubject bytes.Buffer
	if err := subjectTemplates.ExecuteTemplate(&mailSubject, string(tplName), mailMeta); err == nil {
		subject = sanitizeSubject(mailSubject.String())
	} else {
		log.Error("ExecuteTemplate [%s]: %v", string(tplName)+"/subject", err)
	}

	if subject == "" {
		subject = fallback
	}

	subject = emoji.ReplaceAliases(subject)

	mailMeta["Subject"] = subject

	var mailBody bytes.Buffer

	if err := bodyTemplates.ExecuteTemplate(&mailBody, string(tplName), mailMeta); err != nil {
		log.Error("ExecuteTemplate [%s]: %v", string(tplName)+"/body", err)
	}

	// Make sure to compose independent messages to avoid leaking user emails
	msgs := make([]*Message, 0, len(tos))
	for _, to := range tos {
		msg := NewMessageFrom([]string{to}, ctx.Doer.DisplayName(), setting.MailService.FromEmail, subject, mailBody.String())
		msg.Info = fmt.Sprintf("Subject: %s, %s", subject, info)

		// Set Message-ID on first message so replies know what to reference
		if actName == "new" {
			msg.SetHeader("Message-ID", "<"+ctx.Issue.ReplyReference()+">")
		} else {
			msg.SetHeader("In-Reply-To", "<"+ctx.Issue.ReplyReference()+">")
			msg.SetHeader("References", "<"+ctx.Issue.ReplyReference()+">")
		}
		msgs = append(msgs, msg)
	}

	return msgs
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
func SendIssueAssignedMail(issue *models.Issue, doer *models.User, content string, comment *models.Comment, tos []string) {
	SendAsyncs(composeIssueCommentMessages(&mailCommentContext{
		Issue:      issue,
		Doer:       doer,
		ActionType: models.ActionType(0),
		Content:    content,
		Comment:    comment,
	}, tos, false, "issue assigned"))
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
