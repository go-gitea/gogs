---
date: "2016-12-01T16:00:00+02:00"
title: "Installation from package"
slug: "install-from-package"
weight: 20
toc: false
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "From package"
    weight: 20
    identifier: "install-from-package"
---

# Installation from package

**Table of Contents**

{{< toc >}}

## Alpine Linux

Alpine Linux has [Gitea](https://pkgs.alpinelinux.org/packages?name=gitea&branch=edge) in its community repository which follows the latest stable version.

```sh
apk add gitea
```

## Arch Linux

The rolling release distribution has [Gitea](https://www.archlinux.org/packages/community/x86_64/gitea/) in their official community repository and package updates are provided with new Gitea releases.

```sh
pacman -S gitea
```

## Arch Linux ARM

Arch Linux ARM provides packages for [aarch64](https://archlinuxarm.org/packages/aarch64/gitea), [armv7h](https://archlinuxarm.org/packages/armv7h/gitea) and [armv6h](https://archlinuxarm.org/packages/armv6h/gitea).

```sh
pacman -S gitea
```

## SUSE and openSUSE

OpenSUSE build service provides packages for [openSUSE and SLE](https://software.opensuse.org/download/package?package=gitea&project=devel%3Atools%3Ascm) 
in the Development Software Configuration Management Repository

## Windows

There is a [Gitea](https://chocolatey.org/packages/gitea) package for Windows by [Chocolatey](https://chocolatey.org/).

```sh
choco install gitea
```

Or follow the [deployment from binary]({{< relref "from-binary.en-us.md" >}}) guide.

## macOS

Currently, the only supported method of installation on MacOS is [Homebrew](http://brew.sh/).
Following the [deployment from binary]({{< relref "from-binary.en-us.md" >}}) guide may work,
but is not supported. To install Gitea via `brew`:

```
brew tap gitea/tap https://gitea.com/gitea/homebrew-gitea
brew install gitea
```

## FreeBSD

A FreeBSD port `www/gitea` is available. To install the pre-built binary package:

```
pkg install gitea
```

For the most up to date version, or to build the port with custom options,
[install it from the port](https://www.freebsd.org/doc/handbook/ports-using.html):

```
su -
cd /usr/ports/www/gitea
make install clean
```

The port uses the standard FreeBSD file system layout: config files are in `/usr/local/etc/gitea`,
bundled templates, options, plugins and themes are in `/usr/local/share/gitea`, and a start script
is in `/usr/local/etc/rc.d/gitea`.

To enable Gitea to run as a service, run `sysrc gitea_enable=YES` and start it with `service gitea start`.

## Third-party

Various other third-party packages of Gitea exist.
To see a curated list, head over to [awesome-gitea](https://gitea.com/gitea/awesome-gitea/src/branch/master/README.md#user-content-packages).

Do you know of an existing package that isn't on the list? Send in a PR to get it added!
