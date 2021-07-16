---
date: "2017-04-15T14:56:00+02:00"
title: "自定义 Gitea 配置"
slug: "customizing-gitea"
weight: 9
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "自定义 Gitea 配置"
    weight: 9
    identifier: "customizing-gitea"
---

# 自定义 Gitea 配置

Gitea 引用 `custom` 目录中的自定义配置文件来覆盖配置、模板等默认配置。

如果从二进制部署 Gitea ，则所有默认路径都将相对于该 gitea 二进制文件；如果从发行版安装，则可能会将这些路径修改为Linux文件系统标准。Gitea
将会自动创建包括 `custom/` 在内的必要应用目录，应用本身的配置存放在
`custom/conf/app.ini` 当中。在发行版中可能会以 `/etc/gitea/` 的形式为 `custom` 设置一个符号链接，查看配置详情请移步：

- [快速备忘单](https://docs.gitea.io/en-us/config-cheat-sheet/)
- [完整配置清单](https://github.com/go-gitea/gitea/blob/master/custom/conf/app.example.ini)

如果您在 binary 同目录下无法找到 `custom` 文件夹，请检查您的 `GITEA_CUSTOM`
环境变量配置， 因为它可能被配置到了其他地方（可能被一些启动脚本设置指定了目录）。

- [环境变量清单](https://docs.gitea.io/en-us/specific-variables/)

**注：** 必须完全重启 Gitea 以使配置生效。

## 使用自定义 /robots.txt

将 [想要展示的内容](http://www.robotstxt.org/) 存放在 `custom` 目录中的
`robots.txt` 文件来让 Gitea 使用自定义的`/robots.txt` （默认：空 404）。

## 使用自定义的公共文件

将自定义的公共文件（比如页面和图片）作为 webroot 放在 `custom/public/` 中来让 Gitea 提供这些自定义内容（符号链接将被追踪）。

举例说明：`image.png` 存放在 `custom/public/`中，那么它可以通过链接 http://gitea.domain.tld/assets/image.png 访问。

## 修改默认头像

替换以下目录中的 png 图片： `custom/public/img/avatar\_default.png`

## 自定义 Gitea 页面

您可以改变 Gitea `custom/templates` 的每个单页面。您可以在 Gitea 源码的 `templates` 目录中找到用于覆盖的模板文件，应用将根据
`custom/templates` 目录下的路径结构进行匹配和覆盖。

包含在 `{{` 和 `}}` 中的任何语句都是 Gitea 的模板语法，如果您不完全理解这些组件，不建议您对它们进行修改。

### 添加链接和页签

如果您只是想添加额外的链接到顶部导航栏或额外的选项卡到存储库视图，您可以将它们放在您 `custom/templates/custom/` 目录下的 `extra_links.tmpl` 和 `extra_tabs.tmpl` 文件中。

举例说明：假设您需要在网站放置一个静态的“关于”页面，您只需将该页面放在您的
"custom/public/"目录下（比如 `custom/public/impressum.html`）并且将它与 `custom/templates/custom/extra_links.tmpl` 链接起来即可。

这个链接应当使用一个名为“item”的 class 来匹配当前样式，您可以使用 `{{AppSubUrl}}` 来获取 base URL:
`<a class="item" href="{{AppSubUrl}}/assets/impressum.html">Impressum</a>`

同理，您可以将页签添加到 `extra_tabs.tmpl` 中，使用同样的方式来添加页签。它的具体样式需要与
`templates/repo/header.tmpl` 中已有的其他选项卡的样式匹配
([source in GitHub](https://github.com/go-gitea/gitea/blob/master/templates/repo/header.tmpl))

### 页面的其他新增内容

除了 `extra_links.tmpl` 和 `extra_tabs.tmpl`，您可以在您的 `custom/templates/custom/` 目录中存放一些其他有用的模板，例如：

- `header.tmpl`，在 `<head>` 标记结束之前的模板，例如添加自定义CSS文件
- `body_outer_pre.tmpl`，在 `<body>` 标记开始处的模板
- `body_inner_pre.tmpl`，在顶部导航栏之前，但在主 container 内部的模板，例如添加一个 `<div class="full height">`
- `body_inner_post.tmpl`，在主 container 结束处的模板
- `body_outer_post.tmpl`，在底部 `<footer>` 元素之前.
- `footer.tmpl`，在 `<body>` 标签结束处的模板，可以在这里填写一些附加的 Javascript 脚本。

## 自定义 gitignores，labels， licenses， locales 以及 readmes

将自定义文件放在 `custom/options` 下相应子的文件夹中即可

## 更改 Gitea 外观

Gitea 目前由两种内置主题，分别为默认 `gitea` 主题和深色主题 `arc-green`，您可以通过修改
`app.ini` [ui](https://docs.gitea.io/en-us/config-cheat-sheet/#ui-ui) 部分的 `DEFAULT_THEME` 的值来变更至一个可用的 Gitea 外观。
