---
date: "2018-11-23:00:00+02:00"
title: "Внешние рендеры"
slug: "external-renderers"
weight: 40
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Внешние рендеры"
    weight: 40
    identifier: "external-renderers"
---

# Конфигурация рендеринга пользовательских файлов

Gitea поддерживает пользовательский рендеринг файлов (например, записные книжки Jupyter,
asciidoc и т.д.) Через внешние двоичные файлы, это просто вопрос:

* установка внешних двоичных файлов
* добавьте конфигурацию в ваш файл `app.ini`
* перезапустите свой экземпляр Gitea

Это поддерживает рендеринг целых файлов. Если вы хотите отображать блоки кода в уценке, вам нужно будет что-то сделать с javascript. См. Несколько примеров на странице [Customizing Gitea](../customizing-gitea).

## Установка внешних двоичных файлов

Чтобы получить рендеринг файлов через внешние двоичные файлы, необходимо установить связанные с ними пакеты. 
Если вы используете образ Docker, ваш файл `Dockerfile` должен содержать что-то вроде этих строк:

```
FROM gitea/gitea:{{< version >}}
[...]

COPY custom/app.ini /data/gitea/conf/app.ini
[...]

RUN apk --no-cache add asciidoctor freetype freetype-dev gcc g++ libpng libffi-dev python-dev py-pip python3-dev py3-pip py3-pyzmq
# install any other package you need for your external renderers

RUN pip3 install --upgrade pip
RUN pip3 install -U setuptools
RUN pip3 install jupyter docutils 
# add above any other python package you may need to install
```

## конфигурация файла `app.ini`

добавьте по одному разделу `[markup.XXXXX]` для каждого внешнего средства визуализации в свой пользовательский файл `app.ini`:

```
[markup.asciidoc]
ENABLED = true
FILE_EXTENSIONS = .adoc,.asciidoc
RENDER_COMMAND = "asciidoctor -s -a showtitle --out-file=- -"
; Input is not a standard input but a file
IS_INPUT_FILE = false

[markup.jupyter]
ENABLED = true
FILE_EXTENSIONS = .ipynb
RENDER_COMMAND = "jupyter nbconvert --stdout --to html --template basic "
IS_INPUT_FILE = true

[markup.restructuredtext]
ENABLED = true
FILE_EXTENSIONS = .rst
RENDER_COMMAND = rst2html.py
IS_INPUT_FILE = false
```

Если ваша внешняя разметка полагается на дополнительные классы и атрибуты в сгенерированных элементах HTML, вам может потребоваться включить настраиваемые политики дезинфекции. Gitea использует пакет [`bluemonday`] (https://godoc.org/github.com/microcosm-cc/bluemonday) в качестве средства очистки HTML. В приведённом ниже примере будет поддерживаться вывод [KaTeX] (https://katex.org/) из [`pandoc`](https://pandoc.org/).

```ini
[markup.sanitizer.TeX]
; Pandoc renders TeX segments as <span>s with the "math" class, optionally
; with "inline" or "display" classes depending on context.
ELEMENT = span
ALLOW_ATTR = class
REGEXP = ^\s*((math(\s+|$)|inline(\s+|$)|display(\s+|$)))+

[markup.markdown]
ENABLED         = true
FILE_EXTENSIONS = .md,.markdown
RENDER_COMMAND  = pandoc -f markdown -t html --katex
```

Вы должны определить `ELEMENT`, `ALLOW_ATTR` и `REGEXP` в каждом разделе.

Чтобы определить несколько записей, добавьте уникальный буквенно-цифровой суффикс (например, `[markup.sanitizer.1]` и `[markup.sanitizer.something]`).

После внесения изменений в конфигурацию перезапустите Gitea, чтобы изменения вступили в силу.

**Примечание**: До Gitea 1.12 существовал единственный раздел `markup.sanitiser` с ключами, которые были переопределены для нескольких правил, однако
при этом методе настройки возникали серьёзные проблемы, требовавшие настройки через несколько разделов.
