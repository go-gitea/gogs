---
date: "2016-12-01T16:00:00+02:00"
title: "从源代码安装"
slug: "install-from-source"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "从源代码安装"
    weight: 30
    identifier: "install-from-source"
---

# 从源代码安装

首先你需要安装Golang，关于Golang的安装，参见官方文档 [install instructions](https://golang.org/doc/install)。

## 下载

你需要获取Gitea的源码，最方便的方式是使用 go 命令。执行以下命令：

```
go get -d -u code.gitea.io/gitea
cd $GOPATH/src/code.gitea.io/gitea
```

然后你可以选择编译和安装的版本，当前你有多个选择。如果你想编译 `master` 版本，你可以直接跳到 [编译](#build) 部分，这是我们的开发分支，虽然也很稳定但不建议您在正式产品中使用。

如果你想编译最新稳定分支，你可以执行以下命令签出源码：

```
git branch -a
git checkout v{{< version >}}
```

最后，你也可以直接使用标签版本如 `v{{< version >}}`。你可以执行以下命令列出可用的版本并选择某个版本签出：

```
git tag -l
git checkout v{{< version >}}
```

## 编译

要从源代码进行编译，以下依赖程序必须事先安装好：

- `go` {{< min-go-version >}} 或以上版本, 详见 [here](https://golang.org/dl/)
- `node` {{< min-node-version >}} 或以上版本，并且安装 `npm`, 详见 [here](https://nodejs.org/en/download/)
- `make`, 详见 <a href='{{< relref "make.zh-cn.md" >}}'>这里</a>

各种可用的 [make 任务](https://github.com/go-gitea/gitea/blob/master/Makefile)
可以用来使编译过程更方便。

按照您的编译需求，以下 tags 可以使用：

* `bindata`: 这个编译选项将会把运行Gitea所需的所有外部资源都打包到可执行文件中，这样部署将非常简单因为除了可执行程序将不再需要任何其他文件。
* `pam`: 这个编译选项将会启用 PAM (Linux Pluggable Authentication Modules) 认证，如果你使用这一认证模式的话需要开启这个选项。

使用 bindata 可以打包资源文件到二进制可以使开发和测试更容易，你可以根据自己的需求决定是否打包资源文件。
要包含资源文件，请使用 `bindata` tag：

```bash
TAGS="bindata" make build
```

默认的发布版本中的编译选项是： `TAGS="bindata"`。以下为推荐的编译方式：

```bash
TAGS="bindata" make build
```

## 测试

在执行了以上步骤之后，你将会获得 `gitea` 的二进制文件，在你复制到部署的机器之前可以先测试一下。在命令行执行完后，你可以 `Ctrl + C` 关掉程序。

```
./gitea web
```

## 需要帮助?

如果从本页中没有找到你需要的内容，请访问 [帮助页面]({{< relref "seek-help.zh-cn.md" >}})
