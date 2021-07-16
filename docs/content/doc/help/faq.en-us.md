---
date: "2019-04-05T16:00:00+02:00"
title: "FAQ"
slug: "faq"
weight: 5
toc: false
draft: false
menu:
  sidebar:
    parent: "help"
    name: "FAQ"
    weight: 5
    identifier: "faq"
---

# Frequently Asked Questions <!-- omit in toc -->

This page contains some common questions and answers.  
For more help resources, check all [Support Options]({{< relref "doc/help/seek-help.en-us.md" >}}).

**Table of Contents**

{{< toc >}}

## Configuration

### Where does Gitea store what file

- WorkPath
  - Environment variable `GITEA_WORK_DIR`
  - Else binary location
- AppDataPath (default for database, indexers, etc.)
  - `APP_DATA_PATH` from `app.ini`
  - Else `%(WorkPath)/data`
- CustomPath (custom templates)
  - Environment variable `GITEA_CUSTOM`
  - Else `%(WorkPath)/custom`
- HomeDir
  - Unix: Environment variable `HOME`
  - Windows: Environment variable `USERPROFILE`, else environment variables `HOMEDRIVE`+`HOMEPATH`
- RepoRootPath
  - `ROOT` in `app.ini`
  - Else `%(AppDataPath)/gitea-repositories`
- INI (config file)
  - `-c` flag
  - Else `%(CustomPath)/conf/app.ini`
- SQLite Database
  - `PATH` in `database` section of `app.ini`
  - Else `%(AppDataPath)/gitea.db`

### SSHD vs built-in SSH

`sshd` is the SSH server on most Unix systems.
It is by Gitea by default, by placing users' SSH keys in `<giteauser-home>/.ssh/authorized_keys` with a special `command=` option.
This setup only works well, when Gitea runs as a distinct user (i.e. `git`) so that the `authorized_keys` file can be exclusively managed by Gitea.

Gitea also provides its own SSH server, for usage when SSHD is not available.

### Adjusting your server for public/private use

#### Preventing spammers

There are multiple things you can combine to prevent spammers.

1. By whitelisting or blocklisting certain email domains
2. By only whitelisting certain domains with OpenID (see below)
3. Setting `ENABLE_CAPTCHA` to `true` in your `app.ini` and properly configuring `RECAPTCHA_SECRET` and `RECAPTCHA_SITEKEY`
4. Settings `DISABLE_REGISTRATION` to `true` and creating new users via the [CLI]({{< relref "doc/usage/command-line.en-us.md" >}}), [API]({{< relref "doc/developers/api-usage.en-us.md" >}}), or Gitea's Admin UI

#### Only allow/block certain email domains

You can configure `EMAIL_DOMAIN_WHITELIST` or `EMAIL_DOMAIN_BLOCKLIST` in your app.ini under `[service]`

#### Only allow/block certain OpenID providers

You can configure `WHITELISTED_URIS` or `BLACKLISTED_URIS` under `[openid]` in your `app.ini`  
**NOTE:** whitelisted takes precedence, so if it is non-blank then blacklisted is ignored

#### Issue only users

The current way to achieve this is to create/modify a user with a max repo creation limit of 0.
For organization repos, teams can be set up to only access the issue tracker.

#### Enable Fail2ban

Use [Fail2Ban]({{< relref "doc/usage/fail2ban-setup.en-us.md" >}}) to monitor and stop automated login attempts or other malicious behavior based on log patterns

### How to add/use custom themes

