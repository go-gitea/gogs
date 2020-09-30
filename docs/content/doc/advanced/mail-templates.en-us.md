---
date: "2019-10-23T17:00:00-03:00"
title: "Mail templates"
slug: "mail-templates"
weight: 45
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Mail templates"
    weight: 45
    identifier: "mail-templates"
---

# Mail templates

To craft the e-mail subject and contents for certain operations, Gitea can be customized by using templates. The templates
for these functions are located under the [`custom` directory](https://docs.gitea.io/en-us/customizing-gitea/).
Gitea has an internal template that serves as default in case there's no custom alternative.

Custom templates are loaded when Gitea starts. Changes made to them are not recognized until Gitea is restarted again.

## Mail notifications supporting templates

Currently, the following notification events make use of templates:

| Action name   | Usage                                                                                                        |
|---------------|--------------------------------------------------------------------------------------------------------------|
| `new`         | A new issue or pull request was created.                                                                     |
| `comment`     | A new comment was created in an existing issue or pull request.                                              |
| `close`       | An issue or pull request was closed.                                                                         |
| `reopen`      | An issue or pull request was reopened.                                                                       |
| `review`      | The head comment of a review in a pull request.                                                              |
| `approve`     | The head comment of a approving review for a pull request.                                                   |
| `reject`      | The head comment of a review requesting changes for a pull request.                                          |
| `code`        | A single comment on the code of a pull request.                                                              |
| `assigned`    | Used was assigned to an issue or pull request.                                                               |
| `default`     | Any action not included in the above categories, or when the corresponding category template is not present. |

The path for the template of a particular message type is:

```
custom/templates/mail/{action type}/{action name}.tmpl
```

Where `{action type}` is one of `issue` or `pull` (for pull requests), and `{action name}` is one of the names listed above.

For example, the specific template for a mail regarding a comment in a pull request is:
```
custom/templates/mail/pull/comment.tmpl
```

However, creating templates for each and every action type/name combination is not required.
A fallback system is used to choose the appropriate template for an event. The _first existing_
template on this list is used:

* The specific template for the desired **action type** and **action name**.
* The template for action type `issue` and the desired **action name**.
* The template for the desired **action type**, action name `default`.
* The template for action type `issue`, action name `default`.

The only mandatory template is action type `issue`, action name `default`, which is already embedded in Gitea
unless it's overridden by the user in the `custom` directory.

## Template syntax

Mail templates are UTF-8 encoded text files that need to follow one of the following formats:

```
Text and macros for the subject line
------------
Text and macros for the mail body
```

or

```
Text and macros for the mail body
```

Specifying a _subject_ section is optional (and therefore also the dash line separator). When used, the separator between
_subject_ and _mail body_ templates requires at least three dashes; no other characters are allowed in the separator line.


_Subject_ and _mail body_ are parsed by [Golang's template engine](https://golang.org/pkg/text/template/) and
are provided with a _metadata context_ assembled for each notification. The context contains the following elements:

| Name               | Type             | Available     | Usage                                                                                                                                                |
|--------------------|------------------|---------------|------------------------------------------------------------------------------------------------------------------------------------------------------|
| `.FallbackSubject` | string           | Always        | A default subject line. See Below.                                                                                                                   |
| `.Subject`         | string           | Only in body  | The _subject_, once resolved.                                                                                                                        |
| `.Body`            | string           | Always        | The message of the issue, pull request or comment, parsed from Markdown into HTML and sanitized. Do not confuse with the _mail body_.                |
| `.Link`            | string           | Always        | The address of the originating issue, pull request or comment.                                                                                       |
| `.Issue`           | models.Issue     | Always        | The issue (or pull request) originating the notification. To get data specific to a pull request (e.g. `HasMerged`), `.Issue.PullRequest` can be used, but care should be taken as this field will be `nil` if the issue is *not* a pull request. |
| `.Comment`         | models.Comment   | If applicable | If the notification is from a comment added to an issue or pull request, this will contain the information about the comment.                        |
| `.IsPull`          | bool             | Always        | `true` if the mail notification is associated with a pull request (i.e. `.Issue.PullRequest` is not `nil`).                                          |
| `.Repo`            | string           | Always        | Name of the repository, including owner name (e.g. `mike/stuff`)                                                                                     |
| `.User`            | models.User      | Always        | Owner of the repository from which the event originated. To get the user name (e.g. `mike`),`.User.Name` can be used.                                |
| `.Doer`            | models.User      | Always        | User that executed the action triggering the notification event. To get the user name (e.g. `rhonda`), `.Doer.Name` can be used.                     |
| `.IsMention`       | bool             | Always        | `true` if this notification was only generated because the user was mentioned in the comment, while not being subscribed to the source. It will be `false` if the recipient was subscribed to the issue or repository. |
| `.SubjectPrefix`   | string           | Always        | `Re: ` if the notification is about other than issue or pull request creation; otherwise an empty string.                                            |
| `.ActionType`      | string           | Always        | `"issue"` or `"pull"`. Will correspond to the actual _action type_ independently of which template was selected.                                     |
| `.ActionName`      | string           | Always        | It will be one of the action types described above (`new`, `comment`, etc.), and will correspond to the actual _action name_ independently of which template was selected. |
| `.ReviewComments`  | []models.Comment | Always        | List of code comments in a review. The comment text will be in `.RenderedContent` and the referenced code will be in `.Patch`.                       |

All names are case sensitive.

### The _subject_ part of the template

The template engine used for the mail _subject_ is golang's [`text/template`](https://golang.org/pkg/text/template/).
Please refer to the linked documentation for details about its syntax.

The _subject_ is built using the following steps:

* A template is selected according to the type of notification and to what templates are present.
* The template is parsed and resolved (e.g. `{{.Issue.Index}}` is converted to the number of the issue
  or pull request).
* All space-like characters (e.g. `TAB`, `LF`, etc.) are converted to normal spaces.
* All leading, trailing and redundant spaces are removed.
* The string is truncated to its first 256 runes (characters).

If the end result is an empty string, **or** no subject template was available (i.e. the selected template
did not include a subject part), Gitea's **internal default** will be used.

The internal default (fallback) subject is the equivalent of:

```
{{.SubjectPrefix}}[{{.Repo}}] {{.Issue.Title}} (#.Issue.Index)
```

For example: `Re: [mike/stuff] New color palette (#38)`

Gitea's default subject can also be found in the template _metadata_ as `.FallbackSubject` from any of
the two templates, even if a valid subject template is present.

### The _mail body_ part of the template

The template engine used for the _mail body_ is golang's [`html/template`](https://golang.org/pkg/html/template/).
Please refer to the linked documentation for details about its syntax.

The _mail body_ is parsed after the mail subject, so there is an additional _metadata_ field which is
the actual rendered subject, after all considerations.

The expected result is HTML (including structural elements like`<html>`, `<body>`, etc.). Styling
through `<style>` blocks, `class` and `style` attributes is possible. However, `html/template`
does some [automatic escaping](https://golang.org/pkg/html/template/#hdr-Contexts) that should be considered.

Attachments (such as images or external style sheets) are not supported. However, other templates can
be referenced too, for example to provide the contents of a `<style>` element in a centralized fashion.
The external template must be placed under `custom/mail` and referenced relative to that directory.
For example, `custom/mail/styles/base.tmpl` can be included using `{{template styles/base}}`.

The mail is sent with `Content-Type: multipart/alternative`, so the body is sent in both HTML
and text formats. The latter is obtained by stripping the HTML markup.

## Troubleshooting

How a mail is rendered is directly dependent on the capabilities of the mail application. Many mail
clients don't even support HTML, so they show the text version included in the generated mail.

If the template fails to render, it will be noticed only at the moment the mail is sent.
A default subject is used if the subject template fails, and whatever was rendered successfully
from the the _mail body_ is used, disregarding the rest.

Please check [Gitea's logs](https://docs.gitea.io/en-us/logging-configuration/) for error messages in case of trouble.

## Example

`custom/templates/mail/issue/default.tmpl`:

```
[{{.Repo}}] @{{.Doer.Name}}
{{if eq .ActionName "new"}}
    created
{{else if eq .ActionName "comment"}}
    commented on
{{else if eq .ActionName "close"}}
    closed
{{else if eq .ActionName "reopen"}}
    reopened
{{else}}
    updated
{{end}}
{{if eq .ActionType "issue"}}
    issue
{{else}}
    pull request
{{end}}
#{{.Issue.Index}}: {{.Issue.Title}}
------------
<!DOCTYPE html>
<html>
<head>
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
    <title>{{.Subject}}</title>
</head>

<body>
    {{if .IsMention}}
    <p>
        You are receiving this because @{{.Doer.Name}} mentioned you.
    </p>
    {{end}}
    <p>
        <p>
        <a href="{{AppURL}}/{{.Doer.LowerName}}">@{{.Doer.Name}}</a>
        {{if not (eq .Doer.FullName "")}}
            ({{.Doer.FullName}})
        {{end}}
        {{if eq .ActionName "new"}}
            created
        {{else if eq .ActionName "close"}}
            closed
        {{else if eq .ActionName "reopen"}}
            reopened
        {{else}}
            updated
        {{end}}
        <a href="{{.Link}}">{{.Repo}}#{{.Issue.Index}}</a>.
        </p>
        {{if not (eq .Body "")}}
            <h3>Message content:</h3>
            <hr>
            {{.Body | Str2html}}
        {{end}}
    </p>
    <hr>
    <p>
        <a href="{{.Link}}">View it on Gitea</a>.
    </p>
</body>
</html>
```

This template produces something along these lines:

#### Subject

> [mike/stuff] @rhonda commented on pull request #38: New color palette

#### Mail body

> [@rhonda](#) (Rhonda Myers) updated [mike/stuff#38](#).
>
> #### Message content:
>
> \__________________________________________________________________
>
> Mike, I think we should tone down the blues a little.  
> \__________________________________________________________________
> 
> [View it on Gitea](#).

## Advanced

The template system contains several functions that can be used to further process and format
the messages. Here's a list of some of them:

| Name                 | Parameters  | Available | Usage                                                                        |
|----------------------|-------------|-----------|------------------------------------------------------------------------------|
| `AppUrl`             | -           | Any       | Gitea's URL                                                                  |
| `AppName`            | -           | Any       | Set from `app.ini`, usually "Gitea"                                          |
| `AppDomain`          | -           | Any       | Gitea's host name                                                            |
| `EllipsisString`     | string, int | Any       | Truncates a string to the specified length; adds ellipsis as needed          |
| `Str2html`           | string      | Body only | Sanitizes text by removing any HTML tags from it.                            |
| `Safe`               | string      | Body only | Takes the input as HTML; can be used for `.ReviewComments.RenderedContent`.  |

These are _functions_, not metadata, so they have to be used:

```
Like this:         {{Str2html "Escape<my>text"}}
Or this:           {{"Escape<my>text" | Str2html}}
Or this:           {{AppUrl}}
But not like this: {{.AppUrl}}
```
