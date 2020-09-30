---
date: "2017-01-01T16:00:00+02:00"
title: "Использование: резервное копирование и восстановление"
slug: "backup-and-restore"
weight: 11
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Резервное копирование и восстановление"
    weight: 11
    identifier: "backup-and-restore"
---

# Резервное копирование и восстановление

В настоящее время в Gitea есть команда `dump`, которая сохранит установку в zip-файл. Этот
файл можно распаковать и использовать для восстановления экземпляра.

## Команда резервного копирования (`dump`)

Переключитесь на пользователя, работающего с Gitea: `su git`. Запустите `./gitea dump -c /path/to/app.ini` в каталоге установки
Gitea. Должен быть какой-то вывод, подобный следующему:

```none
2016/12/27 22:32:09 Creating tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:09 Dumping local repositories.../home/git/gitea-repositories
2016/12/27 22:32:22 Dumping database...
2016/12/27 22:32:22 Packing dump files...
2016/12/27 22:32:34 Removing tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:34 Finish dumping in file gitea-dump-1482906742.zip
```

Внутри файла `gitea-dump-1482906742.zip`, будет следующим:

* `app.ini` - Необязательная копия файла конфигурации, если изначально он хранится за пределами каталога по умолчанию custom/`
* `custom` - Все файлы конфигурации или настройки в `custom/`.
* `data` - Каталог данных в <GITEA_WORK_DIR>, кроме сеансов, если вы используете сеанс файлов. Этот каталог включает в себя `attachments`, `avatars`, `lfs`, `indexers`, файл sqlite, если вы используете sqlite.
* `gitea-db.sql` - SQL-дамп базы данных
* `gitea-repo.zip` - Полная копия каталога репозитория.
* `log/` - Различные журналы. Они не нужны для восстановления или миграции.

Файлы промежуточных резервных копий создаются во временном каталоге, указанном либо с помощью
параметром командной строки `--tempdir` или `TMPDIR` переменной окружения.

### С помощью Docker (`dump`)

Есть несколько предостережений при использовании команды `dump` с Docker.

Команда должна выполняться с `RUN_USER = <OS_USERNAME>` указанной в `gitea/conf/app.ini`; и, чтобы архивирование резервной папки происходило без ошибки разрешения, команда `docker exec` должна выполняться внутри `--tempdir`.

Пример:

```none
docker exec -u <OS_USERNAME> -it -w <--tempdir> $(docker ps -qf "name=<NAME_OF_DOCKER_CONTAINER>") bash -c '/app/gitea/gitea dump -c </path/to/app.ini>'
```

*Примечание: `--tempdir` относится к временному каталогу среды docker, используемому Gitea; если вы не указали свой `--tempdir`, тогда Gitea использует `/tmp` или `TMPDIR` переменную окружения контейнера docker. Для `--tempdir` отрегулируйте свои параметры команды `docker exec` соответственно.

Результатом должен быть файл, хранящийся в `--tempdir` указанный, по линиям: `gitea-dump-1482906742.zip`

## Команда восстановления (`restore`)

В настоящее время команда восстановления не поддерживается. Это ручной процесс, который
в основном включает перемещение файлов в их правильные места и восстановление дампа базы данных.

Пример:

```none
apt-get install gitea
unzip gitea-dump-1482906742.zip
cd gitea-dump-1482906742
mv custom/conf/app.ini /etc/gitea/conf/app.ini # or mv app.ini /etc/gitea/conf/app.ini
unzip gitea-repo.zip
mv gitea-repo/* /var/lib/gitea/repositories/
chown -R gitea:gitea /etc/gitea/conf/app.ini /var/lib/gitea/repositories/
mysql --default-character-set=utf8mb4 -u$USER -p$PASS $DATABASE <gitea-db.sql
# or  sqlite3 $DATABASE_PATH <gitea-db.sql
service gitea restart
```

Репозиторий git-hook'ы необходимо регенерировать, если метод установки был изменён (например, binary -> Docker), или если Gitea установлен в другой каталог, чем предыдущая установка.

При запущенном Gitea и из каталога, в котором находится двоичный файл Gitea, выполните: `./gitea admin regenerate hooks`

Это гарантирует, что пути к файлам приложения и конфигурации в git-hook'ах репозитория согласованы и применимы к текущей установке. Если эти пути не обновляются, действия репозитория `push` не удастся.
