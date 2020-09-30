[English](README.md)
[Русский](README_RU.md)

# Gitea: 文档

[![Build Status](http://drone.gitea.io/api/badges/go-gitea/docs/status.svg)](http://drone.gitea.io/go-gitea/docs)
[![Join the chat at https://img.shields.io/discord/322538954119184384.svg](https://img.shields.io/discord/322538954119184384.svg)](https://discord.gg/Gitea)
[![](https://images.microbadger.com/badges/image/gitea/docs.svg)](http://microbadger.com/images/gitea/docs "Get your own image badge on microbadger.com")

## 关于托管方式

本页面托管在我们 Docker 容器内的基础设施上， 它会在每次推送到 `master` 分支的时候自动更新，如果你想自己管理这个页面，你可以从我们的 Docker 镜像 [gitea/docs](https://hub.docker.com/r/gitea/docs/) 中获取它。

## 安装 Hugo

本页面使用了 [Hugo](https://github.com/spf13/hugo) 静态页面生成工具，如果您有维护它的意愿，则需要在本地计算机上下载并安装 Hugo。Hugo 的安装教程不在本文档的讲述范围之内，详情请参见 [官方文档](https://gohugo.io/overview/installing/)。

## 如何部署

在 [localhost:1313](http://localhost:1313) 处构建和运行网站的命令如下，如果需要停止可以使用组合键 `Ctrl+C`:

```
make server
```

完成更改后，只需创建一个 Pull Request (PR)，该 PR 一经合并网站将自动更新。

## 如何贡献您的代码

Fork -> Patch -> Push -> Pull Request

## 关于我们

* [维护者信息](https://github.com/orgs/go-gitea/people)
* [代码贡献者信息](https://github.com/go-gitea/docs/graphs/contributors)

## 许可证

此项目采用 Apache-2.0 许可协议，请参见 [协议文件](LICENSE) 获取更多信息。

## 版权声明

```
Copyright (c) 2016 The Gitea Authors <https://gitea.io>
```
