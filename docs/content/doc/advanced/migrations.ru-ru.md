---
date: "2019-04-15T17:29:00+08:00"
title: "Расширеное: Интерфейсы миграции"
slug: "migrations-interfaces"
weight: 30
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Интерфейсы миграции"
    weight: 55
    identifier: "migrations-interfaces"
---

# Интерфейсы миграции

Новые функции миграции были представлены в Gitea 1.9.0. Он определяет
два интерфейса для поддержки переноса данных репозиториев с других
хост-платформ git на gitea или, в будущем, для переноса данных gitea
на другие хост-платформы git. На данный момент реализована только миграция с github через APIv3 на Gitea.

Прежде всего, Gitea определяет некоторые стандартные объекты в пакетах `modules/migrations/base`. Они являются
 `Репозиторий`, `Этап`, `Релиз`, `Метка`, `Задача`, `Комментарий`, `PullRequest`, `Реакция`, `Рецензия`, `Рецензионный комментарий`.

## Интерфейсы загрузчика

Чтобы перейти с новой хост-платформы git, нужно выполнить два шага.

- Вы должны внедрить `Downloader`, который будет получать все виды информации из репозитория.
- Вы должны внедрить `DownloaderFactory`, который используется для определения соответствия URL-адреса
и создания загрузчика.
- Вам нужно будет зарегистрировать `DownloaderFactory` через `RegisterDownloaderFactory` при инициализации.

```Go
type Downloader interface {
	SetContext(context.Context)
	GetRepoInfo() (*Repository, error)
	GetTopics() ([]string, error)
	GetMilestones() ([]*Milestone, error)
	GetReleases() ([]*Release, error)
	GetLabels() ([]*Label, error)
	GetIssues(page, perPage int) ([]*Issue, bool, error)
	GetComments(issueNumber int64) ([]*Comment, error)
	GetPullRequests(page, perPage int) ([]*PullRequest, error)
	GetReviews(pullRequestNumber int64) ([]*Review, error)
}
```

```Go
type DownloaderFactory interface {
	Match(opts MigrateOptions) (bool, error)
	New(opts MigrateOptions) (Downloader, error)
}
```

## Интерфейс загрузчика

В настоящее время реализован только `GiteaLocalUploader`, поэтому мы
сохраняем загруженные данные только через этот`Uploader` в локальном экземпляре Gitea.
Другие программы загрузки не поддерживаются и будут реализованы в будущем.

```Go
// Uploader uploads all the informations
type Uploader interface {
	MaxBatchInsertSize(tp string) int
	CreateRepo(repo *Repository, opts MigrateOptions) error
	CreateTopics(topic ...string) error
	CreateMilestones(milestones ...*Milestone) error
	CreateReleases(releases ...*Release) error
	SyncTags() error
	CreateLabels(labels ...*Label) error
	CreateIssues(issues ...*Issue) error
	CreateComments(comments ...*Comment) error
	CreatePullRequests(prs ...*PullRequest) error
	CreateReviews(reviews ...*Review) error
	Rollback() error
	Close()
}

```
