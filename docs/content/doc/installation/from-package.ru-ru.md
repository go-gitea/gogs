---
date: "2016-12-01T16:00:00+02:00"
title: "Установка из пакета"
slug: "install-from-package"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Из пакета"
    weight: 20
    identifier: "install-from-package"
---

# Установка из package

## Debian

Хотя тут есть пакет Gitea, [вклада](https://wiki.debian.org/SourcesList) Debian,
он не поддерживается напрямую нами.

К сожалению, пакет больше не поддерживается и сломан из-за отсутствия исходников.
Пожалуйста, следуйте руководству [развёртыванию из двоичного]({{< relref "from-binary.ru-ru.md" >}}).

Если пакеты будут обновлены и исправлены, мы предоставим здесь актуальные инструкции по установке.

## Alpine Linux

Alpine Linux имеет gitea в репозитории сообщества. Следует за последней стабильной версией.
для получения дополнительной информации см. https://pkgs.alpinelinux.org/packages?name=gitea&branch=edge.

установка как обычно:
```sh
apk add gitea
```
config находится в **/etc/gitea/app.ini**

## Windows

Eсть [Gitea](https://chocolatey.org/packages/gitea) пакет для Windows от [Chocolatey](https://chocolatey.org/).

```sh
choco install gitea
```

Или следуйте руководству [развёртывания из двоичного]({{< relref "from-binary.ru-ru.md" >}}).
## macOS

В настоящее время единственный поддерживаемый метод установки на MacOS - это [Homebrew](http://brew.sh/).
Следование руководству [развёртывания из двоичного]({{< relref "from-binary.ru-ru.md" >}}) может помочь,
но не поддерживается. Чтобы установить Gitea через `brew`:

```
brew tap gitea/tap https://gitea.com/gitea/homebrew-gitea
brew install gitea
```

## FreeBSD

Доступен порт FreeBSD `www/gitea`. Чтобы установить предварительно созданный двоичный пакет:

```
pkg install gitea
```

Для получения самой последней версии или для создания порта с настраиваемыми параметрами,
[установить его из порта](https://www.freebsd.org/doc/handbook/ports-using.html):

```
su -
cd /usr/ports/www/gitea
make install clean
```

Порт использует стандартную структуру файловой системы FreeBSD: файлы конфигурации находятся в `/usr/local/etc/gitea`,
объединённые шаблоны, параметры, плагины и темы находятся в `/usr/local/share/gitea`, и стартовый скрипт
в `/usr/local/etc/rc.d/gitea`.

Чтобы Gitea работала как служба, запустите `sysrc gitea_enable=YES` и начните с `service gitea start`.

## Cloudron

Gitea доступна для установки в один клик на [Cloudron](https://cloudron.io). 
Cloudron позволяет легко запускать приложения, такие как Gitea, на вашем сервере и поддерживать их в актуальном состоянии и обеспечивать безопасность.

[![Установить](https://cloudron.io/img/button.svg)](https://cloudron.io/button.html?app=io.gitea.cloudronapp)

Пакет Gitea поддерживается [здесь](https://git.cloudron.io/cloudron/gitea-app).

Eсть [демонстрационный экземпляр](https://my.demo.cloudron.io) (username: cloudron password: cloudron) где
вы можете поэкспериментировать с запуском Gitea.

## Третья сторона

Существуют различные другие сторонние пакеты Gitea. 
Чтобы увидеть тщательно подобранный список, перейдите на [awesome-gitea](https://gitea.com/gitea/awesome-gitea/src/branch/master/README.md#user-content-packages).

Знаете о существующем пакете, которого нет в списке? Отправьте PR, чтобы добавить его!
