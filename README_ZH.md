[English](README.md)
[Русский](README_RU.md)

<h1> <img src="https://raw.githubusercontent.com/go-gitea/gitea/master/public/img/gitea-192.png" alt="logo" width="30" height="30"> Gitea - Git with a cup of tea</h1>

[![Build Status](https://drone.gitea.io/api/badges/go-gitea/gitea/status.svg)](https://drone.gitea.io/go-gitea/gitea)
[![Join the Discord chat at https://discord.gg/Gitea](https://img.shields.io/discord/322538954119184384.svg)](https://discord.gg/Gitea)
[![](https://images.microbadger.com/badges/image/gitea/gitea.svg)](https://microbadger.com/images/gitea/gitea "Get your own image badge on microbadger.com")
[![codecov](https://codecov.io/gh/go-gitea/gitea/branch/master/graph/badge.svg)](https://codecov.io/gh/go-gitea/gitea)
[![Go Report Card](https://goreportcard.com/badge/code.gitea.io/gitea)](https://goreportcard.com/report/code.gitea.io/gitea)
[![GoDoc](https://godoc.org/code.gitea.io/gitea?status.svg)](https://godoc.org/code.gitea.io/gitea)
[![GitHub release](https://img.shields.io/github/release/go-gitea/gitea.svg)](https://github.com/go-gitea/gitea/releases/latest)
[![Help Contribute to Open Source](https://www.codetriage.com/go-gitea/gitea/badges/users.svg)](https://www.codetriage.com/go-gitea/gitea)
[![Become a backer/sponsor of gitea](https://opencollective.com/gitea/tiers/backers/badge.svg?label=backers&color=brightgreen)](https://opencollective.com/gitea)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Crowdin](https://badges.crowdin.net/gitea/localized.svg)](https://crowdin.com/project/gitea)

## 目标

Gitea 的首要目标是创建一个极易安装，运行非常快速，安装和使用体验良好的自建 Git 服务。我们采用 Go 作为后端语言，这使我们只要生成一个可执行程序即可。并且他还支持跨平台，支持 Linux, macOS 和 Windows 以及各种架构，除了 x86，amd64，还包括 ARM 和 PowerPC。

如果您想试用一下，请访问 [在线Demo](https://try.gitea.io/)！

## 提示

1. **开始贡献代码之前请确保你已经看过了 [贡献者向导（英文）](CONTRIBUTING.md)**.
2. 所有的安全问题，请私下发送邮件给 **security@gitea.io**。谢谢！
3. 如果你要使用API，请参见 [API 文档](https://godoc.org/code.gitea.io/sdk/gitea).

## 文档

关于如何安装请访问我们的 [文档站](https://docs.gitea.io/zh-cn/)，如果没有找到对应的文档，你也可以通过 [Discord - 英文](https://discord.gg/gitea) 和 QQ群 328432459 来和我们交流。

## 贡献流程

Fork -> Patch -> Push -> Pull Request

## 作者

* [Maintainers](https://github.com/orgs/go-gitea/people)
* [Contributors](https://github.com/go-gitea/gitea/graphs/contributors)
* [Translators](options/locale/TRANSLATORS)

## 授权许可

本项目采用 MIT 开源授权许可证，完整的授权说明已放置在 [LICENSE](https://github.com/go-gitea/gitea/blob/master/LICENSE) 文件中。

## 截图

|![Dashboard](https://dl.gitea.io/screenshots/home_timeline.png)|![User Profile](https://dl.gitea.io/screenshots/user_profile.png)|![Global Issues](https://dl.gitea.io/screenshots/global_issues.png)|
|:---:|:---:|:---:|
|![Branches](https://dl.gitea.io/screenshots/branches.png)|![Web Editor](https://dl.gitea.io/screenshots/web_editor.png)|![Activity](https://dl.gitea.io/screenshots/activity.png)|
|![New Migration](https://dl.gitea.io/screenshots/migration.png)|![Migrating](https://dl.gitea.io/screenshots/migration.gif)|![Pull Request View](https://image.ibb.co/e02dSb/6.png)
![Pull Request Dark](https://dl.gitea.io/screenshots/pull_requests_dark.png)|![Diff Review Dark](https://dl.gitea.io/screenshots/review_dark.png)|![Diff Dark](https://dl.gitea.io/screenshots/diff_dark.png)|
