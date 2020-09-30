[简体中文](README_ZH.md)
[Русский](README_RU.md)

<h1> <img src="https://raw.githubusercontent.com/go-gitea/gitea/master/public/img/gitea-192.png" alt="logo" width="30" height="30"> Gitea - Git with a cup of tea</h1>

[![Build Status](https://drone.gitea.io/api/badges/go-gitea/gitea/status.svg?ref=refs/heads/master)](https://drone.gitea.io/go-gitea/gitea)
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
[![TODOs](https://badgen.net/https/api.tickgit.com/badgen/github.com/go-gitea/gitea)](https://www.tickgit.com/browse?repo=github.com/go-gitea/gitea)

## Purpose

The goal of this project is to make the easiest, fastest, and most
painless way of setting up a self-hosted Git service.
Using Go, this can be done with an independent binary distribution across
**all platforms** which Go supports, including Linux, macOS, and Windows
on x86, amd64, ARM and PowerPC architectures.
Want to try it before doing anything else?
Do it [with the online demo](https://try.gitea.io/)!
This project has been
[forked](https://blog.gitea.io/2016/12/welcome-to-gitea/) from
[Gogs](https://gogs.io) since 2016.11 but changed a lot.

## Building

From the root of the source tree, run:

    TAGS="bindata" make build

or if sqlite support is required:

    TAGS="bindata sqlite sqlite_unlock_notify" make build

The `build` target is split into two sub-targets:

- `make backend` which requires [Go 1.13](https://golang.org/dl/) or greater.
- `make frontend` which requires [Node.js 10.13](https://nodejs.org/en/download/) or greater.

If pre-built frontend files are present it is possible to only build the backend:

    TAGS="bindata" make backend

Parallelism is not supported for these targets, so please don't include `-j <num>`.

More info: https://docs.gitea.io/en-us/install-from-source/

## Using

    ./gitea web

NOTE: If you're interested in using our APIs, we have experimental
support with [documentation](https://try.gitea.io/api/swagger).

## Contributing

Expected workflow is: Fork -> Patch -> Push -> Pull Request

NOTES:

1. **YOU MUST READ THE [CONTRIBUTORS GUIDE](CONTRIBUTING.md) BEFORE STARTING TO WORK ON A PULL REQUEST.**
2. If you have found a vulnerability in the project, please write privately to **security@gitea.io**. Thanks!

## Further information

For more information and instructions about how to install Gitea, please look
at our [documentation](https://docs.gitea.io/en-us/). If you have questions
that are not covered by the documentation, you can get in contact with us on
our [Discord server](https://discord.gg/Gitea),
or [forum](https://discourse.gitea.io/)!

## Authors

* [Maintainers](https://github.com/orgs/go-gitea/people)
* [Contributors](https://github.com/go-gitea/gitea/graphs/contributors)
* [Translators](options/locale/TRANSLATORS)

## Backers

Thank you to all our backers! 🙏 [[Become a backer](https://opencollective.com/gitea#backer)]

<a href="https://opencollective.com/gitea#backers" target="_blank"><img src="https://opencollective.com/gitea/backers.svg?width=890"></a>

## Sponsors

Support this project by becoming a sponsor. Your logo will show up here with a link to your website. [[Become a sponsor](https://opencollective.com/gitea#sponsor)]

<a href="https://opencollective.com/gitea/sponsor/0/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/0/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/1/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/1/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/2/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/2/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/3/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/3/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/4/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/4/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/5/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/5/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/6/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/6/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/7/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/7/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/8/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/8/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/9/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/9/avatar.svg"></a>

## FAQ

**How do you pronounce Gitea?**

Gitea is pronounced [/ɡɪ’ti:/](https://youtu.be/EM71-2uDAoY) as in "gi-tea" with a hard g.

**Why is this not hosted on a Gitea instance?**

We're [working on it](https://github.com/go-gitea/gitea/issues/1029).

## License

This project is licensed under the MIT License.
See the [LICENSE](https://github.com/go-gitea/gitea/blob/master/LICENSE) file
for the full license text.

## Screenshots
Looking for an overview of the interface? Check it out!

|![Dashboard](https://dl.gitea.io/screenshots/home_timeline.png)|![User Profile](https://dl.gitea.io/screenshots/user_profile.png)|![Global Issues](https://dl.gitea.io/screenshots/global_issues.png)|
|:---:|:---:|:---:|
|![Branches](https://dl.gitea.io/screenshots/branches.png)|![Web Editor](https://dl.gitea.io/screenshots/web_editor.png)|![Activity](https://dl.gitea.io/screenshots/activity.png)|
|![New Migration](https://dl.gitea.io/screenshots/migration.png)|![Migrating](https://dl.gitea.io/screenshots/migration.gif)|![Pull Request View](https://image.ibb.co/e02dSb/6.png)
![Pull Request Dark](https://dl.gitea.io/screenshots/pull_requests_dark.png)|![Diff Review Dark](https://dl.gitea.io/screenshots/review_dark.png)|![Diff Dark](https://dl.gitea.io/screenshots/diff_dark.png)|
