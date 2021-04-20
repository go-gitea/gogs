---
date: "2016-12-01T16:00:00+02:00"
title: "Installation from source"
slug: "install-from-source"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "From source"
    weight: 30
    identifier: "install-from-source"
---

# Installation from source

You should [install go](https://golang.org/doc/install) and set up your go
environment correctly. In particular, it is recommended to set the `$GOPATH`
environment variable and to add the go bin directory or directories
`${GOPATH//://bin:}/bin` to the `$PATH`. See the Go wiki entry for
[GOPATH](https://github.com/golang/go/wiki/GOPATH).

Next, [install Node.js with npm](https://nodejs.org/en/download/) which is
required to build the JavaScript and CSS files. The minimum supported Node.js
version is {{< min-node-version >}} and the latest LTS version is recommended.

**Note**: When executing make tasks that require external tools, like
`make misspell-check`, Gitea will automatically download and build these as
necessary. To be able to use these, you must have the `"$GOPATH/bin"` directory
on the executable path. If you don't add the go bin directory to the
executable path, you will have to manage this yourself.

**Note 2**: Go version {{< min-go-version >}} or higher is required. However, it is recommended to
obtain the same version as our continuous integration, see the advice given in
<a href='{{< relref "doc/developers/hacking-on-gitea.en-us.md" >}}'>Hacking on
Gitea</a>

**Table of Contents**

{{< toc >}}

## Download

First, we must retrieve the source code. Since, the advent of go modules, the
simplest way of doing this is to use git directly as we no longer have to have
gitea built from within the GOPATH.

```bash
git clone https://github.com/go-gitea/gitea
```

(Previous versions of this document recommended using `go get`. This is
no longer necessary.)

Decide which version of Gitea to build and install. Currently, there are
multiple options to choose from. The `master` branch represents the current
development version. To build with master, skip to the [build section](#build).

To work with tagged releases, the following commands can be used:

```bash
git branch -a
git checkout v{{< version >}}
```

To validate a Pull Request, first enable the new branch (`xyz` is the PR id;
for example `2663` for [#2663](https://github.com/go-gitea/gitea/pull/2663)):

```bash
git fetch origin pull/xyz/head:pr-xyz
```

To build Gitea from source at a specific tagged release (like v{{< version >}}), list the
available tags and check out the specific tag.

List available tags with the following.

```bash
git tag -l
git checkout v{{< version >}}  # or git checkout pr-xyz
```

## Build

To build from source, the following programs must be present on the system:

- `go` {{< min-go-version >}} or higher, see [here](https://golang.org/dl/)
- `node` {{< min-node-version >}} or higher with `npm`, see [here](https://nodejs.org/en/download/)
- `make`, see <a href='{{< relref "doc/developers/hacking-on-gitea.en-us.md" >}}#installing-make'>here</a>

Various [make tasks](https://github.com/go-gitea/gitea/blob/master/Makefile)
are provided to keep the build process as simple as possible.

Depending on requirements, the following build tags can be included.

- `bindata`: Build a single monolithic binary, with all assets included.
- `pam`: Enable support for PAM (Linux Pluggable Authentication Modules). Can
  be used to authenticate local users or extend authentication to methods
  available to PAM.
* `gogit`: (EXPERIMENTAL) Use go-git variants of git commands.

Bundling assets into the binary using the `bindata` build tag is recommended for
production deployments. It is possible to serve the static assets directly via a reverse proxy,
but in most cases it is not necessary, and assets should still be bundled in the binary.
You may want to exclude bindata while developing/testing Gitea.
To include assets, add the `bindata` tag:

```bash
TAGS="bindata" make build
```

In the default release build of our continuous integration system, the build
tags are: `TAGS="bindata"`. The simplest
recommended way to build from source is therefore:

```bash
TAGS="bindata" make build
```

The `build` target is split into two sub-targets:

- `make backend` which requires [Go {{< min-go-version >}}](https://golang.org/dl/) or greater.
- `make frontend` which requires [Node.js {{< min-node-version >}}](https://nodejs.org/en/download/) or greater.

If pre-built frontend files are present it is possible to only build the backend:

```bash
TAGS="bindata" make backend
```

## Test

After following the steps above, a `gitea` binary will be available in the working directory.
It can be tested from this directory or moved to a directory with test data. When Gitea is
launched manually from command line, it can be killed by pressing `Ctrl + C`.

```bash
./gitea web
```

## Changing default paths

Gitea will search for a number of things from the `CustomPath`. By default this is
the `custom/` directory in the current working directory when running Gitea. It will also
look for its configuration file `CustomConf` in `$CustomPath/conf/app.ini`, and will use the
current working directory as the relative base path `AppWorkPath` for a number configurable
values. Finally the static files will be served from `StaticRootPath` which defaults to the `AppWorkPath`.

These values, although useful when developing, may conflict with downstream users preferences.

One option is to use a script file to shadow the `gitea` binary and create an appropriate
environment before running Gitea. However, when building you can change these defaults
using the `LDFLAGS` environment variable for `make`. The appropriate settings are as follows

- To set the `CustomPath` use `LDFLAGS="-X \"code.gitea.io/gitea/modules/setting.CustomPath=custom-path\""`
- For `CustomConf` you should use `-X \"code.gitea.io/gitea/modules/setting.CustomConf=conf.ini\"`
- For `AppWorkPath` you should use `-X \"code.gitea.io/gitea/modules/setting.AppWorkPath=working-path\"`
- For `StaticRootPath` you should use `-X \"code.gitea.io/gitea/modules/setting.StaticRootPath=static-root-path\"`
- To change the default PID file location use `-X \"code.gitea.io/gitea/modules/setting.PIDFile=/run/gitea.pid\"`

Add as many of the strings with their preceding `-X` to the `LDFLAGS` variable and run `make build`
with the appropriate `TAGS` as above.

Running `gitea help` will allow you to review what the computed settings will be for your `gitea`.

## Cross Build

The `go` compiler toolchain supports cross-compiling to different architecture targets that are supported by the toolchain. See [`GOOS` and `GOARCH` environment variable](https://golang.org/doc/install/source#environment) for the list of supported targets. Cross compilation is helpful if you want to build Gitea for less-powerful systems (such as Raspberry Pi).

To cross build Gitea with build tags (`TAGS`), you also need a C cross compiler which targets the same architecture as selected by the `GOOS` and `GOARCH` variables. For example, to cross build for Linux ARM64 (`GOOS=linux` and `GOARCH=arm64`), you need the `aarch64-unknown-linux-gnu-gcc` cross compiler. This is required because Gitea build tags uses `cgo`'s foreign-function interface (FFI).

Cross-build Gitea for Linux ARM64, without any tags:

```
GOOS=linux GOARCH=arm64 make build
```

Cross-build Gitea for Linux ARM64, with recommended build tags:

```
CC=aarch64-unknown-linux-gnu-gcc GOOS=linux GOARCH=arm64 TAGS="bindata" make build
```

Replace `CC`, `GOOS`, and `GOARCH` as appropriate for your architecture target.

You will sometimes need to build a static compiled image. To do this you will need to add:

```
LDFLAGS="-linkmode external -extldflags '-static' $LDFLAGS" TAGS="netgo osusergo $TAGS" make build
```

This can be combined with `CC`, `GOOS`, and `GOARCH` as above.
