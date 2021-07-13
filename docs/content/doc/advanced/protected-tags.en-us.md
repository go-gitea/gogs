---
date: "2021-05-14T00:00:00-00:00"
title: "Protected tags"
slug: "protected-tags"
weight: 45
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Protected tags"
    weight: 45
    identifier: "protected-tags"
---

# Protected tags

Protected tags allow control over who has permission to create or update git tags. Each rule allows you to match either an individual tag name, or use an appropriate pattern to control multiple tags at once. 

**Table of Contents**

{{< toc >}}

## Setting up protected tags

To protect a tag, you need to follow these steps:

1. Go to the repository’s **Settings** > **Tags** page.
1. Type a pattern to match a name. You can use a single name, a [glob pattern](https://pkg.go.dev/github.com/gobwas/glob#Compile) or a regular expression.
1. Choose the allowed users and/or teams. If you leave these fields empty no one is allowed to create or modify this tag.
1. Select **Save** to save the configuration.

## Pattern protected tags

The pattern uses [glob](https://pkg.go.dev/github.com/gobwas/glob#Compile) or regular expressions to match a tag name. For regular expressions you need to enclose the pattern in slashes.

Examples:

| Type  | Pattern Protected Tag    | Possible Matching Tags                  |
| ----- | ------------------------ | --------------------------------------- |
| Glob  | `v*`                     | `v`, `v-1`, `version2`                  |
| Glob  | `v[0-9]`                 | `v0`, `v1` up to `v9`                   |
| Glob  | `*-release`              | `2.1-release`, `final-release`          |
| Glob  | `gitea`                  | only `gitea`                            |
| Glob  | `*gitea*`                | `gitea`, `2.1-gitea`, `1_gitea-release` |
| Glob  | `{v,rel}-*`              | `v-`, `v-1`, `v-final`, `rel-`, `rel-x` |
| Glob  | `*`                      | matches all possible tag names          |
| Regex | `/\Av/`                  | `v`, `v-1`, `version2`                  |
| Regex | `/\Av[0-9]\z/`           | `v0`, `v1` up to `v9`                   |
| Regex | `/\Av\d+\.\d+\.\d+\z/`   | `v1.0.17`, `v2.1.0`                     |
| Regex | `/\Av\d+(\.\d+){0,2}\z/` | `v1`, `v2.1`, `v1.2.34`                 |
| Regex | `/-release\z/`           | `2.1-release`, `final-release`          |
| Regex | `/gitea/`                | `gitea`, `2.1-gitea`, `1_gitea-release` |
| Regex | `/\Agitea\z/`            | only `gitea`                            |
| Regex | `/^gitea$/`              | only `gitea`                            |
| Regex | `/\A(v\|rel)-/`          | `v-`, `v-1`, `v-final`, `rel-`, `rel-x` |
| Regex | `/.+/`                   | matches all possible tag names          |
