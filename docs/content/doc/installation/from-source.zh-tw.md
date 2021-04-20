---
date: "2016-12-01T16:00:00+02:00"
title: "原始碼安裝"
slug: "install-from-source"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "原始碼安裝"
    weight: 30
    identifier: "install-from-source"
---

# 從原始碼安裝

我們不會在本文教大家如何安裝 Golang 環境。假如您不知道如何設定環境，請直接參考[官方安裝文件](https://golang.org/doc/install)。

## 下載

首先您必須先下載原始碼，最簡單的方式就是透過 Go 指令下載，請透過底下指令下載原始碼並且切換到工作目錄。

```
go get -d -u code.gitea.io/gitea
cd $GOPATH/src/code.gitea.io/gitea
```

現在該決定您要編譯或安裝的 Gitea 版本，您有很多可以選擇。如果您想編譯 `master` 版本，你可以直接跳到[編譯章節](#build)，這是我們開發分支，雖然很穩定，但是不建議用在正式環境。

假如您想要編譯最新穩定版本，可以執行底下命令切換到正確版本:

```
git branch -a
git checkout v{{< version >}}
```

最後您也可以直接編譯最新的標籤版本像是 `v{{< version >}}`，假如您想要從原始碼編譯，這方法是最合適的，在編譯標籤版本前，您需要列出當下所有標籤，並且直接切換到標籤版本，請使用底下指令：:

```
git tag -l
git checkout v{{< version >}}
```

## 編譯

完成設定相依性套件環境等工作後，您就可以開始編譯工作了。我們提供了不同的[編譯選項](https://github.com/go-gitea/gitea/blob/master/Makefile) ，讓編譯過程更加簡單。您可以根據需求來調整編譯選項，底下是可用的編譯選項說明：

* `bindata`: 使用此標籤來嵌入所有 Gitea 相關資源，您不用擔心其他額外檔案，對於部署來說非常方便。
* `pam`: 使用此標籤來啟用 PAM (Linux Pluggable Authentication Modules) 認證，對於系統使用者來說，此方式最方便了。

現在您可以開始編譯執行檔了，我們建議使用 `bindata` 編譯選項:

```
TAGS="bindata" make build
```

**注意**: 因為使用了套件管理工具，我們建議 Go 環境版本為 1.6 或者是更高，這樣不用在 Go 1.5 版本設定全域變數 `GO15VENDOREXPERIMENT`。

## 測試

完成上述步驟後，您可以在當下目錄發現 `gitea` 執行檔，在複製執行檔到遠端環境之前，您必須透過底下指令執行測試，使用 `Ctrl + C` 則可以關閉當下 gitea 程序。

```
./gitea web
```

## 需要協助？

如果本頁中無法解決您的問題，請直接到 [Discord server](https://discord.gg/Gitea)，在那邊可以快速得到協助。
