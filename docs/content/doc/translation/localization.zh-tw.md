---
date: "2016-12-01T16:00:00+02:00"
title: "在地化"
slug: "localization"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "translation"
    name: "在地化"
    weight: 20
    identifier: "localization"
---

# 在地化

我們在 [Crowdin 專案](https://crowdin.com/project/gitea)上進行在地化工作。

**英語系**的翻譯，可在修改[英文語言檔](https://github.com/go-gitea/gitea/blob/master/options/locale/locale_en-US.ini)後提出合併請求。

**非英語系**的翻譯，請前往上述的 Crowdin 專案。

## 已支援的語言

上述 Crowdin 專案中列出的語言在翻譯超過 25% 後將被支援。

翻譯被認可後將在下次 Crowdin 同步後進入到主儲存庫，通常是在任何合併請求被合併之後。  
這表示更改的翻譯要到下次 Gitea 發佈後才會出現。  
如果您使用的是最新建置，它將會在同步完成、您更新後出現。
