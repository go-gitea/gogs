---
date: "2017-04-08T11:34:00+02:00"
title: "Особые переменные"
slug: "specific-variables"
weight: 20
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Особые переменные"
    weight: 20
    identifier: "specific-variables"
---

# Особые переменные

Это перечень переменных среды Gitea. Они меняют поведение Gitea.

Инициализируйте их перед командой Gitea, чтобы они были эффективными, например:

```
GITEA_CUSTOM=/home/gitea/custom ./gitea web
```

## Особенности языка Go

Поскольку Gitea написана на Go, в ней используются некоторые переменные Go,
например:

  * `GOOS`
  * `GOARCH`
  * [`GOPATH`](https://golang.org/cmd/go/#hdr-GOPATH_environment_variable)

Для документации по каждой из доступных переменных см.
[официальную документацию Go](https://golang.org/cmd/go/#hdr-Environment_variables).

## Файлы Gitea

  * `GITEA_WORK_DIR`: Абсолютный путь к рабочему каталогу.
  * `GITEA_CUSTOM`: Gitea по умолчанию использует папку `GITEA_WORK_DIR`/custom. Используйте ээту переменную
     для изменения каталога *custom*.
  * `GOGS_WORK_DIR`: Устарело, используйте `GITEA_WORK_DIR`
  * `GOGS_CUSTOM`: Устарело, используйте `GITEA_CUSTOM`

## Особенности операционной системы

  * `USER`: Системный пользователь, от имени которого будет работать Gitea. Используется для некоторых строк доступа к репозиторию.
  * `USERNAME`: если `USER` не найден, Gitea будет использовать `USERNAME`
  * `HOME`: Путь к домашнему каталогу пользователя. Переменная окружения `USERPROFILE` используется в Windows.

### Только в Windows

  * `USERPROFILE`: User home directory path. If empty, uses `HOMEDRIVE` + `HOMEPATH`
  * `HOMEDRIVE`: Main drive path used to access the home directory (C:)
  * `HOMEPATH`: Home relative path in the given home drive path

## Macaron (фреймворк, используемый Gitea)

  * `HOST`: Хост Macaron, который будет использован для прослушивания
  * `PORT`: Порт Macaron, который будет использован для прослушивания
  * `MACARON_ENV`: глобальная переменная для обеспечения специальных функций для сред разработки
     по сравнению с производственной средой. Если для MACARON_ENV установлено значение ""
	 или "development", то шаблоны будут перекомпилироваться при каждом запросе.
	 Для повышения производительности установите для переменной среды MACARON_ENV значение "production".

## Разное

  * `SKIP_MINWINSVC`: Если установлено значение 1, не запускать в Windows как службу.
