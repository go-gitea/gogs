# Contribution Guidelines

## Table of Contents

- [Contribution Guidelines](#contribution-guidelines)
  - [Table of Contents](#table-of-contents)
  - [Introduction](#introduction)
  - [Bug reports](#bug-reports)
  - [Discuss your design](#discuss-your-design)
  - [Testing redux](#testing-redux)
  - [Vendoring](#vendoring)
  - [Translation](#translation)
  - [Building Gitea](#building-gitea)
  - [Code review](#code-review)
  - [Styleguide](#styleguide)
  - [Design guideline](#design-guideline)
  - [API v1](#api-v1)
  - [Developer Certificate of Origin (DCO)](#developer-certificate-of-origin-dco)
  - [Release Cycle](#release-cycle)
  - [Maintainers](#maintainers)
  - [Owners](#owners)
  - [Versions](#versions)
  - [Releasing Gitea](#releasing-gitea)
  - [Copyright](#copyright)

## Introduction

This document explains how to contribute changes to the Gitea project.
It assumes you have followed the
[installation instructions](https://docs.gitea.io/en-us/).
Sensitive security-related issues should be reported to
[security@gitea.io](mailto:security@gitea.io).

For configuring IDE or code editor to develop Gitea see [IDE and code editor configuration](contrib/ide/)

## Bug reports

Please search the issues on the issue tracker with a variety of keywords
to ensure your bug is not already reported.

If unique, [open an issue](https://github.com/go-gitea/gitea/issues/new)
and answer the questions so we can understand and reproduce the
problematic behavior.

To show us that the issue you are having is in Gitea itself, please
write clear, concise instructions so we can reproduce the behavior—
even if it seems obvious. The more detailed and specific you are,
the faster we can fix the issue. Check out [How to Report Bugs
Effectively](http://www.chiark.greenend.org.uk/~sgtatham/bugs.html).

Please be kind, remember that Gitea comes at no cost to you, and you're
getting free help.

## Discuss your design

The project welcomes submissions. If you want to change or add something,
please let everyone know what you're working on—[file an issue](https://github.com/go-gitea/gitea/issues/new)!
Significant changes must go through the change proposal process
before they can be accepted. To create a proposal, file an issue with
your proposed changes documented, and make sure to note in the title
of the issue that it is a proposal.

This process gives everyone a chance to validate the design, helps
prevent duplication of effort, and ensures that the idea fits inside
the goals for the project and tools. It also checks that the design is
sound before code is written; the code review tool is not the place for
high-level discussions.

## Testing redux

Before submitting a pull request, run all the tests for the whole tree
to make sure your changes don't cause regression elsewhere.

Here's how to run the test suite:

- code lint

|                       |                                                                   |
| :-------------------- | :---------------------------------------------------------------- |
|``make lint``          | lint everything (not suggest if you only change one type code)    |
|``make lint-frontend`` | lint frontend files  |
|``make lint-backend``  | lint backend files   |

- run test code (Suggest run in linux)  

|                                        |                                                  |
| :------------------------------------- | :----------------------------------------------- |
|``make test[\#TestSpecificName]``       |  run unit test  |
|``make test-sqlite[\#TestSpecificName]``|  run [integration](integrations) test for sqlite |  
|[More detail message about integrations](integrations/README.md)  |

## Vendoring

We keep a cached copy of dependencies within the `vendor/` directory,
managing updates via [Modules](https://golang.org/cmd/go/#hdr-Module_maintenance).

Pull requests should only include `vendor/` updates if they are part of
the same change, be it a bugfix or a feature addition.

The `vendor/` update needs to be justified as part of the PR description,
and must be verified by the reviewers and/or merger to always reference
an existing upstream commit.

You can find more information on how to get started with it on the [Modules Wiki](https://github.com/golang/go/wiki/Modules).

## Translation

We do all translation work inside [Crowdin](https://crowdin.com/project/gitea).
The only translation that is maintained in this git repository is
[`en_US.ini`](https://github.com/go-gitea/gitea/blob/master/options/locale/locale_en-US.ini)
and is synced regularly to Crowdin. Once a translation has reached
A SATISFACTORY PERCENTAGE it will be synced back into this repo and
included in the next released version.

## Building Gitea

See the [hacking instructions](https://docs.gitea.io/en-us/hacking-on-gitea/).

## Code review

Changes to Gitea must be reviewed before they are accepted—no matter who
makes the change, even if they are an owner or a maintainer. We use GitHub's
pull request workflow to do that. And, we also use [LGTM](http://lgtm.co)
to ensure every PR is reviewed by at least 2 maintainers.

Please try to make your pull request easy to review for us. And, please read
the *[How to get faster PR reviews](https://github.com/kubernetes/community/blob/261cb0fd089b64002c91e8eddceebf032462ccd6/contributors/guide/pull-requests.md#best-practices-for-faster-reviews)* guide;
it has lots of useful tips for any project you may want to contribute.
Some of the key points:

* Make small pull requests. The smaller, the faster to review and the
  more likely it will be merged soon.
* Don't make changes unrelated to your PR. Maybe there are typos on
  some comments, maybe refactoring would be welcome on a function... but
  if that is not related to your PR, please make *another* PR for that.
* Split big pull requests into multiple small ones. An incremental change
  will be faster to review than a huge PR.

## Styleguide

For imports you should use the following format (_without_ the comments)
```go
import (
  // stdlib
  "encoding/json"
  "fmt"

  // local packages
  "code.gitea.io/gitea/models"
  "code.gitea.io/sdk/gitea"

  // external packages
  "github.com/foo/bar"
  "gopkg.io/baz.v1"
)
```

## Design guideline

To maintain understandable code and avoid circular dependencies it is important to have a good structure of the code. The gitea code is divided into the following parts:

- **integration:** Integrations tests
- **models:** Contains the data structures used by xorm to construct database tables. It also contains supporting functions to query and update the database. Dependencies to other code in Gitea should be avoided although some modules might be needed (for example for logging).
- **models/fixtures:** Sample model data used in integration tests.
- **models/migrations:** Handling of database migrations between versions. PRs that changes a database structure shall also have a migration step.
- **modules:** Different modules to handle specific functionality in Gitea.
- **public:** Frontend files (javascript, images, css, etc.)
- **routers:** Handling of server requests. As it uses other Gitea packages to serve the request, other packages (models, modules or services) shall not depend on routers
- **services:** Support functions for common routing operations. Uses models and modules to handle the request.
- **templates:** Golang templates for generating the html output.
- **vendor:** External code that Gitea depends on.

## API v1

The API is documented by [swagger](http://try.gitea.io/api/swagger) and is based on [GitHub API v3](https://developer.github.com/v3/).
Thus, Gitea´s API should use the same endpoints and fields as GitHub´s API as far as possible, unless there are good reasons to deviate.  
If Gitea provides functionality that GitHub does not, a new endpoint can be created.  
If information is provided by Gitea that is not provided by the GitHub API, a new field can be used that doesn't collide with any GitHub fields.

Updating an existing API should not remove existing fields unless there is a really good reason to do so.
The same applies to status responses. If you notice a problem, feel free to leave a comment in the code for future refactoring to APIv2 (which is currently not planned).

All expected results (errors, success, fail messages) should be documented
([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/repo/issue.go#L319-L327)).

All JSON input types must be defined as a struct in [modules/structs/](modules/structs/)
([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/modules/structs/issue.go#L76-L91))
and referenced in
[routers/api/v1/swagger/options.go](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/swagger/options.go).  
They can then be used like the following:
([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/repo/issue.go#L318)).

All JSON responses must be defined as a struct in [modules/structs/](modules/structs/)
([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/modules/structs/issue.go#L36-L68))
and referenced in its category in [routers/api/v1/swagger/](routers/api/v1/swagger/)
([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/swagger/issue.go#L11-L16))  
They can be used like the following:
([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/repo/issue.go#L277-L279))

In general, HTTP methods are chosen as follows:
 * **GET** endpoints return requested object and status **OK (200)**
 * **DELETE** endpoints return status **No Content (204)**
 * **POST** endpoints return status **Created (201)**, used to **create** new objects (e.g. a User)
 * **PUT** endpoints return status **No Content (204)**, used to **add/assign** existing Objects (e.g. User) to something (e.g. Org-Team)
 * **PATCH** endpoints return changed object and status **OK (200)**, used to **edit/change** an existing object


An endpoint which changes/edits an object expects all fields to be optional (except ones to identify the object, which are required).


## Developer Certificate of Origin (DCO)

We consider the act of contributing to the code by submitting a Pull
Request as the "Sign off" or agreement to the certifications and terms
of the [DCO](DCO) and [MIT license](LICENSE). No further action is required.
Additionally you could add a line at the end of your commit message.

```
Signed-off-by: Joe Smith <joe.smith@email.com>
```

If you set your `user.name` and `user.email` git configs, you can add the
line to the end of your commit automatically with `git commit -s`.

We assume in good faith that the information you provide is legally binding.

## Release Cycle

We adopted a release schedule to streamline the process of working
on, finishing, and issuing releases. The overall goal is to make a
minor release every three or four months, which breaks down into two or three months of
general development followed by one month of testing and polishing
known as the release freeze. All the feature pull requests should be
merged before feature freeze. And, during the frozen period, a corresponding 
release branch is open for fixes backported from main branch. Release candidates 
are made during this period for user testing to
obtain a final version that is maintained in this branch. A release is
maintained by issuing patch releases to only correct critical problems
such as crashes or security issues.

Major release cycles are seasonal. They always begin on the 25th and end on
the 24th (i.e., the 25th of December to March 24th).

During a development cycle, we may also publish any necessary minor releases
for the previous version. For example, if the latest, published release is
v1.2, then minor changes for the previous release—e.g., v1.1.0 -> v1.1.1—are
still possible.

## Maintainers

To make sure every PR is checked, we have [team
maintainers](MAINTAINERS). Every PR **MUST** be reviewed by at least
two maintainers (or owners) before it can get merged. A maintainer
should be a contributor of Gitea (or Gogs) and contributed at least
4 accepted PRs. A contributor should apply as a maintainer in the
[Discord](https://discord.gg/NsatcWJ) #develop channel. The owners
or the team maintainers may invite the contributor. A maintainer
should spend some time on code reviews. If a maintainer has no
time to do that, they should apply to leave the maintainers team
and we will give them the honor of being a member of the [advisors
team](https://github.com/orgs/go-gitea/teams/advisors). Of course, if
an advisor has time to code review, we will gladly welcome them back
to the maintainers team. If a maintainer is inactive for more than 3
months and forgets to leave the maintainers team, the owners may move
him or her from the maintainers team to the advisors team.
For security reasons, Maintainers should use 2FA for their accounts and
if possible provide gpg signed commits.
https://help.github.com/articles/securing-your-account-with-two-factor-authentication-2fa/
https://help.github.com/articles/signing-commits-with-gpg/

## Owners

Since Gitea is a pure community organization without any company support,
to keep the development healthy we will elect three owners every year. All
contributors may vote to elect up to three candidates, one of which will
be the main owner, and the other two the assistant owners. When the new
owners have been elected, the old owners will give up ownership to the
newly elected owners. If an owner is unable to do so, the other owners
will assist in ceding ownership to the newly elected owners.
For security reasons, Owners or any account with write access (like a bot)
must use 2FA.
https://help.github.com/articles/securing-your-account-with-two-factor-authentication-2fa/

After the election, the new owners should proactively agree
with our [CONTRIBUTING](CONTRIBUTING.md) requirements in the
[Discord](https://discord.gg/NsatcWJ) #general channel. Below are the
words to speak:

```
I'm honored to having been elected an owner of Gitea, I agree with
[CONTRIBUTING](CONTRIBUTING.md). I will spend part of my time on Gitea
and lead the development of Gitea.
```

To honor the past owners, here's the history of the owners and the time
they served:

* 2021-01-01 ~ 2021-12-31 - https://github.com/go-gitea/gitea/issues/13801
  * [Lunny Xiao](https://gitea.com/lunny) <xiaolunwen@gmail.com>
  * [Lauris Bukšis-Haberkorns](https://gitea.com/lafriks) <lauris@nix.lv>
  * [Matti Ranta](https://gitea.com/techknowlogick) <techknowlogick@gitea.io>

* 2020-01-01 ~ 2020-12-31 - https://github.com/go-gitea/gitea/issues/9230
  * [Lunny Xiao](https://gitea.com/lunny) <xiaolunwen@gmail.com>
  * [Lauris Bukšis-Haberkorns](https://gitea.com/lafriks) <lauris@nix.lv>
  * [Matti Ranta](https://gitea.com/techknowlogick) <techknowlogick@gitea.io>

* 2019-01-01 ~ 2019-12-31 - https://github.com/go-gitea/gitea/issues/5572
  * [Lunny Xiao](https://github.com/lunny) <xiaolunwen@gmail.com>
  * [Lauris Bukšis-Haberkorns](https://github.com/lafriks) <lauris@nix.lv>
  * [Matti Ranta](https://github.com/techknowlogick) <techknowlogick@gitea.io>

* 2018-01-01 ~ 2018-12-31 - https://github.com/go-gitea/gitea/issues/3255
  * [Lunny Xiao](https://github.com/lunny) <xiaolunwen@gmail.com>
  * [Lauris Bukšis-Haberkorns](https://github.com/lafriks) <lauris@nix.lv>
  * [Kim Carlbäcker](https://github.com/bkcsoft) <kim.carlbacker@gmail.com>

* 2016-11-04 ~ 2017-12-31
  * [Lunny Xiao](https://github.com/lunny) <xiaolunwen@gmail.com>
  * [Thomas Boerger](https://github.com/tboerger) <thomas@webhippie.de>
  * [Kim Carlbäcker](https://github.com/bkcsoft) <kim.carlbacker@gmail.com>

## Versions

Gitea has the `master` branch as a tip branch and has version branches
such as `release/v0.9`. `release/v0.9` is a release branch and we will
tag `v0.9.0` for binary download. If `v0.9.0` has bugs, we will accept
pull requests on the `release/v0.9` branch and publish a `v0.9.1` tag,
after bringing the bug fix also to the master branch.

Since the `master` branch is a tip version, if you wish to use Gitea
in production, please download the latest release tag version. All the
branches will be protected via GitHub, all the PRs to every branch must
be reviewed by two maintainers and must pass the automatic tests.

## Releasing Gitea

* Let $vmaj, $vmin and $vpat be Major, Minor and Patch version numbers, $vpat should be rc1, rc2, 0, 1, ...... $vmaj.$vmin will be kept the same as milestones on github or gitea in future.
* Before releasing, confirm all the version's milestone issues or PRs has been resolved. Then discuss the release on discord channel #maintainers and get agreed with almost all the owners and mergers. Or you can declare the version and if nobody against in about serval hours.
* If this is a big version first you have to create PR for changelog on branch `master` with PRs with label `changelog` and after it has been merged do following steps:
  * Create `-dev` tag as `git tag -s -F release.notes v$vmaj.$vmin.0-dev` and push the tag as `git push origin v$vmaj.$vmin.0-dev`.
  * When CI has finished building tag then you have to create a new branch named `release/v$vmaj.$vmin`
* If it is bugfix version create PR for changelog on branch `release/v$vmaj.$vmin` and wait till it is reviewed and merged.
* Add a tag as `git tag -s -F release.notes v$vmaj.$vmin.$`, release.notes file could be a temporary file to only include the changelog this version which you added to `CHANGELOG.md`.
* And then push the tag as `git push origin v$vmaj.$vmin.$`. Drone CI will automatically created a release and upload all the compiled binary. (But currently it didn't add the release notes automatically. Maybe we should fix that.)
* If needed send PR for changelog on branch `master`.
* Send PR to [blog repository](https://gitea.com/gitea/blog) announcing the release.

## Copyright

Code that you contribute should use the standard copyright header:

```
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
```

Files in the repository contain copyright from the year they are added
to the year they are last changed. If the copyright author is changed,
just paste the header below the old one.