See [here]({{< relref "doc/advanced/customizing-gitea.en-us.md" >}}#customizing-the-look-of-gitea).

### How can I enable password reset

There is no setting for password resets. It is enabled when a [mail service]({{< relref "doc/usage/email-setup.en-us.md" >}}) is configured, and disabled otherwise.

---

## Troubleshooting

### How do I get debug logs?

For setting up debug logging for bug reports, see [here]({{< relref "doc/advanced/logging-documentation.en-us.md" >}}#debugging-problems).

### Repo displayed as empty or pushed commits don't show up in the UI

Essentially this problem boils down to git hooks not running.
That could be due to:

- You're pushing not through Gitea's SSH; see [SSH issues](#ssh-issues).
- The hooks not having the right path for gitea (ie. after moving installations. To fix this, click `Resynchronize pre-receive, update and post-receive hooks of all repositories` in the Gitea admin dashboard.
- The hooks aren't executable because they're on a `noexec` partition, or due to another security feature of the way Gitea is run.

### Custom Templates not loading or working incorrectly

Gitea's custom templates must be added to the correct location or Gitea will not find and use them.  
The correct path for the template(s) will be relative to the `CustomPath`

1. To find `CustomPath`, look for Custom File Root Path in Site Administration -> Configuration
    - If that doesn't exist, you can try `echo $GITEA_CUSTOM`
2. If you are still unable to find a path, the default can be [calculated above](#where-does-gitea-store-what-file)
3. Once you have figured out the correct custom path, you can refer to the [customizing Gitea]({{< relref "doc/advanced/customizing-gitea.en-us.md" >}}) page to add your template to the correct location.

### Not seeing a clone URL or the clone URL being incorrect

There are a few places that could make this show incorrectly.

1. If using a reverse proxy, make sure you have followed the correct instructions in the [reverse proxy guide]({{< relref "doc/usage/reverse-proxies.en-us.md" >}})
2. Make sure you have correctly set `ROOT_URL` in the `server` section of your `app.ini`

If certain clone options aren't showing up (HTTP/S or SSH), the following options can be checked in your `app.ini`

`DISABLE_HTTP_GIT`: if set to true, there will be no HTTP/HTTPS link  
`DISABLE_SSH`: if set to true, there will be no SSH link  
`SSH_EXPOSE_ANONYMOUS`: if set to false, SSH links will be hidden for anonymous users

### File upload fails with: 413 Request Entity Too Large

This error occurs when the reverse proxy limits the file upload size.

See the [reverse proxy guide]({{< relref "doc/usage/reverse-proxies.en-us.md" >}}) for a solution with nginx.

### Why Are Emoji Broken On MySQL

Unfortunately MySQL's `utf8` charset does not completely allow all possible UTF-8 characters, in particular Emoji.
They created a new charset and collation called `utf8mb4` that allows for emoji to be stored but tables which use
the `utf8` charset, and connections which use the `utf8` charset will not use this.

Please run `gitea convert`, or run `ALTER DATABASE database_name CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;`
for the database_name and run `ALTER TABLE table_name CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;`
for each table in the database.

You will also need to change the app.ini database charset to `CHARSET=utf8mb4`.

### Can't create repositories/files

Make sure that Gitea has sufficient permissions to write to its home directory and data directory.  
See [AppDataPath and RepoRootPath](#where-does-gitea-store-what-file)

**Note for Arch users:** At the time of writing this, there is an issue with the Arch package's systemd file including this line:
`ReadWritePaths=/etc/gitea/app.ini`  
Which makes all other paths non-writeable to Gitea.

### SSH issues

If you cannot reach repositories over `ssh`, but `https` works fine, consider looking into the following.

First, make sure you can access Gitea via SSH.  
`ssh git@myremote.example`

If the connection is successful, you should receive an error message like the following:

```
Hi there, You've successfully authenticated, but Gitea does not provide shell access.
If this is unexpected, please log in with password and setup Gitea under another user.
```

If you do not get the above message but still connect, it means your SSH key is **not** being managed by Gitea. This means hooks won't run, among other potential problems.
You can try to run `Update the '.ssh/authorized_keys' file with Gitea SSH keys.` in the admin dashboard.

If you cannot connect at all, your SSH key may not be configured correctly locally.
This is specific to SSH and not Gitea, so will not be covered here.

#### SSH Common Errors

```
Permission denied (publickey).
fatal: Could not read from remote repository.
```

This error signifies that the server rejected a log in attempt, check the
following things:

- On the client:
  - Ensure the public and private ssh keys are added to the correct Gitea user.
  - Make sure there are no issues in the remote url. In particular, ensure the name of the
    git user (before the `@`) is spelled correctly.
  - Ensure public and private ssh keys are correct on client machine.
- On the server:
  - Make sure the repository exists and is correctly named.
  - Check the permissions of the `.ssh` directory in the system user's home directory.
  - Verify that the correct public keys are added to `.ssh/authorized_keys`.  
    Try to run `Rewrite '.ssh/authorized_keys' file (for Gitea SSH keys)` on the
    Gitea admin panel.
  - Read Gitea logs.
  - Read /var/log/auth (or similar).
  - Check permissions of repositories.

The following is an example of a missing public SSH key where authentication
succeeded, but some other setting is preventing SSH from reaching the correct
repository.

```
fatal: Could not read from remote repository.

Please make sure you have the correct access rights
and the repository exists.
```

In this case, look into the following settings:

- On the server:
  - Make sure that the `git` system user has a usable shell set
    - Verify this with `getent passwd git | cut -d: -f7`
    - `usermod` or `chsh` can be used to modify this.
  - Ensure that the `gitea serv` command in `.ssh/authorized_keys` uses the
    correct configuration file.


### LFS Issues

For issues concerning LFS data upload

```
batch response: Authentication required: Authorization error: <GITEA_LFS_URL>/info/lfs/objects/batch
Check that you have proper access to the repository
error: failed to push some refs to '<GIT_REPO_URL>'
```

Check the value of `LFS_HTTP_AUTH_EXPIRY` in your `app.ini` file.  
By default, your LFS token will expire after 20 minutes. If you have a slow connection or a large file (or both), it may not finish uploading within the time limit.

You may want to set this value to `60m` or `120m`.

If git reports the following when pushing, you likely didn't push through Gitea, see [SSH issues](#ssh-issues).
```
error: exit status 127, message: bash: git-lfs-authenticate: command not found (try: 5/6)
```

### Gitea is running slow

The most common culprit for this is loading federated avatars.  
This can be turned off by setting `ENABLE_FEDERATED_AVATAR` to `false` in your `app.ini`  
Another option that may need to be changed is setting `DISABLE_GRAVATAR` to `true` in your `app.ini`

### Issues after updating

Please always check the release notes / [changelog](https://github.com/go-gitea/gitea/blob/main/CHANGELOG.md) for breaking changes; we regularly have breaking changes on minor releases!

#### Upgrade errors with MySQL

- If you are receiving errors on upgrade of Gitea using MySQL that read:

    > `ORM engine initialization failed: migrate: do migrate: Error: 1118: Row size too large...`

    Please run `gitea convert` or run `ALTER TABLE table_name ROW_FORMAT=dynamic;` for each table in the database.

    The underlying problem is that the space allocated for indices by the default row format
    is too small. Gitea requires that the `ROWFORMAT` for its tables is `DYNAMIC`.

- If you are receiving an error line containing `Error 1071: Specified key was too long; max key length is 1000 bytes...`
    then you are attempting to run Gitea on tables which use the ISAM engine. While this may have worked by chance in previous versions of Gitea, it has never been officially supported and
    you must use InnoDB. You should run `ALTER TABLE table_name ENGINE=InnoDB;` for each table in the database.  
    If you are using MySQL 5, another possible fix is
    ```mysql
    SET GLOBAL innodb_file_format=Barracuda;
    SET GLOBAL innodb_file_per_table=1;
    SET GLOBAL innodb_large_prefix=1;
    ```

---

## Usage

### How to migrate from Gogs/GitHub/etc. to Gitea

To migrate repositories including issues, pulls, releases etc, Gitea has built in support for various git forges (GitHub, Gitlab, Gitea, Gogs).
[Example (requires login)](https://try.gitea.io/repo/migrate)

To migrate an entire Gogs installation to Gitea:
- [Gogs version 0.9.146 or less]({{< relref "doc/upgrade/from-gogs.en-us.md" >}})
- [Gogs version 0.11.46.0418](https://github.com/go-gitea/gitea/issues/4286)

### User flags: active, prohibited, restricted

- An "active" has activated their account via email.

- A "login prohibited" user is a user that is not allowed to log in to Gitea anymore.

- Restricted users are limited to a subset of the content based on their organization/team memberships and collaborations, ignoring the public flag on organizations/repos etc.

    Example use case: A company runs a Gitea instance that requires login. Most repos are public (accessible/browsable by all co-workers).

    At some point, a customer or third party needs access to a specific repo and only that repo. Making such a customer account restricted and granting any needed access using team membership(s) and/or collaboration(s) is a simple way to achieve that without the need to make everything private.

### How can I create users before starting Gitea

Gitea provides a sub-command `gitea migrate` to initialize the database, after which you can use the [admin CLI commands]({{< relref "doc/usage/command-line.en-us.md#admin" >}}) to add users like normal.

### How can a user's password be changed

- As an **admin**, you can change any user's password (and optionally force them to change it on next login)...
  - By navigating to your `Site Administration -> User Accounts` page and editing a user.
  - By using the [admin CLI commands]({{< relref "doc/usage/command-line.en-us.md#admin" >}}).  
    Keep in mind most commands will also need a [global flag]({{< relref "doc/usage/command-line.en-us.md#global-options" >}}) to point the CLI at the correct configuration.
- As a **user** you can change it...
  - In your account `Settings -> Account` page (this method **requires** you to know your current password).
  - By using the `Forgot Password` link.  
    If the `Forgot Password/Account Recovery` page is disabled, please contact your administrator to configure a [mail service]({{< relref "doc/usage/email-setup.en-us.md" >}}).


### Missing releases after migrating repository with tags

To migrate an repository _with_ all tags, you need to do two things:

- Push tags to the repository:

```
 git push --tags
```

- (Re-)sync tags of all repositories within Gitea:

```
gitea admin repo-sync-releases
```

### Why are Emoji displaying only as placeholders or in monochrome

Gitea requires the system or browser to have one of the supported Emoji fonts installed, which are Apple Color Emoji, Segoe UI Emoji, Segoe UI Symbol, Noto Color Emoji and Twemoji Mozilla. Generally, the operating system should already provide one of these fonts, but especially on Linux, it may be necessary to install them manually.

---

## Misc

### To which Gitea version is this documentation referring?

Currently, this documentation is always referring to the latest development on the `main` branch.
We're looking provide separate documentation per release, see [this issue](https://github.com/go-gitea/gitea/issues/15279) if you want to help with that.

### Is there end-user documentation available?

Currently not, but Codeberg provides some helpful docs [here](https://docs.codeberg.org/).

### Difference between 1.x and 1.x.x downloads

Version 1.7.x will be used for this example.  
**NOTE:** this example applies to Docker images as well!

On our [downloads page](https://dl.gitea.io/gitea/) you will see a 1.7 directory, as well as directories for 1.7.0, 1.7.1, 1.7.2, 1.7.3, 1.7.4, 1.7.5, and 1.7.6.  
The 1.7 and 1.7.0 directories are **not** the same. The 1.7 directory is built on each merged commit to the [`release/v1.7`](https://github.com/go-gitea/gitea/tree/release/v1.7) branch.  
The 1.7.0 directory, however, is a build that was created when the [`v1.7.0`](https://github.com/go-gitea/gitea/releases/tag/v1.7.0) tag was created.

This means that 1.x downloads will change as commits are merged to their respective branch (think of it as a separate "master" branch for each release).
On the other hand, 1.x.x downloads should never change.

### Translation is incorrect/how to add more translations

See [here]({{< relref "doc/translation/localization.en-us.md" >}}) on how to add or change translations.

### What is Swagger?

[Swagger](https://swagger.io/) (or OpenAPI) is a standard Gitea uses for its API documentation.
This UI is available at `<instance-uri>/api/swagger` ([example](https://try.gitea.io/api/swagger)),
but it can be disabled by configuration]({{< relref "doc/advanced/config-cheat-sheet.en-us.md" >}}#api-api).
For more information on the API, refer to Gitea's [API docs]({{< relref "doc/developers/api-usage.en-us.md" >}})
