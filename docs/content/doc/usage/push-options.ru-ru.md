---
date: "2020-07-06T16:00:00+02:00"
title: "Использование: Push Options"
slug: "push-options"
weight: 15
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Push Options"
    weight: 15
    identifier: "push-options"
---

# Push Options

В Gitea `1.13`, существует поддержка некоторых [push options](https://git-scm.com/docs/git-push#Documentation/git-push.txt--oltoptiongt),
которые были добавлены.


## Поддерживаемые параметры

- `repo.private` (true|false) - Измените видимость репозитория. 
Это особенно полезно в сочетании с нажатием на создание.
- `repo.template` (true|false) - Измените, является ли репозиторий шаблоном.

Пример изменения видимости репозитория на публичный:  
```shell
git push -o repo.private=false -u origin master
```
