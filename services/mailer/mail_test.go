// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package mailer

import (
	"bytes"
	"html/template"
	"testing"
	texttmpl "text/template"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

const subjectTpl = `
{{.SubjectPrefix}}[{{.Repo}}] @{{.Doer.Name}} #{{.Issue.Index}} - {{.Issue.Title}}
`

const bodyTpl = `
<!DOCTYPE html>
<html>
<head>
	<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
	<title>{{.Subject}}</title>
</head>

<body>
	<p>{{.Body}}</p>
	<p>
		---
		<br>
		<a href="{{.Link}}">View it on Gitea</a>.
	</p>
</body>
</html>
`

func TestComposeIssueCommentMessage(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	mailService := setting.Mailer{
		From: "test@gitea.com",
	}

	setting.MailService = &mailService
	setting.Domain = "localhost"

	doer := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1, Owner: doer}).(*models.Repository)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 1, Repo: repo, Poster: doer}).(*models.Issue)
	comment := models.AssertExistsAndLoadBean(t, &models.Comment{ID: 2, Issue: issue}).(*models.Comment)

	stpl := texttmpl.Must(texttmpl.New("issue/comment").Parse(subjectTpl))
	btpl := template.Must(template.New("issue/comment").Parse(bodyTpl))
	InitMailRender(stpl, btpl)

	tos := []string{"test@gitea.com", "test2@gitea.com"}
	msgs := composeIssueCommentMessages(&mailCommentContext{
		Issue: issue, Doer: doer, ActionType: models.ActionCommentIssue,
		Content: "test body", Comment: comment,
	}, tos, false, "issue comment")
	assert.Len(t, msgs, 2)
	gomailMsg := msgs[0].ToMessage()
	mailto := gomailMsg.GetHeader("To")
	subject := gomailMsg.GetHeader("Subject")
	inreplyTo := gomailMsg.GetHeader("In-Reply-To")
	references := gomailMsg.GetHeader("References")

	assert.Len(t, mailto, 1, "exactly one recipient is expected in the To field")
	assert.Equal(t, "Re: ", subject[0][:4], "Comment reply subject should contain Re:")
	assert.Equal(t, "Re: [user2/repo1] @user2 #1 - issue1", subject[0])
	assert.Equal(t, inreplyTo[0], "<user2/repo1/issues/1@localhost>", "In-Reply-To header doesn't match")
	assert.Equal(t, references[0], "<user2/repo1/issues/1@localhost>", "References header doesn't match")
}

func TestComposeIssueMessage(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	mailService := setting.Mailer{
		From: "test@gitea.com",
	}

	setting.MailService = &mailService
	setting.Domain = "localhost"

	doer := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1, Owner: doer}).(*models.Repository)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 1, Repo: repo, Poster: doer}).(*models.Issue)

	stpl := texttmpl.Must(texttmpl.New("issue/new").Parse(subjectTpl))
	btpl := template.Must(template.New("issue/new").Parse(bodyTpl))
	InitMailRender(stpl, btpl)

	tos := []string{"test@gitea.com", "test2@gitea.com"}
	msgs := composeIssueCommentMessages(&mailCommentContext{
		Issue: issue, Doer: doer, ActionType: models.ActionCreateIssue,
		Content: "test body",
	}, tos, false, "issue create")
	assert.Len(t, msgs, 2)

	gomailMsg := msgs[0].ToMessage()
	mailto := gomailMsg.GetHeader("To")
	subject := gomailMsg.GetHeader("Subject")
	messageID := gomailMsg.GetHeader("Message-ID")

	assert.Len(t, mailto, 1, "exactly one recipient is expected in the To field")
	assert.Equal(t, "[user2/repo1] @user2 #1 - issue1", subject[0])
	assert.Nil(t, gomailMsg.GetHeader("In-Reply-To"))
	assert.Nil(t, gomailMsg.GetHeader("References"))
	assert.Equal(t, messageID[0], "<user2/repo1/issues/1@localhost>", "Message-ID header doesn't match")
}

