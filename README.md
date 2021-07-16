<p align="center">
  <a href="https://gitea.io/">
    <img alt="Gitea" src="https://raw.githubusercontent.com/go-gitea/gitea/main/public/img/gitea.svg" width="220"/>
  </a>
</p>
<h1 align="center">Gitea - Git with a cup of tea</h1>

<p align="center">
  <a href="https://drone.gitea.io/go-gitea/gitea" title="Build Status">
    <img src="https://drone.gitea.io/api/badges/go-gitea/gitea/status.svg?ref=refs/heads/main">
  </a>
  <a href="https://discord.gg/Gitea" title="Join the Discord chat at https://discord.gg/Gitea">
    <img src="https://img.shields.io/discord/322538954119184384.svg">
  </a>
  <a href="https://codecov.io/gh/go-gitea/gitea" title="Codecov">
    <img src="https://codecov.io/gh/go-gitea/gitea/branch/main/graph/badge.svg">
  </a>
  <a href="https://godoc.org/code.gitea.io/gitea" title="Go Report Card">
    <img src="https://goreportcard.com/badge/code.gitea.io/gitea">
  </a>
  <a href="https://godoc.org/code.gitea.io/gitea" title="GoDoc">
    <img src="https://godoc.org/code.gitea.io/gitea?status.svg">
  </a>
  <a href="https://github.com/go-gitea/gitea/releases/latest" title="GitHub release">
    <img src="https://img.shields.io/github/release/go-gitea/gitea.svg">
  </a>
  <a href="https://www.codetriage.com/go-gitea/gitea" title="Help Contribute to Open Source">
    <img src="https://www.codetriage.com/go-gitea/gitea/badges/users.svg">
  </a>
  <a href="https://opencollective.com/gitea" title="Become a backer/sponsor of gitea">
    <img src="https://opencollective.com/gitea/tiers/backers/badge.svg?label=backers&color=brightgreen">
  </a>
  <a href="https://opensource.org/licenses/MIT" title="License: MIT">
    <img src="https://img.shields.io/badge/License-MIT-blue.svg">
  </a>
  <a href="https://crowdin.com/project/gitea" title="Crowdin">
    <img src="https://badges.crowdin.net/gitea/localized.svg">
  </a>
  <a href="https://www.tickgit.com/browse?repo=github.com/go-gitea/gitea" title="TODOs">
    <img src="https://badgen.net/https/api.tickgit.com/badgen/github.com/go-gitea/gitea">
  </a>
  <a href="https://www.bountysource.com/teams/gitea" title="Bountysource">
    <img src="https://img.shields.io/bountysource/team/gitea/activity">
  </a>
</p>

<p align="center">
  <a href="README_ZH.md">View the chinese version of this document</a>
</p>

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
- `make frontend` which requires [Node.js 12.17](https://nodejs.org/en/download/) or greater and Internet connectivity to download npm dependencies.

When building from the official source tarballs which include pre-built frontend files, the `frontend` target will not be triggered, making it possible to build without Node.js and Internet connectivity.

Parallelism (`make -j <num>`) is not supported.

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

## Translating

Translations are done through Crowdin. If you want to translate to a new language ask one of the managers in the Crowdin project to add a new language there. 

You can also just create an issue for adding a language or ask on discord on the #translation channel. If you need context or find some translation issues, you can leave a comment on the string or ask on Discord. For general translation questions there is a section in the docs. Currently a bit empty but we hope fo fill it as questions pop up.

https://docs.gitea.io/en-us/translation-guidelines/

[![Crowdin](https://badges.crowdin.net/gitea/localized.svg)](https://crowdin.com/project/gitea)

## Further information

For more information and instructions about how to install Gitea, please look at our [documentation](https://docs.gitea.io/en-us/).
If you have questions that are not covered by the documentation, you can get in contact with us on our [Discord server](https://discord.gg/Gitea) or create  a post in the [discourse forum](https://discourse.gitea.io/).

We maintain a list of Gitea-related projects at [gitea/awesome-gitea](https://gitea.com/gitea/awesome-gitea).  
The hugo-based documentation theme is hosted at [gitea/theme](https://gitea.com/gitea/theme).  
The official Gitea CLI is developed at [gitea/tea](https://gitea.com/gitea/tea).

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
See the [LICENSE](https://github.com/go-gitea/gitea/blob/main/LICENSE) file
for the full license text.

## Screenshots
Looking for an overview of the interface? Check it out!

|![Dashboard](https://dl.gitea.io/screenshots/home_timeline.png)|![User Profile](https://dl.gitea.io/screenshots/user_profile.png)|![Global Issues](https://dl.gitea.io/screenshots/global_issues.png)|
|:---:|:---:|:---:|
|![Branches](https://dl.gitea.io/screenshots/branches.png)|![Web Editor](https://dl.gitea.io/screenshots/web_editor.png)|![Activity](https://dl.gitea.io/screenshots/activity.png)|
|![New Migration](https://dl.gitea.io/screenshots/migration.png)|![Migrating](https://dl.gitea.io/screenshots/migration.gif)|![Pull Request View](https://image.ibb.co/e02dSb/6.png)
![Pull Request Dark](https://dl.gitea.io/screenshots/pull_requests_dark.png)|![Diff Review Dark](https://dl.gitea.io/screenshots/review_dark.png)|![Diff Dark](https://dl.gitea.io/screenshots/diff_dark.png)|
