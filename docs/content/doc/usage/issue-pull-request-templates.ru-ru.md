---
date: "2018-05-10T16:00:00+02:00"
title: "Использование: Шаблоны задач и Pull Request'ов"
slug: "issue-pull-request-templates"
weight: 15
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Шаблоны задач и Pull Request'ов"
    weight: 15
    identifier: "issue-pull-request-templates"
---

# Шаблоны задач и Pull Request'ов

В некоторых проектах есть стандартный список вопросов, на которые пользователи должны
ответить при создании задачи или Pull Request'а. Gitea поддерживает добавление
шаблонов в основную ветку репозитория, чтобы они могли автоматически заполнять форму,
когда пользователи создают задачи и Pull Request'ы. Это сократит начальные
попытки получить некоторые уточняющие детали.

Возможные имена файлов для шаблонов задач:

* ISSUE_TEMPLATE.md
* issue_template.md
* .gitea/ISSUE_TEMPLATE.md
* .gitea/issue_template.md
* .github/ISSUE_TEMPLATE.md
* .github/issue_template.md


Возможные имена файлов для шаблонов PR:

* PULL_REQUEST_TEMPLATE.md
* pull_request_template.md
* .gitea/PULL_REQUEST_TEMPLATE.md
* .gitea/pull_request_template.md
* .github/PULL_REQUEST_TEMPLATE.md
* .github/pull_request_template.md


Кроме того, URL-адрес страницы новой pflfxb может быть дополнен суффиксом `?Body=Issue+Text`, и форма будет заполнена этой строкой. Эта строка будет использоваться вместо шаблона, если он есть.
