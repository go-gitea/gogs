[English](README.md)
[简体中文](README_ZH.md)

<h1> <img src="https://raw.githubusercontent.com/go-gitea/gitea/master/public/img/gitea-192.png" alt="logo" width="30" height="30"> Gitea - Git with a cup of tea</h1>

[![Статус версии](https://drone.gitea.io/api/badges/go-gitea/gitea/status.svg?ref=refs/heads/master)](https://drone.gitea.io/go-gitea/gitea)
[![Присоединяйтесь к нашему чату Discord через https://discord.gg/Gitea](https://img.shields.io/discord/322538954119184384.svg)](https://discord.gg/Gitea)
[![](https://images.microbadger.com/badges/image/gitea/gitea.svg)](https://microbadger.com/images/gitea/gitea "Получите свой собственный значок microbadger.com")
[![codecov](https://codecov.io/gh/go-gitea/gitea/branch/master/graph/badge.svg)](https://codecov.io/gh/go-gitea/gitea)
[![Go Report Card](https://goreportcard.com/badge/code.gitea.io/gitea)](https://goreportcard.com/report/code.gitea.io/gitea)
[![GoDoc](https://godoc.org/code.gitea.io/gitea?status.svg)](https://godoc.org/code.gitea.io/gitea)
[![GitHub релизы](https://img.shields.io/github/release/go-gitea/gitea.svg)](https://github.com/go-gitea/gitea/releases/latest)
[![Помогите внести свой вклад в развитие открытого исходного кода](https://www.codetriage.com/go-gitea/gitea/badges/users.svg)](https://www.codetriage.com/go-gitea/gitea)
[![Станьте стронником//спонсором gitea](https://opencollective.com/gitea/tiers/backers/badge.svg?label=backers&color=brightgreen)](https://opencollective.com/gitea)
[![Лицензия: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Crowdin](https://badges.crowdin.net/gitea/localized.svg)](https://crowdin.com/project/gitea)
[![Лист запланированного](https://badgen.net/https/api.tickgit.com/badgen/github.com/go-gitea/gitea)](https://www.tickgit.com/browse?repo=github.com/go-gitea/gitea)

## Цель

Цель этого проекта - сделать самый простой, быстрый и самый
удобный сервис собственного хостинга репозиториев Git.
Используя Go, это можно сделать с помощью независимого двоичного распределения по
**всем платформам** который поддерживает Go, включая Linux, macOS и Windows
на архитектурах x86, amd64, ARM и PowerPC.
Хотите попробовать, прежде чем делать что-нибудь ещё?
Попробуйте [с онлайн-демонстрацией](https://try.gitea.io/)!
Этот проект был
[форкнут](https://blog.gitea.io/2016/12/welcome-to-gitea/) от
[Gogs](https://gogs.io) с 2016.11 но многое изменилось.

## Сборка

Из корня source tree, выполнить:

    TAGS="bindata" make build

или если требуется поддержка sqlite:

    TAGS="bindata sqlite sqlite_unlock_notify" make build

Цель `build` будет разделена на две подцели:

- `make backend` что требует [Go 1.13](https://golang.org/dl/) или лучше.
- `make frontend` что требует [Node.js 10.13](https://nodejs.org/ru/download/) или лучше.

Если присутствуют предварительно созданные файлы внешнего интерфейса, можно создать только серверную часть:

		TAGS="bindata" make backend

Для этих целей параллелизм не поддерживается, поэтому не включайте `-j <num>`.

Больше информации: https://docs.gitea.io/ru-ru/install-from-source/

## Использование

    ./gitea web

ПРИМЕЧАНИЕ: Если вы заинтересованы в использовании нашего API, у нас есть экспериментальная
поддержка [документации](https://try.gitea.io/api/swagger).

## Содействие

Как это сделать?: Fork -> Patch -> Push -> Pull Request

ПРИМЕЧАНИЕ:

1. **ВЫ ДОЛЖНЫ ПРОЧИТАТЬ [РУКОВОДСТВО ДЛЯ СОУЧАСТНИКОВ](CONTRIBUTING.md) ПЕРЕД НАЧАЛОМ РАБОТЫ НАД PULL REQUEST'ОМ.**
2. Если вы обнаружили уязвимость в проекте, напишите в частном порядке по адресу **security@gitea.io**. Спасибо!

## Дальнейшая информация

Для получения дополнительной информации и инструкций по установке Gitea, пожалуйста, посмотрите
на нашу [документацию](https://docs.gitea.io/ru-ru/). Если у вас есть вопросы
которые не описаны в документации, вы можете связаться с нами по
нашему [Discord серверу](https://discord.gg/Gitea),
или [форуме](https://discourse.gitea.io/)!

## Авторство

* [Разработчики](https://github.com/orgs/go-gitea/people)
* [Участники развития](https://github.com/go-gitea/gitea/graphs/contributors)
* [Переводчики](options/locale/TRANSLATORS)

## Спонсорство

Спасибо всем нашим спонсорам! 🙏 [[Стать спонсором](https://opencollective.com/gitea#backer)]

<a href="https://opencollective.com/gitea#backers" target="_blank"><img src="https://opencollective.com/gitea/backers.svg?width=890"></a>

## Спонсоры

Поддержите этот проект, став спонсором. Здесь будет отображаться ваш логотип со ссылкой на ваш сайт. [[Стать спонсором](https://opencollective.com/gitea#sponsor)]

<a href="https://opencollective.com/gitea/sponsor/0/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/0/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/1/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/1/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/2/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/2/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/3/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/3/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/4/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/4/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/5/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/5/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/6/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/6/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/7/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/7/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/8/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/8/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/9/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/9/avatar.svg"></a>

## ЧАВО

**Как вы произносите Gitea?**

Gitea произносится как [/ги’ти:/](https://youtu.be/EM71-2uDAoY) с твёрдой г.

**Почему это не размещено на инстанции Gitea?**

Мы [работаем над этим](https://github.com/go-gitea/gitea/issues/1029).

## Лицензия

Этот проект находится под лицензией MIT License.
Просмотрите файл [ЛИЦЕНЗИИ](https://github.com/go-gitea/gitea/blob/master/LICENSE)
для полного текста лицензии.

## Скриншоты
Ищете обзор интерфейса? Зацените!

|![Панель управления](https://dl.gitea.io/screenshots/home_timeline.png)|![Профиль пользователя](https://dl.gitea.io/screenshots/user_profile.png)|![Общие задачи](https://dl.gitea.io/screenshots/global_issues.png)|
|:---:|:---:|:---:|
|![Ветки](https://dl.gitea.io/screenshots/branches.png)|![Веб-редактор текста](https://dl.gitea.io/screenshots/web_editor.png)|![Активность](https://dl.gitea.io/screenshots/activity.png)|
|![Новая миграция](https://dl.gitea.io/screenshots/migration.png)|![Миграция](https://dl.gitea.io/screenshots/migration.gif)|![Вид Pull Request'а](https://image.ibb.co/e02dSb/6.png)
![Тёмный Pull Request](https://dl.gitea.io/screenshots/pull_requests_dark.png)|![Тёмная рецензия на Diff(список измёненных файлов в коммите)](https://dl.gitea.io/screenshots/review_dark.png)|![Тёмный Diff(список измёненных файлов в коммите)](https://dl.gitea.io/screenshots/diff_dark.png)|