func TestTemplateSelection(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	mailService := setting.Mailer{
		From: "test@gitea.com",
	}

	setting.MailService = &mailService
	setting.Domain = "localhost"

	doer := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1, Owner: doer}).(*models.Repository)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 1, Repo: repo, Poster: doer}).(*models.Issue)
	tos := []string{"test@gitea.com"}

	stpl := texttmpl.Must(texttmpl.New("issue/default").Parse("issue/default/subject"))
	texttmpl.Must(stpl.New("issue/new").Parse("issue/new/subject"))
	texttmpl.Must(stpl.New("pull/comment").Parse("pull/comment/subject"))
	texttmpl.Must(stpl.New("issue/close").Parse("")) // Must default to fallback subject

	btpl := template.Must(template.New("issue/default").Parse("issue/default/body"))
	template.Must(btpl.New("issue/new").Parse("issue/new/body"))
	template.Must(btpl.New("pull/comment").Parse("pull/comment/body"))
	template.Must(btpl.New("issue/close").Parse("issue/close/body"))

	InitMailRender(stpl, btpl)

	expect := func(t *testing.T, msg *Message, expSubject, expBody string) {
		subject := msg.ToMessage().GetHeader("Subject")
		msgbuf := new(bytes.Buffer)
		_, _ = msg.ToMessage().WriteTo(msgbuf)
		wholemsg := msgbuf.String()
		assert.Equal(t, []string{expSubject}, subject)
		assert.Contains(t, wholemsg, expBody)
	}

	msg := testComposeIssueCommentMessage(t, &mailCommentContext{
		Issue: issue, Doer: doer, ActionType: models.ActionCreateIssue,
		Content: "test body",
	}, tos, false, "TestTemplateSelection")
	expect(t, msg, "issue/new/subject", "issue/new/body")

	comment := models.AssertExistsAndLoadBean(t, &models.Comment{ID: 2, Issue: issue}).(*models.Comment)
	msg = testComposeIssueCommentMessage(t, &mailCommentContext{
		Issue: issue, Doer: doer, ActionType: models.ActionCommentIssue,
		Content: "test body", Comment: comment,
	}, tos, false, "TestTemplateSelection")
	expect(t, msg, "issue/default/subject", "issue/default/body")

	pull := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 2, Repo: repo, Poster: doer}).(*models.Issue)
	comment = models.AssertExistsAndLoadBean(t, &models.Comment{ID: 4, Issue: pull}).(*models.Comment)
	msg = testComposeIssueCommentMessage(t, &mailCommentContext{
		Issue: pull, Doer: doer, ActionType: models.ActionCommentPull,
		Content: "test body", Comment: comment,
	}, tos, false, "TestTemplateSelection")
	expect(t, msg, "pull/comment/subject", "pull/comment/body")

	msg = testComposeIssueCommentMessage(t, &mailCommentContext{
		Issue: issue, Doer: doer, ActionType: models.ActionCloseIssue,
		Content: "test body", Comment: comment,
	}, tos, false, "TestTemplateSelection")
	expect(t, msg, "Re: [user2/repo1] issue1 (#1)", "issue/close/body")
}

func TestTemplateServices(t *testing.T) {
	assert.NoError(t, models.PrepareTestDatabase())
	mailService := setting.Mailer{
		From: "test@gitea.com",
	}

	setting.MailService = &mailService
	setting.Domain = "localhost"

	doer := models.AssertExistsAndLoadBean(t, &models.User{ID: 2}).(*models.User)
	repo := models.AssertExistsAndLoadBean(t, &models.Repository{ID: 1, Owner: doer}).(*models.Repository)
	issue := models.AssertExistsAndLoadBean(t, &models.Issue{ID: 1, Repo: repo, Poster: doer}).(*models.Issue)
	comment := models.AssertExistsAndLoadBean(t, &models.Comment{ID: 2, Issue: issue}).(*models.Comment)
	assert.NoError(t, issue.LoadRepo())

	expect := func(t *testing.T, issue *models.Issue, comment *models.Comment, doer *models.User,
		actionType models.ActionType, fromMention bool, tplSubject, tplBody, expSubject, expBody string) {
		stpl := texttmpl.Must(texttmpl.New("issue/default").Parse(tplSubject))
		btpl := template.Must(template.New("issue/default").Parse(tplBody))
		InitMailRender(stpl, btpl)

		tos := []string{"test@gitea.com"}
		msg := testComposeIssueCommentMessage(t, &mailCommentContext{
			Issue: issue, Doer: doer, ActionType: actionType,
			Content: "test body", Comment: comment,
		}, tos, fromMention, "TestTemplateServices")

		subject := msg.ToMessage().GetHeader("Subject")
		msgbuf := new(bytes.Buffer)
		_, _ = msg.ToMessage().WriteTo(msgbuf)
		wholemsg := msgbuf.String()

		assert.Equal(t, []string{expSubject}, subject)
		assert.Contains(t, wholemsg, "\r\n"+expBody+"\r\n")
	}

	expect(t, issue, comment, doer, models.ActionCommentIssue, false,
		"{{.SubjectPrefix}}[{{.Repo}}]: @{{.Doer.Name}} commented on #{{.Issue.Index}} - {{.Issue.Title}}",
		"//{{.ActionType}},{{.ActionName}},{{if .IsMention}}norender{{end}}//",
		"Re: [user2/repo1]: @user2 commented on #1 - issue1",
		"//issue,comment,//")

	expect(t, issue, comment, doer, models.ActionCommentIssue, true,
		"{{if .IsMention}}must render{{end}}",
		"//subject is: {{.Subject}}//",
		"must render",
		"//subject is: must render//")

	expect(t, issue, comment, doer, models.ActionCommentIssue, true,
		"{{.FallbackSubject}}",
		"//{{.SubjectPrefix}}//",
		"Re: [user2/repo1] issue1 (#1)",
		"//Re: //")
}

func testComposeIssueCommentMessage(t *testing.T, ctx *mailCommentContext, tos []string, fromMention bool, info string) *Message {
	msgs := composeIssueCommentMessages(ctx, tos, fromMention, info)
	assert.Len(t, msgs, 1)
	return msgs[0]
}
