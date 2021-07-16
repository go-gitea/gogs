---
date: "2019-04-15T17:29:00+08:00"
title: "Migrations Interfaces"
slug: "migrations-interfaces"
weight: 30
toc: false
draft: false
menu:
  sidebar:
    parent: "developers"
    name: "Migrations Interfaces"
    weight: 55
    identifier: "migrations-interfaces"
---

# Migration Features

Currently, migrations from GitHub, Gitlab, Gogs, and other Gitea instances are implemented.
Gitea's migration feature defines two interfaces to support migrating
repository data from other git host platforms to Gitea or, in the future, migrating Gitea data to other
git host platforms.

First of all, Gitea defines some standard objects in packages [modules/migrations/base](https://github.com/go-gitea/gitea/tree/main/modules/migrations/base).  
They are `Repository`, `Milestone`, `Release`, `ReleaseAsset`, `Label`, `Issue`, `Comment`, `PullRequest`, `Reaction`, `Review`, `ReviewComment`.

## Downloader Interfaces

To migrate from a new git host platform, there are two steps to be updated.

- You should implement a `Downloader` which will be used to get repository information.
- You should implement a `DownloaderFactory` which will be used to detect if the URL matches and create the above `Downloader`.
   - You'll need to register the `DownloaderFactory` via `RegisterDownloaderFactory` on `init()`.

You can find these interfaces in [downloader.go](https://github.com/go-gitea/gitea/blob/main/modules/migrations/base/downloader.go).

## Uploader Interface

Currently, only a `GiteaLocalUploader` is implemented, so we only save downloaded
data via this `Uploader` to the local Gitea instance. Other uploaders are not supported at this time.

You can find these interfaces in [uploader.go](https://github.com/go-gitea/gitea/blob/main/modules/migrations/base/uploader.go).
