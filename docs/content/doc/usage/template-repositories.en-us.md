---
date: "2019-11-28:00:00+02:00"
title: "Template Repositories"
slug: "template-repositories"
weight: 14
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Template Repositories"
    weight: 14
    identifier: "template-repositories"
---

## Template Repositories
Gitea `1.11.0` and above includes template repositories, and one feature implemented with them is auto-expansion of specific variables within your template files.  
To tell Gitea which files to expand, you must include a `template` file inside the `.gitea` directory of the template repository.  
Gitea uses [gobwas/glob](https://github.com/gobwas/glob) for its glob syntax. It closely resembles a traditional `.gitignore`, however there may be slight differences.

### Example `.gitea/template` file  
All paths are relative to the base of the repository
```gitignore
# All .go files, anywhere in the repository
**.go

# All text files in the text directory
text/*.txt

# A specific file
a/b/c/d.json

# Batch files in both upper or lower case can be matched
**.[bB][aA][tT]
```
**NOTE:** The `template` file will be removed from the `.gitea` directory when a repository is generated from the template.

### Variable Expansion
In any file matched by the above globs, certain variables will be expanded.  
All variables must be of the form `$VAR` or `${VAR}`. To escape an expansion, use a double `$$`, such as `$$VAR` or `$${VAR}`

| Variable             | Expands To                                          | Transformable |
|----------------------|-----------------------------------------------------|---------------|
| REPO_NAME            | The name of the generated repository                | ✓             |
| TEMPLATE_NAME        | The name of the template repository                 | ✓             |
| REPO_DESCRIPTION     | The description of the generated repository         | ✘             |
| TEMPLATE_DESCRIPTION | The description of the template repository          | ✘             |
| REPO_OWNER           | The owner of the generated repository               | ✓             |
| TEMPLATE_OWNER       | The owner of the template repository                | ✓             |
| REPO_LINK            | The URL to the generated repository                 | ✘             |
| TEMPLATE_LINK        | The URL to the template repository                  | ✘             |
| REPO_HTTPS_URL       | The HTTP(S) clone link for the generated repository | ✘             |
| TEMPLATE_HTTPS_URL   | The HTTP(S) clone link for the template repository  | ✘             |
| REPO_SSH_URL         | The SSH clone link for the generated repository     | ✘             |
| TEMPLATE_SSH_URL     | The SSH clone link for the template repository      | ✘             |

### Transformers :robot:
Gitea `1.12.0` adds a few transformers to some of the applicable variables above.  
For example, to get `REPO_NAME` in `PASCAL`-case, your template would use `${REPO_NAME_PASCAL}`

Feeding `go-sdk` to the available transformers yields...

| Transformer | Effect     |
|-------------|------------|
| SNAKE       | go_sdk     |
| KEBAB       | go-sdk     |
| CAMEL       | goSdk      |
| PASCAL      | GoSdk      |
| LOWER       | go-sdk     |
| UPPER       | GO-SDK     |
| TITLE       | Go-Sdk     |

