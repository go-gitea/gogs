---
date: "2019-12-28"
title: "Adding Legal Pages"
slug: adding-legal-pages
weight: 9
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Adding Legal Pages"
    identifier: "adding-legal-pages"
    weight: 9
---

Some jurisdictions (such as EU), requires certain legal pages (e.g. Privacy Policy) to be added to website. Follow these steps to add them to your Gitea instance.

## Getting Pages

Gitea source code ships with sample pages, available in `contrib/legal` directory. Copy them to `custom/public/`. For example, to add Privacy Policy:

```
wget -O /path/to/custom/public/privacy.html https://raw.githubusercontent.com/go-gitea/gitea/master/contrib/legal/privacy.html.sample
```

Now you need to edit the page to meet your requirements. In particular you must change the email addresses, web addresses and references to "Your Gitea Instance" to match your situation.

You absolutely must not place a general ToS or privacy statement that implies that the gitea project is responsible for your server.

## Make it Visible

Create or append to `/path/to/custom/templates/custom/extra_links_footer.tmpl`:

```go
<a class="item" href="{{AppSubUrl}}/assets/privacy.html">Privacy Policy</a>
```

Restart Gitea to see the changes.
