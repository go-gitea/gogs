---
date: "2016-12-26T16:00:00+02:00"
title: "Config Cheat Sheet"
slug: "config-cheat-sheet"
weight: 20
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Config Cheat Sheet"
    weight: 20
    identifier: "config-cheat-sheet"
---

# Configuration Cheat Sheet

This is a cheat sheet for the Gitea configuration file. It contains most of the settings
that can be configured as well as their default values.

Any changes to the Gitea configuration file should be made in `custom/conf/app.ini`
or any corresponding location. When installing from a distribution, this will
typically be found at `/etc/gitea/conf/app.ini`.

The defaults provided here are best-effort (not built automatically). They are
accurately recorded in [app.example.ini](https://github.com/go-gitea/gitea/blob/master/custom/conf/app.example.ini)
(s/master/\<tag|release\>). Any string in the format `%(X)s` is a feature powered
by [ini](https://github.com/go-ini/ini/#recursive-values), for reading values recursively.

Values containing `#` or `;` must be quoted using `` ` `` or `"""`.

**Note:** A full restart is required for Gitea configuration changes to take effect.

{{< toc >}}

## Overall (`DEFAULT`)

- `APP_NAME`: **Gitea: Git with a cup of tea**: Application name, used in the page title.
- `RUN_USER`: **git**: The user Gitea will run as. This should be a dedicated system
   (non-user) account. Setting this incorrectly will cause Gitea to not start.
- `RUN_MODE`: **prod**: Application run mode, affects performance and debugging. Either "dev", "prod" or "test".

## Repository (`repository`)

- `ROOT`: **data/gitea-repositories/**: Root path for storing all repository data. It must be
   an absolute path. By default it is stored in a sub-directory of `APP_DATA_PATH`.
- `SCRIPT_TYPE`: **bash**: The script type this server supports. Usually this is `bash`,
   but some users report that only `sh` is available.
- `DETECTED_CHARSETS_ORDER`: **UTF-8, UTF-16BE, UTF-16LE, UTF-32BE, UTF-32LE, ISO-8859, windows-1252, ISO-8859, windows-1250, ISO-8859, ISO-8859, ISO-8859, windows-1253, ISO-8859, windows-1255, ISO-8859, windows-1251, windows-1256, KOI8-R, ISO-8859, windows-1254, Shift_JIS, GB18030, EUC-JP, EUC-KR, Big5, ISO-2022, ISO-2022, ISO-2022, IBM424_rtl, IBM424_ltr, IBM420_rtl, IBM420_ltr**: Tie-break order of detected charsets - if the detected charsets have equal confidence, charsets earlier in the list will be chosen in preference to those later. Adding `defaults` will place the unnamed charsets at that point.
- `ANSI_CHARSET`: **\<empty\>**: Default ANSI charset to override non-UTF-8 charsets to.
- `FORCE_PRIVATE`: **false**: Force every new repository to be private.
- `DEFAULT_PRIVATE`: **last**: Default private when creating a new repository.
   \[last, private, public\]
- `DEFAULT_PUSH_CREATE_PRIVATE`: **true**: Default private when creating a new repository with push-to-create.
- `MAX_CREATION_LIMIT`: **-1**: Global maximum creation limit of repositories per user,
   `-1` means no limit.
- `PULL_REQUEST_QUEUE_LENGTH`: **1000**: Length of pull request patch test queue, make it
   as large as possible. Use caution when editing this value.
- `MIRROR_QUEUE_LENGTH`: **1000**: Patch test queue length, increase if pull request patch
   testing starts hanging.
- `PREFERRED_LICENSES`: **Apache License 2.0,MIT License**: Preferred Licenses to place at
   the top of the list. Name must match file name in options/license or custom/options/license.
- `DISABLE_HTTP_GIT`: **false**: Disable the ability to interact with repositories over the
   HTTP protocol.
- `USE_COMPAT_SSH_URI`: **false**: Force ssh:// clone url instead of scp-style uri when
   default SSH port is used.
- `ACCESS_CONTROL_ALLOW_ORIGIN`: **\<empty\>**: Value for Access-Control-Allow-Origin header,
   default is not to present. **WARNING**: This maybe harmful to you website if you do not
   give it a right value.
- `DEFAULT_CLOSE_ISSUES_VIA_COMMITS_IN_ANY_BRANCH`:  **false**: Close an issue if a commit on a non default branch marks it as closed.
- `ENABLE_PUSH_CREATE_USER`:  **false**: Allow users to push local repositories to Gitea and have them automatically created for a user.
- `ENABLE_PUSH_CREATE_ORG`:  **false**: Allow users to push local repositories to Gitea and have them automatically created for an org.
- `DISABLED_REPO_UNITS`: **_empty_**: Comma separated list of globally disabled repo units. Allowed values: \[repo.issues, repo.ext_issues, repo.pulls, repo.wiki, repo.ext_wiki, repo.projects\]
- `DEFAULT_REPO_UNITS`: **repo.code,repo.releases,repo.issues,repo.pulls,repo.wiki,repo.projects**: Comma separated list of default repo units. Allowed values: \[repo.code, repo.releases, repo.issues, repo.pulls, repo.wiki, repo.projects\]. Note: Code and Releases can currently not be deactivated. If you specify default repo units you should still list them for future compatibility. External wiki and issue tracker can't be enabled by default as it requires additional settings. Disabled repo units will not be added to new repositories regardless if it is in the default list.
- `PREFIX_ARCHIVE_FILES`: **true**: Prefix archive files by placing them in a directory named after the repository.
- `DISABLE_MIRRORS`: **false**: Disable the creation of **new** mirrors. Pre-existing mirrors remain valid.
- `DISABLE_MIGRATIONS`: **false**: Disable migrating feature.
- `DISABLE_STARS`: **false**: Disable stars feature.
- `DEFAULT_BRANCH`: **master**: Default branch name of all repositories.
- `ALLOW_ADOPTION_OF_UNADOPTED_REPOSITORIES`: **false**: Allow non-admin users to adopt unadopted repositories
- `ALLOW_DELETION_OF_UNADOPTED_REPOSITORIES`: **false**: Allow non-admin users to delete unadopted repositories

### Repository - Editor (`repository.editor`)

- `LINE_WRAP_EXTENSIONS`: **.txt,.md,.markdown,.mdown,.mkd,**: List of file extensions for which lines should be wrapped in the Monaco editor. Separate extensions with a comma. To line wrap files without an extension, just put a comma
- `PREVIEWABLE_FILE_MODES`: **markdown**: Valid file modes that have a preview API associated with them, such as `api/v1/markdown`. Separate the values by commas. The preview tab in edit mode won't be displayed if the file extension doesn't match.

### Repository - Pull Request (`repository.pull-request`)

- `WORK_IN_PROGRESS_PREFIXES`: **WIP:,\[WIP\]**: List of prefixes used in Pull Request
 title to mark them as Work In Progress
- `CLOSE_KEYWORDS`: **close**, **closes**, **closed**, **fix**, **fixes**, **fixed**, **resolve**, **resolves**, **resolved**: List of
 keywords used in Pull Request comments to automatically close a related issue
- `REOPEN_KEYWORDS`: **reopen**, **reopens**, **reopened**: List of keywords used in Pull Request comments to automatically reopen
 a related issue
- `DEFAULT_MERGE_MESSAGE_COMMITS_LIMIT`: **50**: In the default merge message for squash commits include at most this many commits. Set to `-1` to include all commits
- `DEFAULT_MERGE_MESSAGE_SIZE`: **5120**: In the default merge message for squash commits limit the size of the commit messages. Set to `-1` to have no limit. Only used if `POPULATE_SQUASH_COMMENT_WITH_COMMIT_MESSAGES` is `true`.
- `DEFAULT_MERGE_MESSAGE_ALL_AUTHORS`: **false**: In the default merge message for squash commits walk all commits to include all authors in the Co-authored-by otherwise just use those in the limited list
- `DEFAULT_MERGE_MESSAGE_MAX_APPROVERS`: **10**: In default merge messages limit the number of approvers listed as `Reviewed-by:`. Set to `-1` to include all.
- `DEFAULT_MERGE_MESSAGE_OFFICIAL_APPROVERS_ONLY`: **true**: In default merge messages only include approvers who are officially allowed to review.
- `POPULATE_SQUASH_COMMENT_WITH_COMMIT_MESSAGES`: **false**: In default squash-merge messages include the commit message of all commits comprising the pull request.

### Repository - Issue (`repository.issue`)

- `LOCK_REASONS`: **Too heated,Off-topic,Resolved,Spam**: A list of reasons why a Pull Request or Issue can be locked

### Repository - Upload (`repository.upload`)

- `ENABLED`: **true**: Whether repository file uploads are enabled
- `TEMP_PATH`: **data/tmp/uploads**: Path for uploads (tmp gets deleted on gitea restart)
- `ALLOWED_TYPES`: **\<empty\>**: Comma-separated list of allowed file extensions (`.zip`), mime types (`text/plain`) or wildcard type (`image/*`, `audio/*`, `video/*`). Empty value or `*/*` allows all types.
- `FILE_MAX_SIZE`: **3**: Max size of each file in megabytes.
- `MAX_FILES`: **5**: Max number of files per upload

### Repository - Release (`repository.release`)

- `ALLOWED_TYPES`: **\<empty\>**: Comma-separated list of allowed file extensions (`.zip`), mime types (`text/plain`) or wildcard type (`image/*`, `audio/*`, `video/*`). Empty value or `*/*` allows all types.

### Repository - Signing (`repository.signing`)

- `SIGNING_KEY`: **default**: \[none, KEYID, default \]: Key to sign with.
- `SIGNING_NAME` &amp; `SIGNING_EMAIL`: if a KEYID is provided as the `SIGNING_KEY`, use these as the Name and Email address of the signer. These should match publicized name and email address for the key.
- `INITIAL_COMMIT`: **always**: \[never, pubkey, twofa, always\]: Sign initial commit.
  - `never`: Never sign
  - `pubkey`: Only sign if the user has a public key
  - `twofa`: Only sign if the user is logged in with twofa
  - `always`: Always sign
  - Options other than `never` and `always` can be combined as a comma separated list.
- `DEFAULT_TRUST_MODEL`: **collaborator**: \[collaborator, committer, collaboratorcommitter\]: The default trust model used for verifying commits.
   - `collaborator`: Trust signatures signed by keys of collaborators.
   - `committer`: Trust signatures that match committers (This matches GitHub and will force Gitea signed commits to have Gitea as the committer).
   - `collaboratorcommitter`: Trust signatures signed by keys of collaborators which match the committer.
- `WIKI`: **never**: \[never, pubkey, twofa, always, parentsigned\]: Sign commits to wiki.
- `CRUD_ACTIONS`: **pubkey, twofa, parentsigned**: \[never, pubkey, twofa, parentsigned, always\]: Sign CRUD actions.
  - Options as above, with the addition of:
  - `parentsigned`: Only sign if the parent commit is signed.
- `MERGES`: **pubkey, twofa, basesigned, commitssigned**: \[never, pubkey, twofa, approved, basesigned, commitssigned, always\]: Sign merges.
  - `approved`: Only sign approved merges to a protected branch.
  - `basesigned`: Only sign if the parent commit in the base repo is signed.
  - `headsigned`: Only sign if the head commit in the head branch is signed.
  - `commitssigned`: Only sign if all the commits in the head branch to the merge point are signed.

## Repository - Local (`repository.local`)

- `LOCAL_COPY_PATH`: **tmp/local-repo**: Path for temporary local repository copies. Defaults to `tmp/local-repo`

## Repository -  MIME type mapping (`repository.mimetype_mapping`)

Configuration for set the expected MIME type based on file extensions of downloadable files. Configuration presents in key-value pairs and file extensions starts with leading `.`.

The following configuration set `Content-Type: application/vnd.android.package-archive` header when downloading files with `.apk` file extension.
```ini
.apk=application/vnd.android.package-archive
```

## CORS (`cors`)

- `ENABLED`: **false**: enable cors headers (disabled by default)
- `SCHEME`: **http**: scheme of allowed requests
- `ALLOW_DOMAIN`: **\***: list of requesting domains that are allowed
- `ALLOW_SUBDOMAIN`: **false**: allow subdomains of headers listed above to request
- `METHODS`: **GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS**: list of methods allowed to request
- `MAX_AGE`: **10m**: max time to cache response
- `ALLOW_CREDENTIALS`: **false**: allow request with credentials

## UI (`ui`)

- `EXPLORE_PAGING_NUM`: **20**: Number of repositories that are shown in one explore page.
- `ISSUE_PAGING_NUM`: **10**: Number of issues that are shown in one page (for all pages that list issues).
- `MEMBERS_PAGING_NUM`: **20**: Number of members that are shown in organization members.
- `FEED_MAX_COMMIT_NUM`: **5**: Number of maximum commits shown in one activity feed.
- `FEED_PAGING_NUM`: **20**: Number of items that are displayed in home feed.
- `GRAPH_MAX_COMMIT_NUM`: **100**: Number of maximum commits shown in the commit graph.
- `CODE_COMMENT_LINES`: **4**: Number of line of codes shown for a code comment.
- `DEFAULT_THEME`: **gitea**: \[gitea, arc-green\]: Set the default theme for the Gitea install.
- `SHOW_USER_EMAIL`: **true**: Whether the email of the user should be shown in the Explore Users page.
- `THEMES`:  **gitea,arc-green**: All available themes. Allow users select personalized themes.
  regardless of the value of `DEFAULT_THEME`.
- `THEME_COLOR_META_TAG`: **#6cc644**:  Value of `theme-color` meta tag, used by Android >= 5.0. An invalid color like "none" or "disable" will have the default style.  More info: https://developers.google.com/web/updates/2014/11/Support-for-theme-color-in-Chrome-39-for-Android
- `MAX_DISPLAY_FILE_SIZE`: **8388608**: Max size of files to be displayed (default is 8MiB)
- `REACTIONS`: All available reactions users can choose on issues/prs and comments
    Values can be emoji alias (:smile:) or a unicode emoji.
    For custom reactions, add a tightly cropped square image to public/img/emoji/reaction_name.png
- `CUSTOM_EMOJIS`: **gitea, codeberg, gitlab, git, github, gogs**: Additional Emojis not defined in the utf8 standard.
    By default we support gitea (:gitea:), to add more copy them to public/img/emoji/emoji_name.png and
    add it to this config.
- `DEFAULT_SHOW_FULL_NAME`: **false**: Whether the full name of the users should be shown where possible. If the full name isn't set, the username will be used.
- `SEARCH_REPO_DESCRIPTION`: **true**: Whether to search within description at repository search on explore page.
- `USE_SERVICE_WORKER`: **true**: Whether to enable a Service Worker to cache frontend assets.

### UI - Admin (`ui.admin`)

- `USER_PAGING_NUM`: **50**: Number of users that are shown in one page.
- `REPO_PAGING_NUM`: **50**: Number of repos that are shown in one page.
- `NOTICE_PAGING_NUM`: **25**: Number of notices that are shown in one page.
- `ORG_PAGING_NUM`: **50**: Number of organizations that are shown in one page.

### UI - Metadata (`ui.meta`)

- `AUTHOR`: **Gitea - Git with a cup of tea**: Author meta tag of the homepage.
- `DESCRIPTION`: **Gitea (Git with a cup of tea) is a painless self-hosted Git service written in Go**: Description meta tag of the homepage.
- `KEYWORDS`: **go,git,self-hosted,gitea**: Keywords meta tag of the homepage.

### UI - Notification (`ui.notification`)

- `MIN_TIMEOUT`: **10s**: These options control how often notification endpoint is polled to update the notification count. On page load the notification count will be checked after `MIN_TIMEOUT`. The timeout will increase to `MAX_TIMEOUT` by `TIMEOUT_STEP` if the notification count is unchanged. Set MIN_TIMEOUT to 0 to turn off.
- `MAX_TIMEOUT`: **60s**.
- `TIMEOUT_STEP`: **10s**.
- `EVENT_SOURCE_UPDATE_TIME`: **10s**: This setting determines how often the database is queried to update notification counts. If the browser client supports `EventSource` and `SharedWorker`, a `SharedWorker` will be used in preference to polling notification endpoint. Set to **-1** to disable the `EventSource`.

### UI - SVG Images (`ui.svg`)

- `ENABLE_RENDER`: **true**: Whether to render SVG files as images.  If SVG rendering is disabled, SVG files are displayed as text and cannot be embedded in markdown files as images.

### UI - CSV Files (`ui.csv`)

- `MAX_FILE_SIZE`: **524288** (512kb): Maximum allowed file size in bytes to render CSV files as table. (Set to 0 for no limit).

## Markdown (`markdown`)

- `ENABLE_HARD_LINE_BREAK_IN_COMMENTS`: **true**: Render soft line breaks as hard line breaks in comments, which
  means a single newline character between paragraphs will cause a line break and adding
  trailing whitespace to paragraphs is not necessary to force a line break.
- `ENABLE_HARD_LINE_BREAK_IN_DOCUMENTS`: **false**: Render soft line breaks as hard line breaks in documents, which
  means a single newline character between paragraphs will cause a line break and adding
  trailing whitespace to paragraphs is not necessary to force a line break.
- `CUSTOM_URL_SCHEMES`: Use a comma separated list (ftp,git,svn) to indicate additional
  URL hyperlinks to be rendered in Markdown. URLs beginning in http and https are
  always displayed

## Server (`server`)

- `PROTOCOL`: **http**: \[http, https, fcgi, unix, fcgi+unix\]
- `DOMAIN`: **localhost**: Domain name of this server.
- `ROOT_URL`: **%(PROTOCOL)s://%(DOMAIN)s:%(HTTP\_PORT)s/**:
   Overwrite the automatically generated public URL.
   This is useful if the internal and the external URL don't match (e.g. in Docker).
- `STATIC_URL_PREFIX`: **\<empty\>**:
   Overwrite this option to request static resources from a different URL.
   This includes CSS files, images, JS files and web fonts.
   Avatar images are dynamic resources and still served by gitea.
   The option can be just a different path, as in `/static`, or another domain, as in `https://cdn.example.com`.
   Requests are then made as `%(ROOT_URL)s/static/css/index.css` and `https://cdn.example.com/css/index.css` respective.
   The static files are located in the `public/` directory of the gitea source repository.
- `HTTP_ADDR`: **0.0.0.0**: HTTP listen address.
   - If `PROTOCOL` is set to `fcgi`, Gitea will listen for FastCGI requests on TCP socket
     defined by `HTTP_ADDR` and `HTTP_PORT` configuration settings.
   - If `PROTOCOL` is set to `unix` or `fcgi+unix`, this should be the name of the Unix socket file to use.
- `HTTP_PORT`: **3000**: HTTP listen port.
   - If `PROTOCOL` is set to `fcgi`, Gitea will listen for FastCGI requests on TCP socket
     defined by `HTTP_ADDR` and `HTTP_PORT` configuration settings.
- `UNIX_SOCKET_PERMISSION`: **666**: Permissions for the Unix socket.
- `LOCAL_ROOT_URL`: **%(PROTOCOL)s://%(HTTP_ADDR)s:%(HTTP_PORT)s/**: Local
   (DMZ) URL for Gitea workers (such as SSH update) accessing web service. In
   most cases you do not need to change the default value. Alter it only if
   your SSH server node is not the same as HTTP node. Do not set this variable
   if `PROTOCOL` is set to `unix`.
- `PER_WRITE_TIMEOUT`: **30s**: Timeout for any write to the connection. (Set to 0 to
   disable all timeouts.)
- `PER_WRITE_PER_KB_TIMEOUT`: **10s**: Timeout per Kb written to connections.

- `DISABLE_SSH`: **false**: Disable SSH feature when it's not available.
- `START_SSH_SERVER`: **false**: When enabled, use the built-in SSH server.
- `BUILTIN_SSH_SERVER_USER`: **%(RUN_USER)s**: Username to use for the built-in SSH Server.
- `SSH_DOMAIN`: **%(DOMAIN)s**: Domain name of this server, used for displayed clone URL.
- `SSH_PORT`: **22**: SSH port displayed in clone URL.
- `SSH_LISTEN_HOST`: **0.0.0.0**: Listen address for the built-in SSH server.
- `SSH_LISTEN_PORT`: **%(SSH\_PORT)s**: Port for the built-in SSH server.
- `SSH_ROOT_PATH`: **~/.ssh**: Root path of SSH directory.
- `SSH_CREATE_AUTHORIZED_KEYS_FILE`: **true**: Gitea will create a authorized_keys file by default when it is not using the internal ssh server. If you intend to use the AuthorizedKeysCommand functionality then you should turn this off.
- `SSH_AUTHORIZED_KEYS_BACKUP`: **true**: Enable SSH Authorized Key Backup when rewriting all keys, default is true.
- `SSH_TRUSTED_USER_CA_KEYS`: **\<empty\>**: Specifies the public keys of certificate authorities that are trusted to sign user certificates for authentication. Multiple keys should be comma separated. E.g.`ssh-<algorithm> <key>` or `ssh-<algorithm> <key1>, ssh-<algorithm> <key2>`. For more information see `TrustedUserCAKeys` in the sshd config man pages. When empty no file will be created and `SSH_AUTHORIZED_PRINCIPALS_ALLOW` will default to `off`.
- `SSH_TRUSTED_USER_CA_KEYS_FILENAME`: **`RUN_USER`/.ssh/gitea-trusted-user-ca-keys.pem**: Absolute path of the `TrustedUserCaKeys` file gitea will manage. If you're running your own ssh server and you want to use the gitea managed file you'll also need to modify your sshd_config to point to this file. The official docker image will automatically work without further configuration.
- `SSH_AUTHORIZED_PRINCIPALS_ALLOW`: **off** or **username, email**: \[off, username, email, anything\]: Specify the principals values that users are allowed to use as principal. When set to `anything` no checks are done on the principal string. When set to `off` authorized principal are not allowed to be set.
- `SSH_CREATE_AUTHORIZED_PRINCIPALS_FILE`: **false/true**: Gitea will create a authorized_principals file by default when it is not using the internal ssh server and `SSH_AUTHORIZED_PRINCIPALS_ALLOW` is not `off`.
- `SSH_AUTHORIZED_PRINCIPALS_BACKUP`: **false/true**: Enable SSH Authorized Principals Backup when rewriting all keys, default is true if `SSH_AUTHORIZED_PRINCIPALS_ALLOW` is not `off`.
- `SSH_AUTHORIZED_KEYS_COMMAND_TEMPLATE`: **{{.AppPath}} --config={{.CustomConf}} serv key-{{.Key.ID}}**: Set the template for the command to passed on authorized keys. Possible keys are: AppPath, AppWorkPath, CustomConf, CustomPath, Key - where Key is a `models.PublicKey` and the others are strings which are shellquoted.
- `SSH_SERVER_CIPHERS`: **aes128-ctr, aes192-ctr, aes256-ctr, aes128-gcm@openssh.com, arcfour256, arcfour128**: For the built-in SSH server, choose the ciphers to support for SSH connections, for system SSH this setting has no effect.
- `SSH_SERVER_KEY_EXCHANGES`: **diffie-hellman-group1-sha1, diffie-hellman-group14-sha1, ecdh-sha2-nistp256, ecdh-sha2-nistp384, ecdh-sha2-nistp521, curve25519-sha256@libssh.org**: For the built-in SSH server, choose the key exchange algorithms to support for SSH connections, for system SSH this setting has no effect.
- `SSH_SERVER_MACS`: **hmac-sha2-256-etm@openssh.com, hmac-sha2-256, hmac-sha1, hmac-sha1-96**: For the built-in SSH server, choose the MACs to support for SSH connections, for system SSH this setting has no effect
- `SSH_SERVER_HOST_KEYS`: **ssh/gitea.rsa, ssh/gogs.rsa**: For the built-in SSH server, choose the keypairs to offer as the host key. The private key should be at `SSH_SERVER_HOST_KEY` and the public `SSH_SERVER_HOST_KEY.pub`. Relative paths are made absolute relative to the `APP_DATA_PATH`. If no key exists a 4096 bit RSA key will be created for you.
- `SSH_KEY_TEST_PATH`: **/tmp**: Directory to create temporary files in when testing public keys using ssh-keygen, default is the system temporary directory.
- `SSH_KEYGEN_PATH`: **ssh-keygen**: Path to ssh-keygen, default is 'ssh-keygen' which means the shell is responsible for finding out which one to call.
- `SSH_EXPOSE_ANONYMOUS`: **false**: Enable exposure of SSH clone URL to anonymous visitors, default is false.
- `SSH_PER_WRITE_TIMEOUT`: **30s**: Timeout for any write to the SSH connections. (Set to
  0 to disable all timeouts.)
- `SSH_PER_WRITE_PER_KB_TIMEOUT`: **10s**: Timeout per Kb written to SSH connections.
- `MINIMUM_KEY_SIZE_CHECK`: **true**: Indicate whether to check minimum key size with corresponding type.

- `OFFLINE_MODE`: **false**: Disables use of CDN for static files and Gravatar for profile pictures.
- `DISABLE_ROUTER_LOG`: **false**: Mute printing of the router log.
- `CERT_FILE`: **https/cert.pem**: Cert file path used for HTTPS. When chaining, the server certificate must come first, then intermediate CA certificates (if any). From 1.11 paths are relative to `CUSTOM_PATH`.
- `KEY_FILE`: **https/key.pem**: Key file path used for HTTPS. From 1.11 paths are relative to `CUSTOM_PATH`.
- `STATIC_ROOT_PATH`: **./**: Upper level of template and static files path.
- `APP_DATA_PATH`: **data** (**/data/gitea** on docker): Default path for application data.
- `STATIC_CACHE_TIME`: **6h**: Web browser cache time for static resources on `custom/`, `public/` and all uploaded avatars. Note that this cache is disabled when `RUN_MODE` is "dev".
- `ENABLE_GZIP`: **false**: Enable gzip compression for runtime-generated content, static resources excluded.
- `ENABLE_PPROF`: **false**: Application profiling (memory and cpu). For "web" command it listens on localhost:6060. For "serv" command it dumps to disk at `PPROF_DATA_PATH` as `(cpuprofile|memprofile)_<username>_<temporary id>`
- `PPROF_DATA_PATH`: **data/tmp/pprof**: `PPROF_DATA_PATH`, use an absolute path when you start gitea as service
- `LANDING_PAGE`: **home**: Landing page for unauthenticated users \[home, explore, organizations, login\].

- `LFS_START_SERVER`: **false**: Enables git-lfs support.
- `LFS_CONTENT_PATH`: **%(APP_DATA_PATH)/lfs**:  DEPRECATED: Default LFS content path. (if it is on local storage.)
- `LFS_JWT_SECRET`: **\<empty\>**: LFS authentication secret, change this a unique string.
- `LFS_HTTP_AUTH_EXPIRY`: **20m**: LFS authentication validity period in time.Duration, pushes taking longer than this may fail.
- `LFS_MAX_FILE_SIZE`: **0**: Maximum allowed LFS file size in bytes (Set to 0 for no limit).
- `LFS_LOCKS_PAGING_NUM`: **50**: Maximum number of LFS Locks returned per page.

- `REDIRECT_OTHER_PORT`: **false**: If true and `PROTOCOL` is https, allows redirecting http requests on `PORT_TO_REDIRECT` to the https port Gitea listens on.
- `PORT_TO_REDIRECT`: **80**: Port for the http redirection service to listen on. Used when `REDIRECT_OTHER_PORT` is true.
- `ENABLE_LETSENCRYPT`: **false**: If enabled you must set `DOMAIN` to valid internet facing domain (ensure DNS is set and port 80 is accessible by letsencrypt validation server).
   By using Lets Encrypt **you must consent** to their [terms of service](https://letsencrypt.org/documents/LE-SA-v1.2-November-15-2017.pdf).
- `LETSENCRYPT_ACCEPTTOS`: **false**: This is an explicit check that you accept the terms of service for Let's Encrypt.
- `LETSENCRYPT_DIRECTORY`: **https**: Directory that Letsencrypt will use to cache information such as certs and private keys.
- `LETSENCRYPT_EMAIL`: **email@example.com**: Email used by Letsencrypt to notify about problems with issued certificates. (No default)
- `ALLOW_GRACEFUL_RESTARTS`: **true**: Perform a graceful restart on SIGHUP
- `GRACEFUL_HAMMER_TIME`: **60s**: After a restart the parent process will stop accepting new connections and will allow requests to finish before stopping. Shutdown will be forced if it takes longer than this time.
- `STARTUP_TIMEOUT`: **0**: Shutsdown the server if startup takes longer than the provided time. On Windows setting this sends a waithint to the SVC host to tell the SVC host startup may take some time. Please note startup is determined by the opening of the listeners - HTTP/HTTPS/SSH. Indexers may take longer to startup and can have their own timeouts.

## Database (`database`)

- `DB_TYPE`: **mysql**: The database type in use \[mysql, postgres, mssql, sqlite3\].
- `HOST`: **127.0.0.1:3306**: Database host address and port or absolute path for unix socket \[mysql, postgres\] (ex: /var/run/mysqld/mysqld.sock).
- `NAME`: **gitea**: Database name.
- `USER`: **root**: Database username.
- `PASSWD`: **\<empty\>**: Database user password. Use \`your password\` or """your password""" for quoting if you use special characters in the password.
- `SCHEMA`: **\<empty\>**: For PostgreSQL only, schema to use if different from "public". The schema must exist beforehand,
  the user must have creation privileges on it, and the user search path must be set to the look into the schema first
  (e.g. `ALTER USER user SET SEARCH_PATH = schema_name,"$user",public;`).
- `SSL_MODE`: **disable**: SSL/TLS encryption mode for connecting to the database. This option is only applied for PostgreSQL and MySQL.
  - Valid values for MySQL:
     - `true`: Enable TLS with verification of the database server certificate against its root certificate. When selecting this option make sure that the root certificate required to validate the database server certificate (e.g. the CA certificate) is on the system certificate store of both the database and Gitea servers. See your system documentation for instructions on how to add a CA certificate to the certificate store.
     - `false`: Disable TLS.
     - `disable`: Alias for `false`, for compatibility with PostgreSQL.
     - `skip-verify`: Enable TLS without database server certificate verification. Use this option if you have self-signed or invalid certificate on the database server.
     - `prefer`: Enable TLS with fallback to non-TLS connection.
  - Valid values for PostgreSQL:
     - `disable`: Disable TLS.
     - `require`: Enable TLS without any verifications.
     - `verify-ca`: Enable TLS with verification of the database server certificate against its root certificate.
     - `verify-full`: Enable TLS and verify the database server name matches the given certificate in either the `Common Name` or `Subject Alternative Name` fields.
- `SQLITE_TIMEOUT`: **500**: Query timeout for sqlite3 only.
- `ITERATE_BUFFER_SIZE`: **50**: Internal buffer size for iterating.
- `CHARSET`: **utf8mb4**: For MySQL only, either "utf8" or "utf8mb4". NOTICE: for "utf8mb4" you must use MySQL InnoDB > 5.6. Gitea is unable to check this.
- `PATH`: **data/gitea.db**: For SQLite3 only, the database file path.
- `LOG_SQL`: **true**: Log the executed SQL.
- `DB_RETRIES`: **10**: How many ORM init / DB connect attempts allowed.
- `DB_RETRY_BACKOFF`: **3s**: time.Duration to wait before trying another ORM init / DB connect attempt, if failure occurred.
- `MAX_OPEN_CONNS` **0**: Database maximum open connections - default is 0, meaning there is no limit.
- `MAX_IDLE_CONNS` **2**: Max idle database connections on connection pool, default is 2 - this will be capped to `MAX_OPEN_CONNS`.
- `CONN_MAX_LIFETIME` **0 or 3s**: Sets the maximum amount of time a DB connection may be reused - default is 0, meaning there is no limit (except on MySQL where it is 3s - see #6804 & #7071).

Please see #8540 & #8273 for further discussion of the appropriate values for `MAX_OPEN_CONNS`, `MAX_IDLE_CONNS` & `CONN_MAX_LIFETIME` and their
relation to port exhaustion.

## Indexer (`indexer`)

- `ISSUE_INDEXER_TYPE`: **bleve**: Issue indexer type, currently supported: `bleve`, `db` or `elasticsearch`.
- `ISSUE_INDEXER_CONN_STR`: ****: Issue indexer connection string, available when ISSUE_INDEXER_TYPE is elasticsearch. i.e. http://elastic:changeme@localhost:9200
- `ISSUE_INDEXER_NAME`: **gitea_issues**: Issue indexer name, available when ISSUE_INDEXER_TYPE is elasticsearch
- `ISSUE_INDEXER_PATH`: **indexers/issues.bleve**: Index file used for issue search; available when ISSUE_INDEXER_TYPE is bleve and elasticsearch.
- The next 4 configuration values are deprecated and should be set in `queue.issue_indexer` however are kept for backwards compatibility:
- `ISSUE_INDEXER_QUEUE_TYPE`: **levelqueue**: Issue indexer queue, currently supports:`channel`, `levelqueue`, `redis`. **DEPRECATED** use settings in `[queue.issue_indexer]`.
- `ISSUE_INDEXER_QUEUE_DIR`: **queues/common**: When `ISSUE_INDEXER_QUEUE_TYPE` is `levelqueue`, this will be the path where the queue will be saved. **DEPRECATED** use settings in `[queue.issue_indexer]`.
- `ISSUE_INDEXER_QUEUE_CONN_STR`: **addrs=127.0.0.1:6379 db=0**: When `ISSUE_INDEXER_QUEUE_TYPE` is `redis`, this will store the redis connection string. When `ISSUE_INDEXER_QUEUE_TYPE` is `levelqueue`, this is a directory or additional options of the form `leveldb://path/to/db?option=value&....`, and overrides `ISSUE_INDEXER_QUEUE_DIR`. **DEPRECATED** use settings in `[queue.issue_indexer]`.
- `ISSUE_INDEXER_QUEUE_BATCH_NUMBER`: **20**: Batch queue number. **DEPRECATED** use settings in `[queue.issue_indexer]`.

- `REPO_INDEXER_ENABLED`: **false**: Enables code search (uses a lot of disk space, about 6 times more than the repository size).
- `REPO_INDEXER_TYPE`: **bleve**: Code search engine type, could be `bleve` or `elasticsearch`.
- `REPO_INDEXER_PATH`: **indexers/repos.bleve**: Index file used for code search.
- `REPO_INDEXER_CONN_STR`: ****: Code indexer connection string, available when `REPO_INDEXER_TYPE` is elasticsearch. i.e. http://elastic:changeme@localhost:9200
- `REPO_INDEXER_NAME`: **gitea_codes**: Code indexer name, available when `REPO_INDEXER_TYPE` is elasticsearch

- `REPO_INDEXER_INCLUDE`: **empty**: A comma separated list of glob patterns (see https://github.com/gobwas/glob) to **include** in the index. Use `**.txt` to match any files with .txt extension. An empty list means include all files.
- `REPO_INDEXER_EXCLUDE`: **empty**: A comma separated list of glob patterns (see https://github.com/gobwas/glob) to **exclude** from the index. Files that match this list will not be indexed, even if they match in `REPO_INDEXER_INCLUDE`.
- `REPO_INDEXER_EXCLUDE_VENDORED`: **true**: Exclude vendored files from index.
- `UPDATE_BUFFER_LEN`: **20**: Buffer length of index request. **DEPRECATED** use settings in `[queue.issue_indexer]`.
- `MAX_FILE_SIZE`: **1048576**: Maximum size in bytes of files to be indexed.
- `STARTUP_TIMEOUT`: **30s**: If the indexer takes longer than this timeout to start - fail. (This timeout will be added to the hammer time above for child processes - as bleve will not start until the previous parent is shutdown.) Set to zero to never timeout.

## Queue (`queue` and `queue.*`)

- `TYPE`: **persistable-channel**: General queue type, currently support: `persistable-channel` (uses a LevelDB internally), `channel`, `level`, `redis`, `dummy`
- `DATADIR`: **queues/**: Base DataDir for storing persistent and level queues. `DATADIR` for individual queues can be set in `queue.name` sections but will default to `DATADIR/`**`common`**. (Previously each queue would default to `DATADIR/`**`name`**.)
- `LENGTH`: **20**: Maximal queue size before channel queues block
- `BATCH_LENGTH`: **20**: Batch data before passing to the handler
- `CONN_STR`: **redis://127.0.0.1:6379/0**: Connection string for the redis queue type. Options can be set using query params. Similarly LevelDB options can also be set using: **leveldb://relative/path?option=value** or **leveldb:///absolute/path?option=value**, and will override `DATADIR`
- `QUEUE_NAME`: **_queue**: The suffix for default redis and disk queue name. Individual queues will default to **`name`**`QUEUE_NAME` but can be overridden in the specific `queue.name` section.
- `SET_NAME`: **_unique**: The suffix that will be added to the default redis and disk queue `set` name for unique queues. Individual queues will default to
 **`name`**`QUEUE_NAME`_`SET_NAME`_ but can be overridden in the specific `queue.name` section.
- `WRAP_IF_NECESSARY`: **true**: Will wrap queues with a timeoutable queue if the selected queue is not ready to be created - (Only relevant for the level queue.)
- `MAX_ATTEMPTS`: **10**: Maximum number of attempts to create the wrapped queue
- `TIMEOUT`: **GRACEFUL_HAMMER_TIME + 30s**: Timeout the creation of the wrapped queue if it takes longer than this to create.
- Queues by default come with a dynamically scaling worker pool. The following settings configure this:
- `WORKERS`: **0** (v1.14 and before: **1**): Number of initial workers for the queue.
- `MAX_WORKERS`: **10**: Maximum number of worker go-routines for the queue.
- `BLOCK_TIMEOUT`: **1s**: If the queue blocks for this time, boost the number of workers - the `BLOCK_TIMEOUT` will then be doubled before boosting again whilst the boost is ongoing.
- `BOOST_TIMEOUT`: **5m**: Boost workers will timeout after this long.
- `BOOST_WORKERS`: **1** (v1.14 and before: **5**): This many workers will be added to the worker pool if there is a boost.

## Admin (`admin`)

- `DEFAULT_EMAIL_NOTIFICATIONS`: **enabled**: Default configuration for email notifications for users (user configurable). Options: enabled, onmention, disabled
- `DISABLE_REGULAR_ORG_CREATION`: **false**: Disallow regular (non-admin) users from creating organizations.

## Security (`security`)

- `INSTALL_LOCK`: **false**: Disallow access to the install page.
- `SECRET_KEY`: **\<random at every install\>**: Global secret key. This should be changed.
- `LOGIN_REMEMBER_DAYS`: **7**: Cookie lifetime, in days.
- `COOKIE_USERNAME`: **gitea\_awesome**: Name of the cookie used to store the current username.
- `COOKIE_REMEMBER_NAME`: **gitea\_incredible**: Name of cookie used to store authentication
   information.
- `REVERSE_PROXY_AUTHENTICATION_USER`: **X-WEBAUTH-USER**: Header name for reverse proxy
   authentication.
- `REVERSE_PROXY_AUTHENTICATION_EMAIL`: **X-WEBAUTH-EMAIL**: Header name for reverse proxy
   authentication provided email.
- `REVERSE_PROXY_LIMIT`: **1**: Interpret X-Forwarded-For header or the X-Real-IP header and set this as the remote IP for the request.
   Number of trusted proxy count. Set to zero to not use these headers.
- `REVERSE_PROXY_TRUSTED_PROXIES`: **127.0.0.0/8,::1/128**: List of IP addresses and networks separated by comma of trusted proxy servers. Use `*` to trust all.
- `DISABLE_GIT_HOOKS`: **true**: Set to `false` to enable users with git hook privilege to create custom git hooks.
   WARNING: Custom git hooks can be used to perform arbitrary code execution on the host operating system.
   This enables the users to access and modify this config file and the Gitea database and interrupt the Gitea service.
   By modifying the Gitea database, users can gain Gitea administrator privileges.
   It also enables them to access other resources available to the user on the operating system that is running the
   Gitea instance and perform arbitrary actions in the name of the Gitea OS user.
   This maybe harmful to you website or your operating system.
- `DISABLE_WEBHOOKS`: **false**: Set to `true` to disable webhooks feature.
- `ONLY_ALLOW_PUSH_IF_GITEA_ENVIRONMENT_SET`: **true**: Set to `false` to allow local users to push to gitea-repositories without setting up the Gitea environment. This is not recommended and if you want local users to push to gitea repositories you should set the environment appropriately.
- `IMPORT_LOCAL_PATHS`: **false**: Set to `false` to prevent all users (including admin) from importing local path on server.
- `INTERNAL_TOKEN`: **\<random at every install if no uri set\>**: Secret used to validate communication within Gitea binary.
- `INTERNAL_TOKEN_URI`: **<empty>**: Instead of defining internal token in the configuration, this configuration option can be used to give Gitea a path to a file that contains the internal token (example value: `file:/etc/gitea/internal_token`)
- `PASSWORD_HASH_ALGO`: **pbkdf2**: The hash algorithm to use \[argon2, pbkdf2, scrypt, bcrypt\], argon2 will spend more memory than others.
- `CSRF_COOKIE_HTTP_ONLY`: **true**: Set false to allow JavaScript to read CSRF cookie.
- `MIN_PASSWORD_LENGTH`: **6**: Minimum password length for new users.
- `PASSWORD_COMPLEXITY`: **off**: Comma separated list of character classes required to pass minimum complexity. If left empty or no valid values are specified, checking is disabled (off):
    - lower - use one or more lower latin characters
    - upper - use one or more upper latin characters
    - digit - use one or more digits
    - spec - use one or more special characters as ``!"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~``
    - off - do not check password complexity
- `PASSWORD_CHECK_PWN`: **false**: Check [HaveIBeenPwned](https://haveibeenpwned.com/Passwords) to see if a password has been exposed.

## OpenID (`openid`)

- `ENABLE_OPENID_SIGNIN`: **false**: Allow authentication in via OpenID.
- `ENABLE_OPENID_SIGNUP`: **! DISABLE\_REGISTRATION**: Allow registering via OpenID.
- `WHITELISTED_URIS`: **\<empty\>**: If non-empty, list of POSIX regex patterns matching
   OpenID URI's to permit.
- `BLACKLISTED_URIS`: **\<empty\>**: If non-empty, list of POSIX regex patterns matching
   OpenID URI's to block.

## OAuth2 Client (`oauth2_client`)

- `REGISTER_EMAIL_CONFIRM`: *[service]* **REGISTER\_EMAIL\_CONFIRM**: Set this to enable or disable email confirmation of OAuth2 auto-registration. (Overwrites the REGISTER\_EMAIL\_CONFIRM setting of the `[service]` section)
- `OPENID_CONNECT_SCOPES`: **\<empty\>**: List of additional openid connect scopes. (`openid` is implicitly added)
- `ENABLE_AUTO_REGISTRATION`: **false**: Automatically create user accounts for new oauth2 users.
- `USERNAME`: **nickname**: The source of the username for new oauth2 accounts:
    - userid - use the userid / sub attribute
    - nickname - use the nickname attribute
    - email - use the username part of the email attribute
- `UPDATE_AVATAR`: **false**: Update avatar if available from oauth2 provider. Update will be performed on each login.
- `ACCOUNT_LINKING`: **login**: How to handle if an account / email already exists:
    - disabled - show an error
    - login - show an account linking login
    - auto - automatically link with the account (Please be aware that this will grant access to an existing account just because the same username or email is provided. You must make sure that this does not cause issues with your authentication providers.)

## Service (`service`)

- `ACTIVE_CODE_LIVE_MINUTES`: **180**: Time limit (min) to confirm account/email registration.
- `RESET_PASSWD_CODE_LIVE_MINUTES`: **180**: Time limit (min) to confirm forgot password reset
   process.
- `REGISTER_EMAIL_CONFIRM`: **false**: Enable this to ask for mail confirmation of registration.
   Requires `Mailer` to be enabled.
- `REGISTER_MANUAL_CONFIRM`: **false**: Enable this to manually confirm new registrations.
   Requires `REGISTER_EMAIL_CONFIRM` to be disabled.
- `DISABLE_REGISTRATION`: **false**: Disable registration, after which only admin can create
   accounts for users.
- `REQUIRE_EXTERNAL_REGISTRATION_PASSWORD`: **false**: Enable this to force externally created
   accounts (via GitHub, OpenID Connect, etc) to create a password. Warning: enabling this will
   decrease security, so you should only enable it if you know what you're doing.
- `REQUIRE_SIGNIN_VIEW`: **false**: Enable this to force users to log in to view any page or to use API.
- `ENABLE_NOTIFY_MAIL`: **false**: Enable this to send e-mail to watchers of a repository when
   something happens, like creating issues. Requires `Mailer` to be enabled.
- `ENABLE_BASIC_AUTHENTICATION`: **true**: Disable this to disallow authenticaton using HTTP
   BASIC and the user's password. Please note if you disable this you will not be able to access the
   tokens API endpoints using a password. Further, this only disables BASIC authentication using the
   password - not tokens or OAuth Basic.
- `ENABLE_REVERSE_PROXY_AUTHENTICATION`: **false**: Enable this to allow reverse proxy authentication.
- `ENABLE_REVERSE_PROXY_AUTO_REGISTRATION`: **false**: Enable this to allow auto-registration
   for reverse authentication.
- `ENABLE_REVERSE_PROXY_EMAIL`: **false**: Enable this to allow to auto-registration with a
   provided email rather than a generated email.
- `ENABLE_CAPTCHA`: **false**: Enable this to use captcha validation for registration.
- `REQUIRE_EXTERNAL_REGISTRATION_CAPTCHA`: **false**: Enable this to force captcha validation
   even for External Accounts (i.e. GitHub, OpenID Connect, etc). You must `ENABLE_CAPTCHA` also.
- `CAPTCHA_TYPE`: **image**: \[image, recaptcha, hcaptcha\]
- `RECAPTCHA_SECRET`: **""**: Go to https://www.google.com/recaptcha/admin to get a secret for recaptcha.
- `RECAPTCHA_SITEKEY`: **""**: Go to https://www.google.com/recaptcha/admin to get a sitekey for recaptcha.
- `RECAPTCHA_URL`: **https://www.google.com/recaptcha/**: Set the recaptcha url - allows the use of recaptcha net.
- `HCAPTCHA_SECRET`: **""**: Sign up at https://www.hcaptcha.com/ to get a secret for hcaptcha.
- `HCAPTCHA_SITEKEY`: **""**: Sign up at https://www.hcaptcha.com/ to get a sitekey for hcaptcha.
- `DEFAULT_KEEP_EMAIL_PRIVATE`: **false**: By default set users to keep their email address private.
- `DEFAULT_ALLOW_CREATE_ORGANIZATION`: **true**: Allow new users to create organizations by default.
- `DEFAULT_ENABLE_DEPENDENCIES`: **true**: Enable this to have dependencies enabled by default.
- `ALLOW_CROSS_REPOSITORY_DEPENDENCIES` : **true** Enable this to allow dependencies on issues from any repository where the user is granted access.
- `ENABLE_USER_HEATMAP`: **true**: Enable this to display the heatmap on users profiles.
- `ENABLE_TIMETRACKING`: **true**: Enable Timetracking feature.
- `DEFAULT_ENABLE_TIMETRACKING`: **true**: Allow repositories to use timetracking by deault.
- `DEFAULT_ALLOW_ONLY_CONTRIBUTORS_TO_TRACK_TIME`: **true**: Only allow users with write permissions to track time.
- `EMAIL_DOMAIN_WHITELIST`: **\<empty\>**: If non-empty, list of domain names that can only be used to register
  on this instance.
- `EMAIL_DOMAIN_BLOCKLIST`: **\<empty\>**: If non-empty, list of domain names that cannot be used to register on this instance
- `SHOW_REGISTRATION_BUTTON`: **! DISABLE\_REGISTRATION**: Show Registration Button
- `SHOW_MILESTONES_DASHBOARD_PAGE`: **true** Enable this to show the milestones dashboard page - a view of all the user's milestones
- `AUTO_WATCH_NEW_REPOS`: **true**: Enable this to let all organisation users watch new repos when they are created
- `AUTO_WATCH_ON_CHANGES`: **false**: Enable this to make users watch a repository after their first commit to it
- `DEFAULT_USER_VISIBILITY`: **public**: Set default visibility mode for users, either "public", "limited" or "private".
- `ALLOWED_USER_VISIBILITY_MODES`: **public,limited,private**: Set which visibility modes a user can have
- `DEFAULT_ORG_VISIBILITY`: **public**: Set default visibility mode for organisations, either "public", "limited" or "private".
- `DEFAULT_ORG_MEMBER_VISIBLE`: **false** True will make the membership of the users visible when added to the organisation.
- `ALLOW_ONLY_INTERNAL_REGISTRATION`: **false** Set to true to force registration only via gitea.
- `ALLOW_ONLY_EXTERNAL_REGISTRATION`: **false** Set to true to force registration only using third-party services.
- `NO_REPLY_ADDRESS`: **noreply.DOMAIN** Value for the domain part of the user's email address in the git log if user has set KeepEmailPrivate to true. DOMAIN resolves to the value in server.DOMAIN.
  The user's email will be replaced with a concatenation of the user name in lower case, "@" and NO_REPLY_ADDRESS.
- `USER_DELETE_WITH_COMMENTS_MAX_TIME`: **0** Minimum amount of time a user must exist before comments are kept when the user is deleted.
- `VALID_SITE_URL_SCHEMES`: **http, https**: Valid site url schemes for user profiles

### Service - Expore (`service.explore`)

- `REQUIRE_SIGNIN_VIEW`: **false**: Only allow signed in users to view the explore pages.
- `DISABLE_USERS_PAGE`: **false**: Disable the users explore page.


## SSH Minimum Key Sizes (`ssh.minimum_key_sizes`)

Define allowed algorithms and their minimum key length (use -1 to disable a type):

- `ED25519`: **256**
- `ECDSA`: **256**
- `RSA`: **2048**
- `DSA`: **-1**: DSA is now disabled by default. Set to **1024** to re-enable but ensure you may need to reconfigure your SSHD provider

## Webhook (`webhook`)

- `QUEUE_LENGTH`: **1000**: Hook task queue length. Use caution when editing this value.
- `DELIVER_TIMEOUT`: **5**: Delivery timeout (sec) for shooting webhooks.
- `SKIP_TLS_VERIFY`: **false**: Allow insecure certification.
- `PAGING_NUM`: **10**: Number of webhook history events that are shown in one page.
- `PROXY_URL`: ****: Proxy server URL, support http://, https//, socks://, blank will follow environment http_proxy/https_proxy
- `PROXY_HOSTS`: ****: Comma separated list of host names requiring proxy. Glob patterns (*) are accepted; use ** to match all hosts.

## Mailer (`mailer`)

- `ENABLED`: **false**: Enable to use a mail service.
- `DISABLE_HELO`: **\<empty\>**: Disable HELO operation.
- `HELO_HOSTNAME`: **\<empty\>**: Custom hostname for HELO operation.
- `HOST`: **\<empty\>**: SMTP mail host address and port (example: smtp.gitea.io:587).
  - As per RFC 8314, if supported, Implicit TLS/SMTPS on port 465 is recommended, otherwise opportunistic TLS via STARTTLS on port 587 should be used.
- `IS_TLS_ENABLED` :  **false** : Forcibly use TLS to connect even if not on a default SMTPS port.
  - Note, if the port ends with `465` Implicit TLS/SMTPS/SMTP over TLS will be used despite this setting.
  - Otherwise if `IS_TLS_ENABLED=false` and the server supports `STARTTLS` this will be used. Thus if `STARTTLS` is preferred you should set `IS_TLS_ENABLED=false`.
- `FROM`: **\<empty\>**: Mail from address, RFC 5322. This can be just an email address, or
   the "Name" \<email@example.com\> format.
- `USER`: **\<empty\>**: Username of mailing user (usually the sender's e-mail address).
- `PASSWD`: **\<empty\>**: Password of mailing user.  Use \`your password\` for quoting if you use special characters in the password.
   - Please note: authentication is only supported when the SMTP server communication is encrypted with TLS (this can be via `STARTTLS`) or `HOST=localhost`. See [Email Setup]({{< relref "doc/usage/email-setup.en-us.md" >}}) for more information.
- `SEND_AS_PLAIN_TEXT`: **false**: Send mails as plain text.
- `SKIP_VERIFY`: **false**: Whether or not to skip verification of certificates; `true` to disable verification.
   - **Warning:** This option is unsafe. Consider adding the certificate to the system trust store instead.
   - **Note:** Gitea only supports SMTP with STARTTLS.
- `USE_CERTIFICATE`: **false**: Use client certificate.
- `CERT_FILE`: **custom/mailer/cert.pem**
- `KEY_FILE`: **custom/mailer/key.pem**
- `SUBJECT_PREFIX`: **\<empty\>**: Prefix to be placed before e-mail subject lines.
- `MAILER_TYPE`: **smtp**: \[smtp, sendmail, dummy\]
   - **smtp** Use SMTP to send mail
   - **sendmail** Use the operating system's `sendmail` command instead of SMTP.
   This is common on linux systems.
   - **dummy** Send email messages to the log as a testing phase.
   - Note that enabling sendmail will ignore all other `mailer` settings except `ENABLED`,
     `FROM`, `SUBJECT_PREFIX` and `SENDMAIL_PATH`.
   - Enabling dummy will ignore all settings except `ENABLED`, `SUBJECT_PREFIX` and `FROM`.
- `SENDMAIL_PATH`: **sendmail**: The location of sendmail on the operating system (can be
   command or full path).
- `SENDMAIL_ARGS`: **_empty_**: Specify any extra sendmail arguments.
- `SENDMAIL_TIMEOUT`: **5m**: default timeout for sending email through sendmail
- `SEND_BUFFER_LEN`: **100**: Buffer length of mailing queue.

## Cache (`cache`)

- `ENABLED`: **true**: Enable the cache.
- `ADAPTER`: **memory**: Cache engine adapter, either `memory`, `redis`, or `memcache`.
- `INTERVAL`: **60**: Garbage Collection interval (sec), for memory cache only.
- `HOST`: **\<empty\>**: Connection string for `redis` and `memcache`.
   - Redis: `redis://:macaron@127.0.0.1:6379/0?pool_size=100&idle_timeout=180s`
   - Memcache: `127.0.0.1:9090;127.0.0.1:9091`
- `ITEM_TTL`: **16h**: Time to keep items in cache if not used, Setting it to 0 disables caching.

## Cache - LastCommitCache settings (`cache.last_commit`)

- `ENABLED`: **true**: Enable the cache.
- `ITEM_TTL`: **8760h**: Time to keep items in cache if not used, Setting it to 0 disables caching.
- `COMMITS_COUNT`: **1000**: Only enable the cache when repository's commits count great than.

## Session (`session`)

- `PROVIDER`: **memory**: Session engine provider \[memory, file, redis, db, mysql, couchbase, memcache, postgres\].
- `PROVIDER_CONFIG`: **data/sessions**: For file, the root path; for db, empty (database config will be used); for others, the connection string.
- `COOKIE_SECURE`: **false**: Enable this to force using HTTPS for all session access.
- `COOKIE_NAME`: **i\_like\_gitea**: The name of the cookie used for the session ID.
- `GC_INTERVAL_TIME`: **86400**: GC interval in seconds.
- `SESSION_LIFE_TIME`: **86400**: Session life time in seconds, default is 86400 (1 day)
- `DOMAIN`: **\<empty\>**: Sets the cookie Domain
- `SAME_SITE`: **lax** \[strict, lax, none\]: Set the SameSite setting for the cookie.

## Picture (`picture`)

- `GRAVATAR_SOURCE`: **gravatar**: Can be `gravatar`, `duoshuo` or anything like
   `http://cn.gravatar.com/avatar/`.
- `DISABLE_GRAVATAR`: **false**: Enable this to use local avatars only.
- `ENABLE_FEDERATED_AVATAR`: **false**: Enable support for federated avatars (see
   [http://www.libravatar.org](http://www.libravatar.org)).

- `AVATAR_STORAGE_TYPE`: **default**: Storage type defined in `[storage.xxx]`. Default is `default` which will read `[storage]` if no section `[storage]` will be a type `local`.
- `AVATAR_UPLOAD_PATH`: **data/avatars**: Path to store user avatar image files.
- `AVATAR_MAX_WIDTH`: **4096**: Maximum avatar image width in pixels.
- `AVATAR_MAX_HEIGHT`: **3072**: Maximum avatar image height in pixels.
- `AVATAR_MAX_FILE_SIZE`: **1048576** (1Mb): Maximum avatar image file size in bytes.

- `REPOSITORY_AVATAR_STORAGE_TYPE`: **default**: Storage type defined in `[storage.xxx]`. Default is `default` which will read `[storage]` if no section `[storage]` will be a type `local`.
- `REPOSITORY_AVATAR_UPLOAD_PATH`: **data/repo-avatars**: Path to store repository avatar image files.
- `REPOSITORY_AVATAR_FALLBACK`: **none**: How Gitea deals with missing repository avatars
  - none = no avatar will be displayed
  - random = random avatar will be generated
  - image = default image will be used (which is set in `REPOSITORY_AVATAR_FALLBACK_IMAGE`)
- `REPOSITORY_AVATAR_FALLBACK_IMAGE`: **/img/repo_default.png**: Image used as default repository avatar (if `REPOSITORY_AVATAR_FALLBACK` is set to image and none was uploaded)


## Project (`project`)

Default templates for project boards:

- `PROJECT_BOARD_BASIC_KANBAN_TYPE`: **To Do, In Progress, Done**
- `PROJECT_BOARD_BUG_TRIAGE_TYPE`: **Needs Triage, High Priority, Low Priority, Closed**

## Issue and pull request attachments (`attachment`)

- `ENABLED`: **true**: Whether issue and pull request attachments are enabled.
- `ALLOWED_TYPES`: **.docx,.gif,.gz,.jpeg,.jpg,.log,.pdf,.png,.pptx,.txt,.xlsx,.zip**: Comma-separated list of allowed file extensions (`.zip`), mime types (`text/plain`) or wildcard type (`image/*`, `audio/*`, `video/*`). Empty value or `*/*` allows all types.
- `MAX_SIZE`: **4**: Maximum size (MB).
- `MAX_FILES`: **5**: Maximum number of attachments that can be uploaded at once.
- `STORAGE_TYPE`: **local**: Storage type for attachments, `local` for local disk or `minio` for s3 compatible object storage service, default is `local` or other name defined with `[storage.xxx]`
- `SERVE_DIRECT`: **false**: Allows the storage driver to redirect to authenticated URLs to serve files directly. Currently, only Minio/S3 is supported via signed URLs, local does nothing.
- `PATH`: **data/attachments**: Path to store attachments only available when STORAGE_TYPE is `local`
- `MINIO_ENDPOINT`: **localhost:9000**: Minio endpoint to connect only available when STORAGE_TYPE is `minio`
- `MINIO_ACCESS_KEY_ID`: Minio accessKeyID to connect only available when STORAGE_TYPE is `minio`
- `MINIO_SECRET_ACCESS_KEY`: Minio secretAccessKey to connect only available when STORAGE_TYPE is `minio`
- `MINIO_BUCKET`: **gitea**: Minio bucket to store the attachments only available when STORAGE_TYPE is `minio`
- `MINIO_LOCATION`: **us-east-1**: Minio location to create bucket only available when STORAGE_TYPE is `minio`
- `MINIO_BASE_PATH`: **attachments/**: Minio base path on the bucket only available when STORAGE_TYPE is `minio`
- `MINIO_USE_SSL`: **false**: Minio enabled ssl only available when STORAGE_TYPE is `minio`

## Log (`log`)

- `ROOT_PATH`: **\<empty\>**: Root path for log files.
- `MODE`: **console**: Logging mode. For multiple modes, use a comma to separate values. You can configure each mode in per mode log subsections `\[log.modename\]`. By default the file mode will log to `$ROOT_PATH/gitea.log`.
- `LEVEL`: **Info**: General log level. \[Trace, Debug, Info, Warn, Error, Critical, Fatal, None\]
- `STACKTRACE_LEVEL`: **None**: Default log level at which to log create stack traces. \[Trace, Debug, Info, Warn, Error, Critical, Fatal, None\]
- `ROUTER_LOG_LEVEL`: **Info**: The log level that the router should log at. (If you are setting the access log, its recommended to place this at Debug.)
- `ROUTER`: **console**: The mode or name of the log the router should log to. (If you set this to `,` it will log to default gitea logger.)
NB: You must have `DISABLE_ROUTER_LOG` set to `false` for this option to take effect. Configure each mode in per mode log subsections `\[log.modename.router\]`.
- `ENABLE_ACCESS_LOG`: **false**: Creates an access.log in NCSA common log format, or as per the following template
- `ENABLE_SSH_LOG`: **false**: save ssh log to log file
- `ACCESS`: **file**: Logging mode for the access logger, use a comma to separate values. Configure each mode in per mode log subsections `\[log.modename.access\]`. By default the file mode will log to `$ROOT_PATH/access.log`. (If you set this to `,` it will log to the default gitea logger.)
- `ACCESS_LOG_TEMPLATE`: **`{{.Ctx.RemoteAddr}} - {{.Identity}} {{.Start.Format "[02/Jan/2006:15:04:05 -0700]" }} "{{.Ctx.Req.Method}} {{.Ctx.Req.URL.RequestURI}} {{.Ctx.Req.Proto}}" {{.ResponseWriter.Status}} {{.ResponseWriter.Size}} "{{.Ctx.Req.Referer}}\" \"{{.Ctx.Req.UserAgent}}"`**: Sets the template used to create the access log.
  - The following variables are available:
  - `Ctx`: the `context.Context` of the request.
  - `Identity`: the SignedUserName or `"-"` if not logged in.
  - `Start`: the start time of the request.
  - `ResponseWriter`: the responseWriter from the request.
  - You must be very careful to ensure that this template does not throw errors or panics as this template runs outside of the panic/recovery script.
- `ENABLE_XORM_LOG`: **true**: Set whether to perform XORM logging. Please note SQL statement logging can be disabled by setting `LOG_SQL` to false in the `[database]` section.

### Log subsections (`log.name`, `log.name.*`)

- `LEVEL`: **log.LEVEL**: Sets the log-level of this sublogger. Defaults to the `LEVEL` set in the global `[log]` section.
- `STACKTRACE_LEVEL`: **log.STACKTRACE_LEVEL**: Sets the log level at which to log stack traces.
- `MODE`: **name**: Sets the mode of this sublogger - Defaults to the provided subsection name. This allows you to have two different file loggers at different levels.
- `EXPRESSION`: **""**: A regular expression to match either the function name, file or message. Defaults to empty. Only log messages that match the expression will be saved in the logger.
- `FLAGS`: **stdflags**: A comma separated string representing the log flags. Defaults to `stdflags` which represents the prefix: `2009/01/23 01:23:23 ...a/b/c/d.go:23:runtime.Caller() [I]: message`. `none` means don't prefix log lines. See `modules/log/base.go` for more information.
- `PREFIX`: **""**: An additional prefix for every log line in this logger. Defaults to empty.
- `COLORIZE`: **false**: Colorize the log lines by default

### Console log mode (`log.console`, `log.console.*`, or `MODE=console`)

- For the console logger `COLORIZE` will default to `true` if not on windows or the terminal is determined to be able to color.
- `STDERR`: **false**: Use Stderr instead of Stdout.

### File log mode (`log.file`, `log.file.*` or `MODE=file`)

- `FILE_NAME`: Set the file name for this logger. Defaults as described above. If relative will be relative to the `ROOT_PATH`
- `LOG_ROTATE`: **true**: Rotate the log files.
- `MAX_SIZE_SHIFT`: **28**: Maximum size shift of a single file, 28 represents 256Mb.
- `DAILY_ROTATE`: **true**: Rotate logs daily.
- `MAX_DAYS`: **7**: Delete the log file after n days
- `COMPRESS`: **true**: Compress old log files by default with gzip
- `COMPRESSION_LEVEL`: **-1**: Compression level

### Conn log mode (`log.conn`, `log.conn.*` or `MODE=conn`)

- `RECONNECT_ON_MSG`: **false**: Reconnect host for every single message.
- `RECONNECT`: **false**: Try to reconnect when connection is lost.
- `PROTOCOL`: **tcp**: Set the protocol, either "tcp", "unix" or "udp".
- `ADDR`: **:7020**: Sets the address to connect to.

### SMTP log mode (`log.smtp`, `log.smtp.*` or `MODE=smtp`)

- `USER`: User email address to send from.
- `PASSWD`: Password for the smtp server.
- `HOST`: **127.0.0.1:25**: The SMTP host to connect to.
- `RECEIVERS`: Email addresses to send to.
- `SUBJECT`: **Diagnostic message from Gitea**

## Cron (`cron`)

- `ENABLED`: **false**: Enable to run all cron tasks periodically with default settings.
- `RUN_AT_START`: **false**: Run cron tasks at application start-up.
- `NO_SUCCESS_NOTICE`: **false**: Set to true to switch off success notices.

- `SCHEDULE` accept formats
   - Full crontab specs, e.g. `* * * * * ?`
   - Descriptors, e.g. `@midnight`, `@every 1h30m` ...
   - See more: [cron decument](https://pkg.go.dev/github.com/gogs/cron@v0.0.0-20171120032916-9f6c956d3e14)

### Basic cron tasks - enabled by default

#### Cron - Cleanup old repository archives (`cron.archive_cleanup`)

- `ENABLED`: **true**: Enable service.
- `RUN_AT_START`: **true**: Run tasks at start up time (if ENABLED).
- `SCHEDULE`: **@every 24h**: Cron syntax for scheduling repository archive cleanup, e.g. `@every 1h`.
- `OLDER_THAN`: **24h**: Archives created more than `OLDER_THAN` ago are subject to deletion, e.g. `12h`.

#### Cron - Update Mirrors (`cron.update_mirrors`)

- `SCHEDULE`: **@every 10m**: Cron syntax for scheduling update mirrors, e.g. `@every 3h`.
- `NO_SUCCESS_NOTICE`: **true**: The cron task for update mirrors success report is not very useful - as it just means that the mirrors have been queued. Therefore this is turned off by default.

#### Cron - Repository Health Check (`cron.repo_health_check`)

- `SCHEDULE`: **@every 24h**: Cron syntax for scheduling repository health check.
- `TIMEOUT`: **60s**: Time duration syntax for health check execution timeout.
- `ARGS`: **\<empty\>**: Arguments for command `git fsck`, e.g. `--unreachable --tags`. See more on http://git-scm.com/docs/git-fsck

#### Cron - Repository Statistics Check (`cron.check_repo_stats`)

- `RUN_AT_START`: **true**: Run repository statistics check at start time.
- `SCHEDULE`: **@every 24h**: Cron syntax for scheduling repository statistics check.

### Cron - Cleanup hook_task Table (`cron.cleanup_hook_task_table`)

- `ENABLED`: **true**: Enable cleanup hook_task job.
- `RUN_AT_START`: **false**: Run cleanup hook_task at start time (if ENABLED).
- `SCHEDULE`: **@every 24h**: Cron syntax for cleaning hook_task table.
- `CLEANUP_TYPE` **OlderThan** OlderThan or PerWebhook Method to cleanup hook_task, either by age (i.e. how long ago hook_task record was delivered) or by the number to keep per webhook (i.e. keep most recent x deliveries per webhook).
- `OLDER_THAN`: **168h**: If CLEANUP_TYPE is set to OlderThan, then any delivered hook_task records older than this expression will be deleted.
- `NUMBER_TO_KEEP`: **10**: If CLEANUP_TYPE is set to PerWebhook, this is number of hook_task records to keep for a webhook (i.e. keep the most recent x deliveries).

#### Cron - Update Migration Poster ID (`cron.update_migration_poster_id`)

- `SCHEDULE`: **@every 24h** : Interval as a duration between each synchronization, it will always attempt synchronization when the instance starts.

#### Cron - Sync External Users (`cron.sync_external_users`)

- `SCHEDULE`: **@every 24h** : Interval as a duration between each synchronization, it will always attempt synchronization when the instance starts.
- `UPDATE_EXISTING`: **true**: Create new users, update existing user data and disable users that are not in external source anymore (default) or only create new users if UPDATE_EXISTING is set to false.

### Extended cron tasks (not enabled by default)

#### Cron - Garbage collect all repositories ('cron.git_gc_repos')
- `ENABLED`: **false**: Enable service.
- `RUN_AT_START`: **false**: Run tasks at start up time (if ENABLED).
- `SCHEDULE`: **@every 72h**: Cron syntax for scheduling repository archive cleanup, e.g. `@every 1h`.
- `TIMEOUT`: **60s**: Time duration syntax for garbage collection execution timeout.
- `NO_SUCCESS_NOTICE`: **false**: Set to true to switch off success notices.
- `ARGS`: **\<empty\>**: Arguments for command `git gc`, e.g. `--aggressive --auto`. The default value is same with [git] -> GC_ARGS

#### Cron - Update the '.ssh/authorized_keys' file with Gitea SSH keys ('cron.resync_all_sshkeys')
- `ENABLED`: **false**: Enable service.
- `RUN_AT_START`: **false**: Run tasks at start up time (if ENABLED).
- `NO_SUCCESS_NOTICE`: **false**: Set to true to switch off success notices.
- `SCHEDULE`: **@every 72h**: Cron syntax for scheduling repository archive cleanup, e.g. `@every 1h`.

#### Cron - Resynchronize pre-receive, update and post-receive hooks of all repositories ('cron.resync_all_hooks')
- `ENABLED`: **false**: Enable service.
- `RUN_AT_START`: **false**: Run tasks at start up time (if ENABLED).
- `NO_SUCCESS_NOTICE`: **false**: Set to true to switch off success notices.
- `SCHEDULE`: **@every 72h**: Cron syntax for scheduling repository archive cleanup, e.g. `@every 1h`.

#### Cron - Reinitialize all missing Git repositories for which records exist ('cron.reinit_missing_repos')
- `ENABLED`: **false**: Enable service.
- `RUN_AT_START`: **false**: Run tasks at start up time (if ENABLED).
- `NO_SUCCESS_NOTICE`: **false**: Set to true to switch off success notices.
- `SCHEDULE`: **@every 72h**: Cron syntax for scheduling repository archive cleanup, e.g. `@every 1h`.

#### Cron - Delete all repositories missing their Git files ('cron.delete_missing_repos')
- `ENABLED`: **false**: Enable service.
- `RUN_AT_START`: **false**: Run tasks at start up time (if ENABLED).
- `NO_SUCCESS_NOTICE`: **false**: Set to true to switch off success notices.
- `SCHEDULE`: **@every 72h**: Cron syntax for scheduling repository archive cleanup, e.g. `@every 1h`.

#### Cron -  Delete generated repository avatars ('cron.delete_generated_repository_avatars')
- `ENABLED`: **false**: Enable service.
- `RUN_AT_START`: **false**: Run tasks at start up time (if ENABLED).
- `NO_SUCCESS_NOTICE`: **false**: Set to true to switch off success notices.
- `SCHEDULE`: **@every 72h**: Cron syntax for scheduling repository archive cleanup, e.g. `@every 1h`.

#### Cron -  Delete all old actions from database ('cron.delete_old_actions')
- `ENABLED`: **false**: Enable service.
- `RUN_AT_START`: **false**: Run tasks at start up time (if ENABLED).
- `NO_SUCCESS_NOTICE`: **false**: Set to true to switch off success notices.
- `SCHEDULE`: **@every 128h**: Cron syntax for scheduling a work, e.g. `@every 128h`.
- `OLDER_THAN`: **@every 8760h**: any action older than this expression will be deleted from database, suggest using `8760h` (1 year) because that's the max length of heatmap.

## Git (`git`)

- `PATH`: **""**: The path of git executable. If empty, Gitea searches through the PATH environment.
- `DISABLE_DIFF_HIGHLIGHT`: **false**: Disables highlight of added and removed changes.
- `MAX_GIT_DIFF_LINES`: **1000**: Max number of lines allowed of a single file in diff view.
- `MAX_GIT_DIFF_LINE_CHARACTERS`: **5000**: Max character count per line highlighted in diff view.
- `MAX_GIT_DIFF_FILES`: **100**: Max number of files shown in diff view.
- `COMMITS_RANGE_SIZE`: **50**: Set the default commits range size
- `BRANCHES_RANGE_SIZE`: **20**: Set the default branches range size
- `GC_ARGS`: **\<empty\>**: Arguments for command `git gc`, e.g. `--aggressive --auto`. See more on http://git-scm.com/docs/git-gc/
- `ENABLE_AUTO_GIT_WIRE_PROTOCOL`: **true**: If use git wire protocol version 2 when git version >= 2.18, default is true, set to false when you always want git wire protocol version 1
- `PULL_REQUEST_PUSH_MESSAGE`: **true**: Respond to pushes to a non-default branch with a URL for creating a Pull Request (if the repository has them enabled)
- `VERBOSE_PUSH`: **true**: Print status information about pushes as they are being processed.
- `VERBOSE_PUSH_DELAY`: **5s**: Only print verbose information if push takes longer than this delay.
- `LARGE_OBJECT_THRESHOLD`: **1048576**: (Go-Git only), don't cache objects greater than this in memory. (Set to 0 to disable.)
## Git - Timeout settings (`git.timeout`)
- `DEFAUlT`: **360**: Git operations default timeout seconds.
- `MIGRATE`: **600**: Migrate external repositories timeout seconds.
- `MIRROR`: **300**: Mirror external repositories timeout seconds.
- `CLONE`: **300**: Git clone from internal repositories timeout seconds.
- `PULL`: **300**: Git pull from internal repositories timeout seconds.
- `GC`: **60**: Git repository GC timeout seconds.

## Metrics (`metrics`)

- `ENABLED`: **false**: Enables /metrics endpoint for prometheus.
- `TOKEN`: **\<empty\>**: You need to specify the token, if you want to include in the authorization the metrics . The same token need to be used in prometheus parameters `bearer_token` or `bearer_token_file`.

## API (`api`)

- `ENABLE_SWAGGER`: **true**: Enables /api/swagger, /api/v1/swagger etc. endpoints. True or false; default is true.
- `MAX_RESPONSE_ITEMS`: **50**: Max number of items in a page.
- `DEFAULT_PAGING_NUM`: **30**: Default paging number of API.
- `DEFAULT_GIT_TREES_PER_PAGE`: **1000**: Default and maximum number of items per page for git trees API.
- `DEFAULT_MAX_BLOB_SIZE`: **10485760**: Default max size of a blob that can be return by the blobs API.

## OAuth2 (`oauth2`)

- `ENABLE`: **true**: Enables OAuth2 provider.
- `ACCESS_TOKEN_EXPIRATION_TIME`: **3600**: Lifetime of an OAuth2 access token in seconds
- `REFRESH_TOKEN_EXPIRATION_TIME`: **730**: Lifetime of an OAuth2 refresh token in hours
- `INVALIDATE_REFRESH_TOKENS`: **false**: Check if refresh token has already been used
- `JWT_SIGNING_ALGORITHM`: **RS256**: Algorithm used to sign OAuth2 tokens. Valid values: \[`HS256`, `HS384`, `HS512`, `RS256`, `RS384`, `RS512`, `ES256`, `ES384`, `ES512`\]
- `JWT_SECRET`: **\<empty\>**: OAuth2 authentication secret for access and refresh tokens, change this to a unique string. This setting is only needed if `JWT_SIGNING_ALGORITHM` is set to `HS256`, `HS384` or `HS512`.
- `JWT_SIGNING_PRIVATE_KEY_FILE`: **jwt/private.pem**: Private key file path used to sign OAuth2 tokens. The path is relative to `APP_DATA_PATH`. This setting is only needed if `JWT_SIGNING_ALGORITHM` is set to `RS256`, `RS384`, `RS512`, `ES256`, `ES384` or `ES512`. The file must contain a RSA or ECDSA private key in the PKCS8 format. If no key exists a 4096 bit key will be created for you.
- `MAX_TOKEN_LENGTH`: **32767**: Maximum length of token/cookie to accept from OAuth2 provider

## i18n (`i18n`)

- `LANGS`: **en-US,zh-CN,zh-HK,zh-TW,de-DE,fr-FR,nl-NL,lv-LV,ru-RU,ja-JP,es-ES,pt-BR,pt-PT,pl-PL,bg-BG,it-IT,fi-FI,tr-TR,cs-CZ,sr-SP,sv-SE,ko-KR**: List of locales shown in language selector
- `NAMES`: **English,简体中文,繁體中文（香港）,繁體中文（台灣）,Deutsch,français,Nederlands,latviešu,русский,日本語,español,português do Brasil,Português de Portugal,polski,български,italiano,suomi,Türkçe,čeština,српски,svenska,한국어**: Visible names corresponding to the locales

## U2F (`U2F`)
- `APP_ID`: **`ROOT_URL`**: Declares the facet of the application. Requires HTTPS.
- `TRUSTED_FACETS`: List of additional facets which are trusted. This is not support by all browsers.

## Markup (`markup`)

Gitea can support Markup using external tools. The example below will add a markup named `asciidoc`.

```ini
[markup.asciidoc]
ENABLED = true
NEED_POSTPROCESS = true
FILE_EXTENSIONS = .adoc,.asciidoc
RENDER_COMMAND = "asciidoc --out-file=- -"
IS_INPUT_FILE = false
```

- ENABLED: **false** Enable markup support; set to **true** to enable this renderer.
- NEED\_POSTPROCESS: **true** set to **true** to replace links / sha1 and etc.
- FILE\_EXTENSIONS: **\<empty\>** List of file extensions that should be rendered by an external
   command. Multiple extensions needs a comma as splitter.
- RENDER\_COMMAND: External command to render all matching extensions.
- IS\_INPUT\_FILE: **false** Input is not a standard input but a file param followed `RENDER_COMMAND`.

Two special environment variables are passed to the render command:
- `GITEA_PREFIX_SRC`, which contains the current URL prefix in the `src` path tree. To be used as prefix for links.
- `GITEA_PREFIX_RAW`, which contains the current URL prefix in the `raw` path tree. To be used as prefix for image paths.


Gitea supports customizing the sanitization policy for rendered HTML. The example below will support KaTeX output from pandoc.

```ini
[markup.sanitizer.TeX]
; Pandoc renders TeX segments as <span>s with the "math" class, optionally
; with "inline" or "display" classes depending on context.
ELEMENT = span
ALLOW_ATTR = class
REGEXP = ^\s*((math(\s+|$)|inline(\s+|$)|display(\s+|$)))+
ALLOW_DATA_URI_IMAGES = true
```

 - `ELEMENT`: The element this policy applies to. Must be non-empty.
 - `ALLOW_ATTR`: The attribute this policy allows. Must be non-empty.
 - `REGEXP`: A regex to match the contents of the attribute against. Must be present but may be empty for unconditional whitelisting of this attribute.
 - `ALLOW_DATA_URI_IMAGES`: **false** Allow data uri images (`<img src="data:image/png;base64,..."/>`).

Multiple sanitisation rules can be defined by adding unique subsections, e.g. `[markup.sanitizer.TeX-2]`.
To apply a sanitisation rules only for a specify external renderer they must use the renderer name, e.g. `[markup.sanitizer.asciidoc.rule-1]`.
If the rule is defined above the renderer ini section or the name does not match a renderer it is applied to every renderer.

## Time (`time`)

- `FORMAT`: Time format to display on UI. i.e. RFC1123 or 2006-01-02 15:04:05
- `DEFAULT_UI_LOCATION`: Default location of time on the UI, so that we can display correct user's time on UI. i.e. Shanghai/Asia

## Task (`task`)

Task queue configuration has been moved to `queue.task`. However, the below configuration values are kept for backwards compatibility:

- `QUEUE_TYPE`: **channel**: Task queue type, could be `channel` or `redis`.
- `QUEUE_LENGTH`: **1000**: Task queue length, available only when `QUEUE_TYPE` is `channel`.
- `QUEUE_CONN_STR`: **redis://127.0.0.1:6379/0**: Task queue connection string, available only when `QUEUE_TYPE` is `redis`. If redis needs a password, use `redis://123@127.0.0.1:6379/0`.

## Migrations (`migrations`)

- `MAX_ATTEMPTS`: **3**: Max attempts per http/https request on migrations.
- `RETRY_BACKOFF`: **3**: Backoff time per http/https request retry (seconds)
- `ALLOWED_DOMAINS`: **\<empty\>**: Domains allowlist for migrating repositories, default is blank. It means everything will be allowed. Multiple domains could be separated by commas.
- `BLOCKED_DOMAINS`: **\<empty\>**: Domains blocklist for migrating repositories, default is blank. Multiple domains could be separated by commas. When `ALLOWED_DOMAINS` is not blank, this option will be ignored.
- `ALLOW_LOCALNETWORKS`: **false**: Allow private addresses defined by RFC 1918, RFC 1122, RFC 4632 and RFC 4291

## Mirror (`mirror`)

- `DEFAULT_INTERVAL`: **8h**: Default interval between each check
- `MIN_INTERVAL`: **10m**: Minimum interval for checking. (Must be >1m).

## LFS (`lfs`)

Storage configuration for lfs data. It will be derived from default `[storage]` or
`[storage.xxx]` when set `STORAGE_TYPE` to `xxx`. When derived, the default of `PATH`
is `data/lfs` and the default of `MINIO_BASE_PATH` is `lfs/`.

- `STORAGE_TYPE`: **local**: Storage type for lfs, `local` for local disk or `minio` for s3 compatible object storage service or other name defined with `[storage.xxx]`
- `SERVE_DIRECT`: **false**: Allows the storage driver to redirect to authenticated URLs to serve files directly. Currently, only Minio/S3 is supported via signed URLs, local does nothing.
- `PATH`: **./data/lfs**: Where to store LFS files, only available when `STORAGE_TYPE` is `local`. If not set it fall back to deprecated LFS_CONTENT_PATH value in [server] section.
- `MINIO_ENDPOINT`: **localhost:9000**: Minio endpoint to connect only available when `STORAGE_TYPE` is `minio`
- `MINIO_ACCESS_KEY_ID`: Minio accessKeyID to connect only available when `STORAGE_TYPE` is `minio`
- `MINIO_SECRET_ACCESS_KEY`: Minio secretAccessKey to connect only available when `STORAGE_TYPE is` `minio`
- `MINIO_BUCKET`: **gitea**: Minio bucket to store the lfs only available when `STORAGE_TYPE` is `minio`
- `MINIO_LOCATION`: **us-east-1**: Minio location to create bucket only available when `STORAGE_TYPE` is `minio`
- `MINIO_BASE_PATH`: **lfs/**: Minio base path on the bucket only available when `STORAGE_TYPE` is `minio`
- `MINIO_USE_SSL`: **false**: Minio enabled ssl only available when `STORAGE_TYPE` is `minio`

## Storage (`storage`)

Default storage configuration for attachments, lfs, avatars and etc.

- `SERVE_DIRECT`: **false**: Allows the storage driver to redirect to authenticated URLs to serve files directly. Currently, only Minio/S3 is supported via signed URLs, local does nothing.
- `MINIO_ENDPOINT`: **localhost:9000**: Minio endpoint to connect only available when `STORAGE_TYPE` is `minio`
- `MINIO_ACCESS_KEY_ID`: Minio accessKeyID to connect only available when `STORAGE_TYPE` is `minio`
- `MINIO_SECRET_ACCESS_KEY`: Minio secretAccessKey to connect only available when `STORAGE_TYPE is` `minio`
- `MINIO_BUCKET`: **gitea**: Minio bucket to store the data only available when `STORAGE_TYPE` is `minio`
- `MINIO_LOCATION`: **us-east-1**: Minio location to create bucket only available when `STORAGE_TYPE` is `minio`
- `MINIO_USE_SSL`: **false**: Minio enabled ssl only available when `STORAGE_TYPE` is `minio`

And you can also define a customize storage like below:

```ini
[storage.my_minio]
STORAGE_TYPE = minio
; Minio endpoint to connect only available when STORAGE_TYPE is `minio`
MINIO_ENDPOINT = localhost:9000
; Minio accessKeyID to connect only available when STORAGE_TYPE is `minio`
MINIO_ACCESS_KEY_ID =
; Minio secretAccessKey to connect only available when STORAGE_TYPE is `minio`
MINIO_SECRET_ACCESS_KEY =
; Minio bucket to store the attachments only available when STORAGE_TYPE is `minio`
MINIO_BUCKET = gitea
; Minio location to create bucket only available when STORAGE_TYPE is `minio`
MINIO_LOCATION = us-east-1
; Minio enabled ssl only available when STORAGE_TYPE is `minio`
MINIO_USE_SSL = false
```

And used by `[attachment]`, `[lfs]` and etc. as `STORAGE_TYPE`.

## Repository Archive Storage (`storage.repo-archive`)

Configuration for repository archive storage. It will inherit from default `[storage]` or
`[storage.xxx]` when set `STORAGE_TYPE` to `xxx`. The default of `PATH`
is `data/repo-archive` and the default of `MINIO_BASE_PATH` is `repo-archive/`.

- `STORAGE_TYPE`: **local**: Storage type for repo archive, `local` for local disk or `minio` for s3 compatible object storage service or other name defined with `[storage.xxx]`
- `SERVE_DIRECT`: **false**: Allows the storage driver to redirect to authenticated URLs to serve files directly. Currently, only Minio/S3 is supported via signed URLs, local does nothing.
- `PATH`: **./data/repo-archive**: Where to store archive files, only available when `STORAGE_TYPE` is `local`.
- `MINIO_ENDPOINT`: **localhost:9000**: Minio endpoint to connect only available when `STORAGE_TYPE` is `minio`
- `MINIO_ACCESS_KEY_ID`: Minio accessKeyID to connect only available when `STORAGE_TYPE` is `minio`
- `MINIO_SECRET_ACCESS_KEY`: Minio secretAccessKey to connect only available when `STORAGE_TYPE is` `minio`
- `MINIO_BUCKET`: **gitea**: Minio bucket to store the lfs only available when `STORAGE_TYPE` is `minio`
- `MINIO_LOCATION`: **us-east-1**: Minio location to create bucket only available when `STORAGE_TYPE` is `minio`
- `MINIO_BASE_PATH`: **repo-archive/**: Minio base path on the bucket only available when `STORAGE_TYPE` is `minio`
- `MINIO_USE_SSL`: **false**: Minio enabled ssl only available when `STORAGE_TYPE` is `minio`

## Other (`other`)

- `SHOW_FOOTER_BRANDING`: **false**: Show Gitea branding in the footer.
- `SHOW_FOOTER_VERSION`: **true**: Show Gitea and Go version information in the footer.
- `SHOW_FOOTER_TEMPLATE_LOAD_TIME`: **true**: Show time of template execution in the footer.
