---
date: "2017-01-14T11:00:00-02:00"
title: "Make"
slug: "make"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Make"
    weight: 30
    identifier: "make"
---

# Make

Gitea активно использует Make для автоматизации задач и улучшения разработки. В этом
руководстве рассказывается, как установить Make.

### На Linux

Установить с помощью диспетчера пакетов.

На Ubuntu/Debian:

```bash
sudo apt-get install make
```

На Fedora/RHEL/CentOS:

```bash
sudo yum install make
```

### На Windows

Один из этих трёх дистрибутивов Make будет работать в Windows:

- [Single binary build](http://www.equation.com/servlet/equation.cmd?fa=make). Скопируйте куда-нибудь и добавьте в `PATH`.
  - [32-битная версия](ftp://ftp.equation.com/make/32/make.exe)
  - [64-битная версия](ftp://ftp.equation.com/make/64/make.exe)
- [MinGW](http://www.mingw.org/) включает сборку.
  - Бинарный файл называется `mingw32-make.exe` вместо того `make.exe`. Добавьте папку `bin` в `PATH`.
- [Chocolatey package](https://chocolatey.org/packages/make). Запустите `choco install make`
