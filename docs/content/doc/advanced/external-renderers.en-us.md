---
date: "2018-11-23:00:00+02:00"
title: "External renderers"
slug: "external-renderers"
weight: 40
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "External renderers"
    weight: 40
    identifier: "external-renderers"
---

# Custom files rendering configuration

**Table of Contents**

{{< toc >}}

Gitea supports custom file renderings (i.e., Jupyter notebooks, asciidoc, etc.) through external binaries,
it is just a matter of:

- installing external binaries
- add some configuration to your `app.ini` file
- restart your Gitea instance

This supports rendering of whole files. If you want to render code blocks in markdown you would need to do something with javascript. See some examples on the [Customizing Gitea](../customizing-gitea) page.

## Installing external binaries

In order to get file rendering through external binaries, their associated packages must be installed.
If you're using a Docker image, your `Dockerfile` should contain something along this lines:

```docker
FROM gitea/gitea:{{< version >}}
[...]

COPY custom/app.ini /data/gitea/conf/app.ini
[...]

RUN apk --no-cache add asciidoctor freetype freetype-dev gcc g++ libpng libffi-dev py-pip python3-dev py3-pip py3-pyzmq
# install any other package you need for your external renderers

RUN pip3 install --upgrade pip
RUN pip3 install -U setuptools
RUN pip3 install jupyter docutils
# add above any other python package you may need to install
```

## `app.ini` file configuration

add one `[markup.XXXXX]` section per external renderer on your custom `app.ini`:

```ini
[markup.asciidoc]
ENABLED = true
FILE_EXTENSIONS = .adoc,.asciidoc
RENDER_COMMAND = "asciidoctor -s -a showtitle --out-file=- -"
; Input is not a standard input but a file
IS_INPUT_FILE = false

[markup.jupyter]
ENABLED = true
FILE_EXTENSIONS = .ipynb
RENDER_COMMAND = "jupyter nbconvert --stdin --stdout --to html --template basic"
IS_INPUT_FILE = false

[markup.restructuredtext]
ENABLED = true
FILE_EXTENSIONS = .rst
RENDER_COMMAND = "timeout 30s pandoc +RTS -M512M -RTS -f rst"
IS_INPUT_FILE = false
```

If your external markup relies on additional classes and attributes on the generated HTML elements, you might need to enable custom sanitizer policies. Gitea uses the [`bluemonday`](https://godoc.org/github.com/microcosm-cc/bluemonday) package as our HTML sanitizier. The example below will support [KaTeX](https://katex.org/) output from [`pandoc`](https://pandoc.org/).

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

You must define `ELEMENT` and `ALLOW_ATTR` in each section.

To define multiple entries, add a unique alphanumeric suffix (e.g., `[markup.sanitizer.1]` and `[markup.sanitizer.something]`).

To apply a sanitisation rules only for a specify external renderer they must use the renderer name, e.g. `[markup.sanitizer.asciidoc.rule-1]`, `[markup.sanitizer.<renderer>.rule-1]`.

**Note**: If the rule is defined above the renderer ini section or the name does not match a renderer it is applied to every renderer.

Once your configuration changes have been made, restart Gitea to have changes take effect.

**Note**: Prior to Gitea 1.12 there was a single `markup.sanitiser` section with keys that were redefined for multiple rules, however,
there were significant problems with this method of configuration necessitating configuration through multiple sections.

### Example: Office DOCX

Display Office DOCX files with [`pandoc`](https://pandoc.org/):
```ini
[markup.docx]
ENABLED = true
FILE_EXTENSIONS = .docx
RENDER_COMMAND = "pandoc --from docx --to html --self-contained --template /path/to/basic.html"

[markup.sanitizer.docx.img]
ALLOW_DATA_URI_IMAGES = true
```

The template file has the following content:
```
$body$
```

### Example: Jupyter Notebook

Display Jupyter Notebook files with [`nbconvert`](https://github.com/jupyter/nbconvert):
```ini
[markup.jupyter]
ENABLED = true
FILE_EXTENSIONS = .ipynb
RENDER_COMMAND = "jupyter-nbconvert --stdin --stdout --to html --template basic"

[markup.sanitizer.jupyter.img]
ALLOW_DATA_URI_IMAGES = true
```

## Customizing CSS
The external renderer is specified in the .ini in the format `[markup.XXXXX]` and the HTML supplied by your external renderer will be wrapped in a `<div>` with classes `markup` and `XXXXX`. The `markup` class provides out of the box styling (as does `markdown` if `XXXXX` is `markdown`). Otherwise you can use these classes to specifically target the contents of your rendered HTML. 

And so you could write some CSS:
```css
.markup.XXXXX html {
  font-size: 100%;
  overflow-y: scroll;
  -webkit-text-size-adjust: 100%;
  -ms-text-size-adjust: 100%;
}

.markup.XXXXX body {
  color: #444;
  font-family: Georgia, Palatino, 'Palatino Linotype', Times, 'Times New Roman', serif;
  font-size: 12px;
  line-height: 1.7;
  padding: 1em;
  margin: auto;
  max-width: 42em;
  background: #fefefe;
}

.markup.XXXXX p {
  color: orangered;
}
```

Add your stylesheet to your custom directory e.g `custom/public/css/my-style-XXXXX.css` and import it using a custom header file `custom/templates/custom/header.tmpl`:
```html
<link type="text/css" href="{{AppSubUrl}}/assets/css/my-style-XXXXX.css" />
```
