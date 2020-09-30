---
date: "2019-12-31T13:55:00+05:00"
title: "Продвинутая: Индексация поисковых систем"
slug: "search-engines-indexation"
weight: 30
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Индексация поисковых систем"
    weight: 60
    identifier: "search-engines-indexation"
---

# Индексация вашей установки Gitea поисковыми системами

По умолчанию ваша установка Gitea будет проиндексирована поисковыми системами.
Если вы не хотите, чтобы ваш репозиторий был виден поисковиками, читайте дальше.

## Блокировать индексацию поисковыми системами с помощью robots.txt

Чтобы Gitea обслуживала специальный файл `robots.txt` (по умолчанию: пустой 404) для установок верхнего уровня,
создайте файл с именем` robots.txt` в [папке `custom` или `CustomPath`]({{< relref "doc/advanced/customizing-gitea.ru-ru.md" >}})

Примеры того, как настроить `robots.txt`, можно найти на [https://moz.com/learn/seo/robotstxt](https://moz.com/learn/seo/robotstxt).


```txt
User-agent: *
Disallow: /
```

Если вы установили Gitea в подкаталог, вам нужно будет создать или отредактировать файл `robots.txt` в каталоге верхнего уровня.

```txt
User-agent: *
Disallow: /gitea/
```
