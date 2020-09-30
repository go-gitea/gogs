---
date: "2020-01-25T21:00:00-03:00"
title: "Встроенный инструмент извлечения данных"
slug: "cmd-embedded"
weight: 40
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Встроенный инструмент извлечения данных"
    weight: 40
    identifier: "cmd-embedded"
---

# Встроенный инструмент извлечения данных

Исполняемый файл Gitea содержит все ресурсы, необходимые для запуска:
шаблоны, изображения, таблицы стилей и переводы. Любой из них можно
переопределить, поместив замену в соответствующий путь внутри каталога
`custom` (см. [Настройка Gitea]({{< relref "doc/advanced/customizing-gitea.ru-ru.md" >}})).

Чтобы получить копию встроенных ресурсов, готовую к редактированию, можно использовать команду `embedded`
из интерфейса командной строки и из интерфейса оболочки ОС.

**ПРИМЕЧАНИЕ:** Встроенный инструмент извлечения данных включен в Gitea версии 1.12 и выше.

## Список ресурсов

Чтобы перечислить ресурсы, встроенные в исполняемый файл Gitea, используйте следующий синтаксис:

```
gitea embedded list [--include-vendored] [patterns...]
```

Флаг `--include-vendored` заставляет команду включать файлы, предоставленные поставщиком,
которые обычно исключаются; то есть файлы из внешних библиотек, необходимые для Gitea
(например, [font-awesome](https://fontawesome.com/), [octicons](https://octicons.github.com/), и т.д.).

Может быть предоставлен список шаблонов поиска файлов. Gitea использует [gobwas/glob](https://github.com/gobwas/glob)
за его глобальный синтаксис. вот несколько примеров:

- Список всех файлов шаблонов в любом виртуальном каталоге: `**.tmpl`
- Список всех файлов почтовых шаблонов: `templates/mail/**.tmpl`
- Список всех файлов внутри `public/img`: `public/img/**`

Не забывайте использовать кавычки для шаблонов, поскольку пробелы, `*` и другие символы могут иметь
особое значение для вашей командной оболочки.

Если шаблон не указан, отображаются все файлы

#### Пример

Вывод списка всех встроенных файлов с `openid` в пути:

```
$ gitea embedded list '**openid**'
public/img/auth/openid_connect.svg
public/img/openid-16x16.png
templates/user/auth/finalize_openid.tmpl
templates/user/auth/signin_openid.tmpl
templates/user/auth/signup_openid_connect.tmpl
templates/user/auth/signup_openid_navbar.tmpl
templates/user/auth/signup_openid_register.tmpl
templates/user/settings/security_openid.tmpl
```

## Извлечение ресурсов

Чтобы извлечь ресурсы, встроенные в исполняемый файл Gitea, используйте следующий синтаксис:

```
gitea [--config {file}] embedded extract [--destination {dir}|--custom] [--overwrite|--rename] [--include-vendored] {patterns...}
```

Опция `--config` сообщает gitea расположение файла конфигурации`app.ini`,
если он не находится в местоположении по умолчанию. Эта опция используется только с флагом `--custom`.

Параметр `--destination` указывает gitea каталог, в который должны быть извлечены файлы.
По умолчанию это текущий каталог.

Флаг `--custom` указывает gitea извлекать файлы непосредственно в каталог `custom`.
Чтобы это работало, команде необходимо знать расположение файла конфигурации `app.ini`
(`--config`) и, в зависимости от конфигурации, запускаться из каталога, в котором
обычно запускается gitea. Прочтите [Customizing Gitea]({{< relref "doc/advanced/customizing-gitea.en-us.md" >}}) для большей информации.

Флаг `--overwrite` позволяет перезаписывать любые существующие файлы в каталоге назначения.

Флаг `--rename` указывает gitea переименовать любые существующие файлы в
каталоге назначения как `filename.bak`. Предыдущие файлы `.bak` перезаписываются.

Должен быть предоставлен хотя бы один шаблон поиска файла; синтаксис и примеры шаблонов
см. в подкоманде `list` выше.

#### Важное замечание

Убедись в **извлечении только тех файлов, которые требуют настройки**. Файлы,
находящиеся в каталоге `custom`, не обновляются в процессе обновления Gitea.
Когда Gitea обновляется до новой версии (путем замены исполняемого файла), многие встроенные
файлы претерпевают изменения. Gitea будет учитывать и использовать любые файлы, найденные в
каталоге `custom`, даже если они старые и несовместимые.

#### Пример

Извлечение почтовых шаблонов во временный каталог:

```
$ mkdir tempdir
$ gitea embedded extract --destination tempdir 'templates/mail/**.tmpl'
Extracting to tempdir:
tempdir/templates/mail/auth/activate.tmpl
tempdir/templates/mail/auth/activate_email.tmpl
tempdir/templates/mail/auth/register_notify.tmpl
tempdir/templates/mail/auth/reset_passwd.tmpl
tempdir/templates/mail/issue/assigned.tmpl
tempdir/templates/mail/issue/default.tmpl
tempdir/templates/mail/notify/collaborator.tmpl
```
