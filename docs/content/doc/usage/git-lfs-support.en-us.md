---
date: "2019-10-06T08:00:00+05:00"
title: "Usage: Git LFS setup"
slug: "git-lfs-setup"
weight: 12
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Git LFS setup"
    weight: 12
    identifier: "git-lfs-setup"
---

# Git Large File Storage setup

To use Gitea's built-in LFS support, you must update the `app.ini` file:

```ini
[server]
; Enables git-lfs support. true or false, default is false.
LFS_START_SERVER = true
; Where your lfs files reside, default is data/lfs.
LFS_CONTENT_PATH = /home/gitea/data/lfs
```