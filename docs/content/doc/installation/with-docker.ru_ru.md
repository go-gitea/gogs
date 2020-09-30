---
date: "2020-03-19T19:27:00+02:00"
title: "Установка с помощью Docker"
slug: "install-with-docker"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "С помощью Docker"
    weight: 10
    identifier: "install-with-docker"
---

# Установка с помощью Docker

Gitea предоставляет автоматически обновляемые образы Docker внутри своей организации Docker Hub. Всегда
можно использовать последний стабильный тег или другую службу, которая обрабатывает обновление
изображения Docker.

Эта справочная установка поможет пользователям выполнить установку на основе `docker-compose`, но установка
из `docker-compose` выходит за рамки данной документации. Чтобы установить сам `docker-compose`, следуйте
официальной [инструкции по установке](https://docs.docker.com/compose/install/).

## Основы

Самая простая установка просто создаёт том и сеть и запускает `gitea/gitea:latest`
изображение как службу. Поскольку базы данных нет, её можно инициализировать с помощью SQLite3.
Создайте каталог вроде `gitea` и вставьте следующий контент в файл с именем `docker-compose.yml`.
Обратите внимание, что том должен принадлежать пользователю/группе с UID/GID, указанным в файле конфигурации.
Если вы не предоставите тому правильные разрешения, контейнер может не запуститься.
Также имейте в виду, что тег `:latest` установит текущую разрабатываемую версию.
Для стабильного релиза вы можете использовать `:1` или укажите конкретный релиз, например `:{{< version >}}`.

```yaml
version: "2"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:latest
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
```

## Пользовательский порт

Чтобы привязать интегрированный daemon openSSH и веб-сервер к другому порту, настройте
раздел порта. Обычно просто меняют порт хоста и оставляют порты в пределах
контейнера.

```diff
version: "2"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:latest
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
-      - "3000:3000"
-      - "222:22"
+      - "8080:3000"
+      - "2221:22"
```

## База данных MySQL

Чтобы запустить Gitea в сочетании с базой данных MySQL, примените эти изменения к файлу
`docker-compose.yml` созданному выше.

```diff
version: "2"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:latest
    environment:
      - USER_UID=1000
      - USER_GID=1000
+      - DB_TYPE=mysql
+      - DB_HOST=db:3306
+      - DB_NAME=gitea
+      - DB_USER=gitea
+      - DB_PASSWD=gitea
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
     ports:
       - "3000:3000"
       - "222:22"
+    depends_on:
+      - db
+
+  db:
+    image: mysql:5.7
+    restart: always
+    environment:
+      - MYSQL_ROOT_PASSWORD=gitea
+      - MYSQL_USER=gitea
+      - MYSQL_PASSWORD=gitea
+      - MYSQL_DATABASE=gitea
+    networks:
+      - gitea
+    volumes:
+      - ./mysql:/var/lib/mysql
```

## База данных PostgreSQL

Чтобы запустить Gitea в сочетании с базой данных PostgreSQL, примените эти изменения к
файлу `docker-compose.yml` созданному выше.

```diff
version: "2"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:latest
    environment:
      - USER_UID=1000
      - USER_GID=1000
+      - DB_TYPE=postgres
+      - DB_HOST=db:5432
+      - DB_NAME=gitea
+      - DB_USER=gitea
+      - DB_PASSWD=gitea
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
+    depends_on:
+      - db
+
+  db:
+    image: postgres:9.6
+    restart: always
+    environment:
+      - POSTGRES_USER=gitea
+      - POSTGRES_PASSWORD=gitea
+      - POSTGRES_DB=gitea
+    networks:
+      - gitea
+    volumes:
+      - ./postgres:/var/lib/postgresql/data
```

## Именованные тома

Чтобы использовать именованные тома вместо томов хоста, определите и используйте именованный том
в пределах конфигурации `docker-compose.yml`. Это изменение автоматически
создаст необходимый объём. Вам не нужно беспокоиться о разрешениях с
именованные тома; Docker справится с этим автоматически.

```diff
version: "2"

networks:
  gitea:
    external: false

+volumes:
+  gitea:
+    driver: local
+
services:
  server:
    image: gitea/gitea:latest
    restart: always
    networks:
      - gitea
    volumes:
-      - ./gitea:/data
+      - gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
```

Контейнеры MySQL или PostgreSQL нужно будет создавать отдельно.

## Начало

Чтобы начать эту настройку на основе `docker-compose`, выполните `docker-compose up -d`,
для запуска Gitea в фоновом режиме. С помощью `docker-compose ps` покажет, если Gitea
запустился правильно. Журнал можно просмотреть с помощью `docker-compose logs`.

Чтобы завершить установку, выполните `docker-compose down`. Это остановит
и выключит контейнеры. Тома по-прежнему будут существовать.

Примечание: при использовании порта http, отличного от 3000, измените app.ini в соответствии с
`LOCAL_ROOT_URL = http://localhost:3000/`.

## Установка

После запуска установки Docker через `docker-compose`, Gitea должен быть доступен с использованием
в любимом браузере, чтобы завершить установку. Посетите http://server-ip:3000 и следуйте
указаниям мастера установки. Если база данных была запущена с установкой `docker-compose`,
как описано выше, обратите внимание, что `db` должно использоваться как имя хоста базы данных.

## Окружающая среда переменных

Вы можете настроить некоторые параметры Gitea с помощью переменных окружающей среды:

(Значения по умолчанию приведены в **жирном тексте**)

* `APP_NAME`: **"Gitea: Git with a cup of tea"**: Название приложения, используемое в заголовке страницы.
* `RUN_MODE`: **dev**: Для повышения производительности и других целей измените это значение на `prod` при развёртывании в производственной среде.
* `DOMAIN`: **localhost**: Доменное имя этого сервера, используемое для отображаемого URL клона http в пользовательском интерфейсе Gitea.
* `SSH_DOMAIN`: **localhost**: Доменное имя этого сервера, используемое для отображаемого URL-адреса клона ssh в пользовательском интерфейсе Gitea. Если страница установки включена, сервер домена SSH принимает значение DOMAIN в форме (которое перезаписывает этот параметр при сохранении).
* `SSH_PORT`: **22**: SSH порт отображается в клоне URL.
* `SSH_LISTEN_PORT`: **%(SSH\_PORT)s**: Порт для встроенного SSH-сервера.
* `DISABLE_SSH`: **false**: Отключите функцию SSH, если она недоступна.
* `HTTP_PORT`: **3000**: Прослушивающий порт HTTP
* `ROOT_URL`: **""**: Замените автоматически созданный общедоступный URL. Это полезно, если внутренний и внешний URL-адреса не совпадают (например, в Docker).
* `LFS_START_SERVER`: **false**: Включает поддержку git-lfs.
* `DB_TYPE`: **sqlite3**: Тип используемой базы данных \[mysql, postgres, mssql, sqlite3\].
* `DB_HOST`: **localhost:3306**: Адрес и порт хоста базы данных.
* `DB_NAME`: **gitea**: Имя базы данных.
* `DB_USER`: **root**: Имя пользователя базы данных.
* `DB_PASSWD`: **"\<empty>"**: Пароль пользователя базы данных. Используйте \`your password\` для цитирования, если вы используете в пароле специальные символы.
* `INSTALL_LOCK`: **false**: Запретить доступ к странице установки.
* `SECRET_KEY`: **""**: Глобальный секретный ключ. Это следует изменить. Если это имеет значение и `INSTALL_LOCK` пуст, `INSTALL_LOCK` автоматически установится на `true`.
* `DISABLE_REGISTRATION`: **false**: Отключить регистрацию, после чего только администратор сможет создавать учётные записи для пользователей.
* `REQUIRE_SIGNIN_VIEW`: **false**: Включите это, чтобы заставить пользователей входить в систему для просмотра любой страницы.
* `USER_UID`: **1000**: UID (идентификатор пользователя Unix) пользователя, который запускает Gitea в контейнере. Сопоставьте это с UID владельца `/data` при использовании томов хоста (это не обязательно для именованных томов).
* `USER_GID`: **1000**: GID (идентификатор группы Unix) пользователя, запускающего Gitea в контейнере. Сопоставьте это с GID владельца `/data` при использовании томов хоста (это не обязательно для именованных томов).

# Настройка

Описание файлов настройки находится [тут](https://docs.gitea.io/ru-ru/customizing-gitea/) должен
быть помещён в `/data/gitea` directory. При использовании томов хоста получить доступ к
файлам; для именованных томов это делается через другой контейнер или путём прямого доступа 
`/var/lib/docker/volumes/gitea_gitea/_data`. Файл конфигурации будет сохранён в
`/data/gitea/conf/app.ini` после установки.

# Обновление

:exclamation::exclamation: **Убедитесь, что у вас есть объёмные данные за пределами контейнера Docker** :exclamation::exclamation:

Чтобы обновить вашу установку до последней версии:
```
# Измените `docker-compose.yml`, чтобы обновить версию, если она у вас указана
# Залейте новые изменения
docker-compose pull
# Запустите новый контейнер, автоматически удаляя старый
docker-compose up -d
```

# Транспортировка контейнера SSH

Поскольку SSH работает внутри контейнера, вам нужно передать SSH с хоста на
контейнер, если вы хотите использовать поддержку SSH. Если вы хотите сделать это без запуска контейнера
SSH на нестандартном порту (или переместите порт хоста на нестандартный порт), вы можете перенаправить
подключения SSH, предназначенные для контейнера, с небольшой дополнительной установкой.

В этом руководстве предполагается, что вы создали пользователя на хосте с именем `git` который разделяет то же самое значение
UID/GID как значения контейнера `USER_UID`/`USER_GID`. Вы также должны создать каталог
`/var/lib/gitea` на хосте, принадлежащем пользователю `git` и установлен в контейнере, например.

```
  services:
    server:
      image: gitea/gitea:latest
      environment:
        - USER_UID=1000
        - USER_GID=1000
      restart: always
      networks:
        - gitea
      volumes:
        - /var/lib/gitea:/data
        - /etc/timezone:/etc/timezone:ro
        - /etc/localtime:/etc/localtime:ro
      ports:
        - "3000:3000"
        - "127.0.0.1:2222:22"
```

Вы можете видеть, что мы также открываем SSH-порт контейнера для порта 2222 на хосте и связываем этот
на 127.0.0.1, чтобы предотвратить доступ к нему извне по отношению к самому хост-компьютеру.

На **хосте**, вы должны создать файл `/app/gitea/gitea` со следующим содержанием и
сделать его исполняемым (`chmod +x /app/gitea/gitea`):

```
#!/bin/sh
ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 "SSH_ORIGINAL_COMMAND=\"$SSH_ORIGINAL_COMMAND\" $0 $@"
```

Вашему пользователю `git` необходимо создать ключ SSH:

```
sudo -u git ssh-keygen -t rsa -b 4096 -C "Gitea Host Key"
```

Всё ещё на хосте, символическая ссылка на контейнер `.ssh/authorized_keys` файл вашему пользователю git `.ssh/authorized_keys`.
Это можно сделать на хосте как каталогом `/var/lib/gitea` монтируется внутри контейнера под `/data`:

```
ln -s /var/lib/gitea/git/.ssh/authorized_keys /home/git/.ssh/authorized_keys
```

Затем повторите SSH-ключ пользователя `git` в файл authorized_keys, чтобы хост мог общаться с контейнером по SSH:

```
echo "no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty $(cat /home/git/.ssh/id_rsa.pub)" >> /var/lib/gitea/git/.ssh/authorized_keys
```

Теперь вы должны иметь возможность использовать Git через SSH для своего контейнера, не прерывая SSH-доступ к хосту.

Обратите внимание: сквозной проход контейнера SSH будет работать только при использовании opensshd в контейнере и не будет работать, если
`AuthorizedKeysCommand` используется в сочетании с настройкой `SSH_CREATE_AUTHORIZED_KEYS_FILE=false`, чтобы отключить
генерацию ключей авторизованных файлов.
