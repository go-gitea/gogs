---
date: "2016-12-21T15:00:00-02:00"
title: "Зарегистрировать в службе Windows"
slug: "windows-service"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Служба Windows"
    weight: 30
    identifier: "windows-service"
---

# Предпосылки

Следующие изменения внесены в C:\gitea\custom\conf\app.ini:

```
RUN_USER = COMPUTERNAME$
```

Устанавливает Gitea для запуска от имени пользователя локальной системы.

COMPUTERNAME какой бы ни был ответ `echo %COMPUTERNAME%` в командной строке. Если ответ `USER-PC` тогда `RUN_USER = USER-PC$`

## Использование абсолютных путей

Если вы используете sqlite3, измените `PATH`, чтобы включить полный путь:

```
[database]
PATH     = c:/gitea/data/gitea.db
```

# Регистрация службы Windows

Чтобы зарегистрировать Gitea как службу Windows, откройте командную строку (cmd) от имени администратора,
затем запустите следующую команду:

```
sc.exe create gitea start= auto binPath= "\"C:\gitea\gitea.exe\" web --config \"C:\gitea\custom\conf\app.ini\""
```

Не забудьте заменить `C:\gitea` с правильным каталогом Gitea.

Откройте "Службы Windows", найдите службу с именем "gitea", щёлкните ее правой кнопкой мыши и выберите
"Запустить". Если всё в порядке, Gitea будет доступен через `http://localhost:3000` (или порт,
который был настроен).

## Отменить регистрацию службы

Чтобы отменить регистрацию Gitea как службы, откройте командную строку (cmd) от имени администратора и запустите:

```
sc.exe delete gitea
```
