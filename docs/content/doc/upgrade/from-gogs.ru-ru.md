---
date: "2016-12-01T16:00:00+02:00"
title: "Обновление от Gogs"
slug: "upgrade-from-gogs"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "upgrade"
    name: "От Gogs"
    weight: 10
    identifier: "upgrade-from-gogs"
---

# Обновление от Gogs

Gogs, версии 0.9.146 и новее, можно легко мигрировать на Gitea.

Необходимо выполнить несколько основных шагов. В системе Linux запустите как пользователь Gogs:

* Создайте резервную копию Gogs с помощью `gogs backup`. Это создает файл `gogs-backup-[timestamp].zip`
  содержащий все важные данные Gogs. Он понадобится вам, если вы захотите мигрировать в `gogs` обратно.
* Загрузите файл, соответствующий целевой платформе, с [страницы загрузок](https://dl.gitea.io/gitea/).
 Должно быть `1.0.x` версии. Миграция из `gogs` в любую другую версию невозможен.
* Поместите двоичный файл в желаемое место установки.
* Скопируйте `gogs/custom/conf/app.ini` в `gitea/custom/conf/app.ini`.
* Скопируйте пользовательский `templates, public` из `gogs/custom/` в `gitea/custom/`.
* Для любых других пользовательских папок, например `gitignore, label, license, locale, readme` в
  `gogs/custom/conf`, скопируйте их в `gitea/custom/options`.
* Скопируйте `gogs/data/` в `gitea/data/`. Он содержит вложения задач и аватары.
* Проверить, запустив Gitea с помощью `gitea web`.
* Войдите в панель администратора Gitea в пользовательском интерфейсе, запустите `Rewrite '.ssh/authorized_keys' file`.
* Запустите все основные версии двоичного файла ( `1.1.4` → `1.2.3` → `1.3.4` → `1.4.2` →  и т.д. ), чтобы мигрировать базу данных.
* Если пользовательский или конфигурационный путь был изменён, запустите `Rewrite all update hook of repositories`.

## Изменить специальную информацию о gogs

* Переименуйте `gogs-repositories/` на `gitea-repositories/`
* Переименуйте `gogs-data/` на `gitea-data/`
* В `gitea/custom/conf/app.ini` измените:

  ИЗ:

  ```ini
  [database]
  PATH = /home/:USER/gogs/data/:DATABASE.db
  [attachment]
  PATH = /home/:USER/gogs-data/attachments
  [picture]
  AVATAR_UPLOAD_PATH = /home/:USER/gogs-data/avatars
  [log]
  ROOT_PATH = /home/:USER/gogs/log
  ```

  НА:

  ```ini
  [database]
  PATH = /home/:USER/gitea/data/:DATABASE.db
  [attachment]
  PATH = /home/:USER/gitea-data/attachments
  [picture]
  AVATAR_UPLOAD_PATH = /home/:USER/gitea-data/avatars
  [log]
  ROOT_PATH = /home/:USER/gitea/log
  ```

* Подтвердите, запустив Gitea с `gitea web`

## Обновление до самой последней версии `gitea`

После успешной миграции с `gogs` to `gitea 1.0.x`, можно обновить `gitea` к современной версии
в два этапа.

Сначала обновите до [`gitea 1.6.4`](https://dl.gitea.io/gitea/1.6.4/). Скачайте файл соответствующий
платформе назначения из [страницы загрузок](https://dl.gitea.io/gitea/1.6.4/) и замените двоичный.
Запустите Gitea хотя бы один раз и убедитесь, что всё работает должным образом.

Затем повторите процедуру, но на этот раз с помощью [последнего релиза](https://dl.gitea.io/gitea/{{< version >}}/).

## Обновление с более новой версии Gogs

Обновление с более новой версии Gogs также возможно, но требует немного больше работы. 
Просмотрите [#4286](https://github.com/go-gitea/gitea/issues/4286).

## Исправление проблем

* Если обнаружены ошибки, связанные с пользовательскими шаблонами в папке `gitea/custom/templates`
  , попробуйте переместить шаблоны, вызывающие ошибки, один за другим. Они не могут быть
   совместимыми с Gitea или обновлением.

## Добавьте Gitea в автозагрузку в Unix

Обновите соответствующий файл из [gitea/contrib](https://github.com/go-gitea/gitea/tree/master/contrib)
с правильными переменными среды.

Для дистрибутивов с systemd:

* Скопируйте обновлённый скрипт в `/etc/systemd/system/gitea.service`
* Добавьте сервис в автозагрузку с: `sudo systemctl enable gitea`
* Отключите старый скрипт автозагрузки gogs: `sudo systemctl disable gogs`

Для дистрибутивов с SysVinit:

* Скопируйте обновлённый скрипт в `/etc/init.d/gitea`
* Добавьте сервис в автозагрузку с: `sudo rc-update add gitea`
* Отключите старый скрипт автозагрузки gogs: `sudo rc-update del gogs`
