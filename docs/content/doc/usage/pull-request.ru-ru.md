---
date: "2018-06-01T19:00:00+02:00"
title: "Использование: Pull Request'а"
slug: "pull-request"
weight: 13
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Pull Request"
    weight: 13
    identifier: "pull-request"
---

# Pull Request

## "Незавёршенные" pull request'ы

Пометка pull request'а как незавёршенного предотвратит случайное слияние этого pull request'а. Чтобы пометить pull request как незавершённый, вы должны добавить к его заголовку префикс `WIP:` или `[WIP]` (без учёта регистра). Эти значения можно настроить в файле `app.ini` :

```
[repository.pull-request]
WORK_IN_PROGRESS_PREFIXES=WIP:,[WIP]
```

Первое значение списка будет использоваться в помощниках.

## Шаблоны Pull Request'а

Дополнительную информацию о pull request'ах можно найти на странице [Issue and Pull Request templates](../issue-pull-request-templates).
