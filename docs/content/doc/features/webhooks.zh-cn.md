---
date: "2016-12-01T16:00:00+02:00"
title: "Webhooks"
slug: "webhooks"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "features"
    name: "Webhooks"
    weight: 30
    identifier: "webhooks"
---

# Webhooks


Gitea 的存储 webhook。这可以有存储库管路设定页 `/:username/:reponame/settings/hooks` 中的。Webhook 也可以按照组织调整或全系统调整，所有时间的推送都是POST请求
。此方法目前被下列服务支援：

- Gitea (也可以是 GET 請求)
- Gogs
- Slack
- Discord
- Dingtalk
- Telegram
- Microsoft Teams
- Feishu
- Wechatwork
## TBD
