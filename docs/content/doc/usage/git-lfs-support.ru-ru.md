---
date: "2019-10-06T08:00:00+05:00"
title: "Использование: Настройка Git LFS"
slug: "git-lfs-setup"
weight: 12
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Настройка Git LFS"
    weight: 12
    identifier: "git-lfs-setup"
---

# Настройка Large File Storage Git

Чтобы использовать встроенную поддержку LFS в Gitea, необходимо обновить файл `app.ini`:

```ini
[server]
; Enables git-lfs support. true or false, default is false.
LFS_START_SERVER = true
; Where your lfs files reside, default is data/lfs.
LFS_CONTENT_PATH = /home/gitea/data/lfs
```