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

## Overall (`DEFAULT`)

- `APP_NAME`: **Gitea: Git with a cup of tea**: Application name, used in the page title.
- `RUN_USER`: **git**: The user Gitea will run as. This should be a dedicated system
   (non-user) account. Setting this incorrectly will cause Gitea to not start.
- `RUN_MODE`: **dev**: For performance and other purposes, change this to `prod` when
   deployed to a production environment. The installation process will set this to `prod`
   automatically. \[prod, dev, test\]

## Repository (`repository`)

- `ROOT`: **~/gitea-repositories/**: Root path for storing all repository data. It must be
   an absolute path.
- `SCRIPT_TYPE`: **bash**: The script type this server supports. Usually this is `bash`,
   but some users report that only `sh` is available.
- `DETECTED_CHARSETS_ORDER`: **UTF-8, UTF-16BE, UTF-16LE, UTF-32BE, UTF-32LE, ISO-8859, windows-1252, ISO-8859, windows-1250, ISO-8859, ISO-8859, ISO-8859, windows-1253, ISO-8859, windows-1255, ISO-8859, windows-1251, windows-1256, KOI8-R, ISO-8859, windows-1254, Shift_JIS, GB18030, EUC-JP, EUC-KR, Big5, ISO-2022, ISO-2022, ISO-2022, IBM424_rtl, IBM424_ltr, IBM420_rtl, IBM420_ltr**: Tie-break order of detected charsets - if the detected charsets have equal confidence, charsets earlier in the list will be chosen in preference to those later. Adding `defaults` will place the unnamed charsets at that point.
- `ANSI_CHARSET`: **\<empty\>**: Default ANSI charset to override non-UTF-8 charsets to.
- `FORCE_PRIVATE`: **false**: Force every new repository to be private.
- `DEFAULT_PRIVATE`: **last**: Default private when creating a new repository.
   \[last, private, public\]
- `MAX_CREATION_LIMIT`: **-1**: Global maximum creation limit of repositories per user,
   `-1` means no limit.
- `PULL_REQUEST_QUEUE_LENGTH`: **1000**: Length of pull request patch test queue, make it
   as large as possible. Use caution when editing this value.
- `MIRROR_QUEUE_LENGTH`: **1000**: Patch test queue length, increase if pull request patch
   testing starts hanging.
- `PREFERRED_LICENSES`: **Apache License 2.0,MIT License**: Preferred Licenses to place at
   the top of the list. Name must match file name in conf/license or custom/conf/license.
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
- `PREFIX_ARCHIVE_FILES`: **true**: Prefix archive files by placing them in a directory named after the repository.
- `DISABLE_MIRRORS`: **false**: Disable the creation of **new** mirrors. Pre-existing mirrors remain valid.
- `DEFAULT_BRANCH`: **master**: Default branch name of all repositories.

### Repository - Pull Request (`repository.pull-request`)

- `WORK_IN_PROGRESS_PREFIXES`: **WIP:,\[WIP\]**: List of prefixes used in Pull Request
 title to mark them as Work In Progress
- `CLOSE_KEYWORDS`: **close**, **closes**, **closed**, **fix**, **fixes**, **fixed**, **resolve**, **resolves**, **resolved**: List of
 keywords used in Pull Request comments to automatically close a related issue
- `REOPEN_KEYWORDS`: **reopen**, **reopens**, **reopened**: List of keywords used in Pull Request comments to automatically reopen
 a related issue
- `DEFAULT_MERGE_MESSAGE_COMMITS_LIMIT`: **50**: In the default merge message for squash commits include at most this many commits. Set to `-1` to include all commits
- `DEFAULT_MERGE_MESSAGE_SIZE`: **5120**: In the default merge message for squash commits limit the size of the commit messages. Set to `-1` to have no limit.
- `DEFAULT_MERGE_MESSAGE_ALL_AUTHORS`: **false**: In the default merge message for squash commits walk all commits to include all authors in the Co-authored-by otherwise just use those in the limited list
- `DEFAULT_MERGE_MESSAGE_MAX_APPROVERS`: **10**: In default merge messages limit the number of approvers listed as `Reviewed-by:`. Set to `-1` to include all.
- `DEFAULT_MERGE_MESSAGE_OFFICIAL_APPROVERS_ONLY`: **true**: In default merge messages only include approvers who are officially allowed to review.

### Repository - Issue (`repository.issue`)

- `LOCK_REASONS`: **Too heated,Off-topic,Resolved,Spam**: A list of reasons why a Pull Request or Issue can be locked

### Repository - Signing (`repository.signing`)

- `SIGNING_KEY`: **default**: \[none, KEYID, default \]: Key to sign with.
- `SIGNING_NAME` &amp; `SIGNING_EMAIL`: if a KEYID is provided as the `SIGNING_KEY`, use these as the Name and Email address of the signer. These should match publicized name and email address for the key.
- `INITIAL_COMMIT`: **always**: \[never, pubkey, twofa, always\]: Sign initial commit.
  - `never`: Never sign
  - `pubkey`: Only sign if the user has a public key
  - `twofa`: Only sign if the user is logged in with twofa
  - `always`: Always sign
  - Options other than `never` and `always` can be combined as a comma separated list.
- `WIKI`: **never**: \[never, pubkey, twofa, always, parentsigned\]: Sign commits to wiki.
- `CRUD_ACTIONS`: **pubkey, twofa, parentsigned**: \[never, pubkey, twofa, parentsigned, always\]: Sign CRUD actions.
  - Options as above, with the addition of:
  - `parentsigned`: Only sign if the parent commit is signed.
- `MERGES`: **pubkey, twofa, basesigned, commitssigned**: \[never, pubkey, twofa, approved, basesigned, commitssigned, always\]: Sign merges.
  - `approved`: Only sign approved merges to a protected branch.
  - `basesigned`: Only sign if the parent commit in the base repo is signed.
  - `headsigned`: Only sign if the head commit in the head branch is signed.
  - `commitssigned`: Only sign if all the commits in the head branch to the merge point are signed.

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
- `GRAPH_MAX_COMMIT_NUM`: **100**: Number of maximum commits shown in the commit graph.
- `DEFAULT_THEME`: **gitea**: \[gitea, arc-green\]: Set the default theme for the Gitea install.
- `THEMES`:  **gitea,arc-green**: All available themes. Allow users select personalized themes
  regardless of the value of `DEFAULT_THEME`.
- `REACTIONS`: All available reactions users can choose on issues/prs and comments
    Values can be emoji alias (:smile:) or a unicode emoji.
    For custom reactions, add a tightly cropped square image to public/emoji/img/reaction_name.png
- `DEFAULT_SHOW_FULL_NAME`: **false**: Whether the full name of the users should be shown where possible. If the full name isn't set, the username will be used.
- `SEARCH_REPO_DESCRIPTION`: **true**: Whether to search within description at repository search on explore page.
- `USE_SERVICE_WORKER`: **true**: Whether to enable a Service Worker to cache frontend assets.

### UI - Admin (`ui.admin`)

- `USER_PAGING_NUM`: **50**: Number of users that are shown in one page.
- `REPO_PAGING_NUM`: **50**: Number of repos that are shown in one page.
- `NOTICE_PAGING_NUM`: **25**: Number of notices that are shown in one page.
- `ORG_PAGING_NUM`: **50**: Number of organizations that are shown in one page.

### UI - Notification (`ui.notification`)

- `MIN_TIMEOUT`: **10s**: These options control how often notification endpoint is polled to update the notification count. On page load the notification count will be checked after `MIN_TIMEOUT`. The timeout will increase to `MAX_TIMEOUT` by `TIMEOUT_STEP` if the notification count is unchanged. Set MIN_TIMEOUT to 0 to turn off.
- `MAX_TIMEOUT`: **60s**.
- `TIMEOUT_STEP`: **10s**.
- `EVENT_SOURCE_UPDATE_TIME`: **10s**: This setting determines how often the database is queried to update notification counts. If the browser client supports `EventSource` and `SharedWorker`, a `SharedWorker` will be used in preference to polling notification endpoint. Set to **-1** to disable the `EventSource`.

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
- `DISABLE_SSH`: **false**: Disable SSH feature when it's not available.
- `START_SSH_SERVER`: **false**: When enabled, use the built-in SSH server.
- `SSH_DOMAIN`: **%(DOMAIN)s**: Domain name of this server, used for displayed clone URL.
- `SSH_PORT`: **22**: SSH port displayed in clone URL.
- `SSH_LISTEN_HOST`: **0.0.0.0**: Listen address for the built-in SSH server.
- `SSH_LISTEN_PORT`: **%(SSH\_PORT)s**: Port for the built-in SSH server.
- `OFFLINE_MODE`: **false**: Disables use of CDN for static files and Gravatar for profile pictures.
- `DISABLE_ROUTER_LOG`: **false**: Mute printing of the router log.
- `CERT_FILE`: **https/cert.pem**: Cert file path used for HTTPS. From 1.11 paths are relative to `CUSTOM_PATH`.
- `KEY_FILE`: **https/key.pem**: Key file path used for HTTPS. From 1.11 paths are relative to `CUSTOM_PATH`.
- `STATIC_ROOT_PATH`: **./**: Upper level of template and static files path.
- `STATIC_CACHE_TIME`: **6h**: Web browser cache time for static resources on `custom/`, `public/` and all uploaded avatars.
- `ENABLE_GZIP`: **false**: Enables application-level GZIP support.
- `LANDING_PAGE`: **home**: Landing page for unauthenticated users \[home, explore, organizations, login\].
- `LFS_START_SERVER`: **false**: Enables git-lfs support.
- `LFS_CONTENT_PATH`: **./data/lfs**: Where to store LFS files.
- `LFS_JWT_SECRET`: **\<empty\>**: LFS authentication secret, change this a unique string.
- `LFS_HTTP_AUTH_EXPIRY`: **20m**: LFS authentication validity period in time.Duration, pushes taking longer than this may fail.
- `LFS_MAX_FILE_SIZE`: **0**: Maximum allowed LFS file size in bytes (Set to 0 for no limit).
- `LFS_LOCK_PAGING_NUM`: **50**: Maximum number of LFS Locks returned per page.
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
- `PASSWD`: **\<empty\>**: Database user password. Use \`your password\` for quoting if you use special characters in the password.
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
- `CHARSET`: **utf8mb4**: For MySQL only, either "utf8" or "utf8mb4". NOTICE: for "utf8mb4" you must use MySQL InnoDB > 5.6. Gitea is unable to check this.
- `PATH`: **data/gitea.db**: For SQLite3 only, the database file path.
- `LOG_SQL`: **true**: Log the executed SQL.
- `DB_RETRIES`: **10**: How many ORM init / DB connect attempts allowed.
- `DB_RETRY_BACKOFF`: **3s**: time.Duration to wait before trying another ORM init / DB connect attempt, if failure occured.
- `MAX_OPEN_CONNS` **0**: Database maximum open connections - default is 0, meaning there is no limit.
- `MAX_IDLE_CONNS` **2**: Max idle database connections on connnection pool, default is 2 - this will be capped to `MAX_OPEN_CONNS`.
- `CONN_MAX_LIFETIME` **0 or 3s**: Sets the maximum amount of time a DB connection may be reused - default is 0, meaning there is no limit (except on MySQL where it is 3s - see #6804 & #7071).
  
Please see #8540 & #8273 for further discussion of the appropriate values for `MAX_OPEN_CONNS`, `MAX_IDLE_CONNS` & `CONN_MAX_LIFETIME` and their
relation to port exhaustion.

## Indexer (`indexer`)

- `ISSUE_INDEXER_TYPE`: **bleve**: Issue indexer type, currently supported: `bleve`, `db` or `elasticsearch`.
- `ISSUE_INDEXER_CONN_STR`: ****: Issue indexer connection string, available when ISSUE_INDEXER_TYPE is elasticsearch. i.e. http://elastic:changeme@localhost:9200
- `ISSUE_INDEXER_NAME`: **gitea_issues**: Issue indexer name, available when ISSUE_INDEXER_TYPE is elasticsearch
- `ISSUE_INDEXER_PATH`: **indexers/issues.bleve**: Index file used for issue search; available when ISSUE_INDEXER_TYPE is bleve and elasticsearch.
- The next 4 configuration values are deprecated and should be set in `queue.issue_indexer` however are kept for backwards compatibility:
- `ISSUE_INDEXER_QUEUE_TYPE`: **levelqueue**: Issue indexer queue, currently supports:`channel`, `levelqueue`, `redis`.
- `ISSUE_INDEXER_QUEUE_DIR`: **indexers/issues.queue**: When `ISSUE_INDEXER_QUEUE_TYPE` is `levelqueue`, this will be the queue will be saved path.
- `ISSUE_INDEXER_QUEUE_CONN_STR`: **addrs=127.0.0.1:6379 db=0**: When `ISSUE_INDEXER_QUEUE_TYPE` is `redis`, this will store the redis connection string.
- `ISSUE_INDEXER_QUEUE_BATCH_NUMBER`: **20**: Batch queue number.

- `REPO_INDEXER_ENABLED`: **false**: Enables code search (uses a lot of disk space, about 6 times more than the repository size).
- `REPO_INDEXER_PATH`: **indexers/repos.bleve**: Index file used for code search.
- `REPO_INDEXER_INCLUDE`: **empty**: A comma separated list of glob patterns (see https://github.com/gobwas/glob) to **include** in the index. Use `**.txt` to match any files with .txt extension. An empty list means include all files.
- `REPO_INDEXER_EXCLUDE`: **empty**: A comma separated list of glob patterns (see https://github.com/gobwas/glob) to **exclude** from the index. Files that match this list will not be indexed, even if they match in `REPO_INDEXER_INCLUDE`.
- `REPO_INDEXER_EXCLUDE_VENDORED`: **true**: Exclude vendored files from index.
- `UPDATE_BUFFER_LEN`: **20**: Buffer length of index request.
- `MAX_FILE_SIZE`: **1048576**: Maximum size in bytes of files to be indexed.
- `STARTUP_TIMEOUT`: **30s**: If the indexer takes longer than this timeout to start - fail. (This timeout will be added to the hammer time above for child processes - as bleve will not start until the previous parent is shutdown.) Set to zero to never timeout.

## Queue (`queue` and `queue.*`)

- `TYPE`: **persistable-channel**: General queue type, currently support: `persistable-channel`, `channel`, `level`, `redis`, `dummy`
- `DATADIR`: **queues/**: Base DataDir for storing persistent and level queues. `DATADIR` for inidividual queues can be set in `queue.name` sections but will default to `DATADIR/`**`name`**.
- `LENGTH`: **20**: Maximal queue size before channel queues block
- `BATCH_LENGTH`: **20**: Batch data before passing to the handler
- `CONN_STR`: **addrs=127.0.0.1:6379 db=0**: Connection string for the redis queue type.
- `QUEUE_NAME`: **_queue**: The suffix for default redis queue name. Individual queues will default to **`name`**`QUEUE_NAME` but can be overriden in the specific `queue.name` section.
- `SET_NAME`: **_unique**: The suffix that will added to the default redis
set name for unique queues. Individual queues will default to
**`name`**`QUEUE_NAME`_`SET_NAME`_ but can be overridden in the specific
`queue.name` section.
- `WRAP_IF_NECESSARY`: **true**: Will wrap queues with a timeoutable queue if the selected queue is not ready to be created - (Only relevant for the level queue.)
- `MAX_ATTEMPTS`: **10**: Maximum number of attempts to create the wrapped queue
- `TIMEOUT`: **GRACEFUL_HAMMER_TIME + 30s**: Timeout the creation of the wrapped queue if it takes longer than this to create.
- Queues by default come with a dynamically scaling worker pool. The following settings configure this:
- `WORKERS`: **1**: Number of initial workers for the queue.
- `MAX_WORKERS`: **10**: Maximum number of worker go-routines for the queue.
- `BLOCK_TIMEOUT`: **1s**: If the queue blocks for this time, boost the number of workers - the `BLOCK_TIMEOUT` will then be doubled before boosting again whilst the boost is ongoing.
- `BOOST_TIMEOUT`: **5m**: Boost workers will timeout after this long.
- `BOOST_WORKERS`: **5**: This many workers will be added to the worker pool if there is a boost.

## Admin (`admin`)
- `DEFAULT_EMAIL_NOTIFICATIONS`: **enabled**: Default configuration for email notifications for users (user configurable). Options: enabled, onmention, disabled

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
- `DISABLE_GIT_HOOKS`: **false**: Set to `true` to prevent all users (including admin) from creating custom
   git hooks.
- `ONLY_ALLOW_PUSH_IF_GITEA_ENVIRONMENT_SET`: **true**: Set to `false` to allow local users to push to gitea-repositories without setting up the Gitea environment. This is not recommended and if you want local users to push to gitea repositories you should set the environment appropriately.
- `IMPORT_LOCAL_PATHS`: **false**: Set to `false` to prevent all users (including admin) from importing local path on server.
- `ALLOW_USE_OF_SERVER_CREDENTIALS_FOR_SSH_IMPORT`: **false**: Set to `false` to prevent non-admin users from importing via SSH using the credentials of Gitea's user. When `true` only admins can import using SSH with those credentials.
- `INTERNAL_TOKEN`: **\<random at every install if no uri set\>**: Secret used to validate communication within Gitea binary.
- `INTERNAL_TOKEN_URI`: **<empty>**: Instead of defining internal token in the configuration, this configuration option can be used to give Gitea a path to a file that contains the internal token (example value: `file:/etc/gitea/internal_token`)
- `PASSWORD_HASH_ALGO`: **pbkdf2**: The hash algorithm to use \[pbkdf2, argon2, scrypt, bcrypt\].
- `CSRF_COOKIE_HTTP_ONLY`: **true**: Set false to allow JavaScript to read CSRF cookie.
- `PASSWORD_COMPLEXITY`: **lower,upper,digit,spec**: Comma separated list of character classes required to pass minimum complexity. If left empty or no valid values are specified, the default values will be used. Possible values are: 
    - lower - use one or more lower latin characters
    - upper - use one or more upper latin characters
    - digit - use one or more digits
    - spec - use one or more special characters as ``!"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~``
    - off - do not check password complexity

## OpenID (`openid`)

- `ENABLE_OPENID_SIGNIN`: **false**: Allow authentication in via OpenID.
- `ENABLE_OPENID_SIGNUP`: **! DISABLE\_REGISTRATION**: Allow registering via OpenID.
- `WHITELISTED_URIS`: **\<empty\>**: If non-empty, list of POSIX regex patterns matching
   OpenID URI's to permit.
- `BLACKLISTED_URIS`: **\<empty\>**: If non-empty, list of POSIX regex patterns matching
   OpenID URI's to block.

## Service (`service`)

- `ACTIVE_CODE_LIVE_MINUTES`: **180**: Time limit (min) to confirm account/email registration.
- `RESET_PASSWD_CODE_LIVE_MINUTES`: **180**: Time limit (min) to confirm forgot password reset
   process.
- `REGISTER_EMAIL_CONFIRM`: **false**: Enable this to ask for mail confirmation of registration.
   Requires `Mailer` to be enabled.
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
- `CAPTCHA_TYPE`: **image**: \[image, recaptcha\]
- `RECAPTCHA_SECRET`: **""**: Go to https://www.google.com/recaptcha/admin to get a secret for recaptcha.
- `RECAPTCHA_SITEKEY`: **""**: Go to https://www.google.com/recaptcha/admin to get a sitekey for recaptcha.
- `RECAPTCHA_URL`: **https://www.google.com/recaptcha/**: Set the recaptcha url - allows the use of recaptcha net.
- `DEFAULT_ENABLE_DEPENDENCIES`: **true**: Enable this to have dependencies enabled by default.
- `ALLOW_CROSS_REPOSITORY_DEPENDENCIES` : **true** Enable this to allow dependencies on issues from any repository where the user is granted access.
- `ENABLE_USER_HEATMAP`: **true**: Enable this to display the heatmap on users profiles.
- `EMAIL_DOMAIN_WHITELIST`: **\<empty\>**: If non-empty, list of domain names that can only be used to register
  on this instance.
- `SHOW_REGISTRATION_BUTTON`: **! DISABLE\_REGISTRATION**: Show Registration Button
- `SHOW_MILESTONES_DASHBOARD_PAGE`: **true** Enable this to show the milestones dashboard page - a view of all the user's milestones
- `AUTO_WATCH_NEW_REPOS`: **true**: Enable this to let all organisation users watch new repos when they are created
- `AUTO_WATCH_ON_CHANGES`: **false**: Enable this to make users watch a repository after their first commit to it
- `DEFAULT_ORG_VISIBILITY`: **public**: Set default visibility mode for organisations, either "public", "limited" or "private".
- `DEFAULT_ORG_MEMBER_VISIBLE`: **false** True will make the membership of the users visible when added to the organisation.
- `ALLOW_ONLY_EXTERNAL_REGISTRATION`: **false** Set to true to force registration only using third-party services.
- `NO_REPLY_ADDRESS`: **DOMAIN** Default value for the domain part of the user's email address in the git log if he has set KeepEmailPrivate to true. 
  The user's email will be replaced with a concatenation of the user name in lower case, "@" and NO_REPLY_ADDRESS.

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
  - Using opportunistic TLS via STARTTLS on port 587 is recommended per RFC 6409.
- `IS_TLS_ENABLED` :  **false** : Forcibly use TLS to connect even if not on a default SMTPS port. 
  - Note, if the port ends with `465` SMTPS/SMTP over TLS will be used despite this setting.
  - Otherwise if `IS_TLS_ENABLED=false` and the server supports `STARTTLS` this will be used. Thus if `STARTTLS` is preferred you should set `IS_TLS_ENABLED=false`.
- `FROM`: **\<empty\>**: Mail from address, RFC 5322. This can be just an email address, or
   the "Name" \<email@example.com\> format.
- `USER`: **\<empty\>**: Username of mailing user (usually the sender's e-mail address).
- `PASSWD`: **\<empty\>**: Password of mailing user.  Use \`your password\` for quoting if you use special characters in the password.
   - Please note: authentication is only supported when the SMTP server communication is encrypted with TLS (this can be via `STARTTLS`) or `HOST=localhost`. See [Email Setup]({{< relref "doc/usage/email-setup.en-us.md" >}}) for more information.
- `SKIP_VERIFY`: **false**: Whether or not to skip verification of certificates; `true` to disable verification.
   - **Warning:** This option is unsafe. Consider adding the certificate to the system trust store instead.
   - **Note:** Gitea only supports SMTP with STARTTLS.
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
- `SENDMAIL_TIMEOUT`: **5m**: default timeout for sending email through sendmail

## Cache (`cache`)

- `ENABLED`: **true**: Enable the cache.
- `ADAPTER`: **memory**: Cache engine adapter, either `memory`, `redis`, or `memcache`.
- `INTERVAL`: **60**: Garbage Collection interval (sec), for memory cache only.
- `HOST`: **\<empty\>**: Connection string for `redis` and `memcache`.
   - Redis: `network=tcp,addr=127.0.0.1:6379,password=macaron,db=0,pool_size=100,idle_timeout=180`
   - Memcache: `127.0.0.1:9090;127.0.0.1:9091`
- `ITEM_TTL`: **16h**: Time to keep items in cache if not used, Setting it to 0 disables caching.

## Cache - LastCommitCache settings (`cache.last_commit`)

- `ENABLED`: **true**: Enable the cache.
- `ITEM_TTL`: **8760h**: Time to keep items in cache if not used, Setting it to 0 disables caching.
- `COMMITS_COUNT`: **1000**: Only enable the cache when repository's commits count great than.

## Session (`session`)

- `PROVIDER`: **memory**: Session engine provider \[memory, file, redis, mysql, couchbase, memcache, nodb, postgres\].
- `PROVIDER_CONFIG`: **data/sessions**: For file, the root path; for others, the connection string.
- `COOKIE_SECURE`: **false**: Enable this to force using HTTPS for all session access.
- `COOKIE_NAME`: **i\_like\_gitea**: The name of the cookie used for the session ID.
- `GC_INTERVAL_TIME`: **86400**: GC interval in seconds.

## Picture (`picture`)

- `GRAVATAR_SOURCE`: **gravatar**: Can be `gravatar`, `duoshuo` or anything like
   `http://cn.gravatar.com/avatar/`.
- `DISABLE_GRAVATAR`: **false**: Enable this to use local avatars only.
- `ENABLE_FEDERATED_AVATAR`: **false**: Enable support for federated avatars (see
   [http://www.libravatar.org](http://www.libravatar.org)).
- `AVATAR_UPLOAD_PATH`: **data/avatars**: Path to store user avatar image files.
- `REPOSITORY_AVATAR_UPLOAD_PATH`: **data/repo-avatars**: Path to store repository avatar image files.
- `REPOSITORY_AVATAR_FALLBACK`: **none**: How Gitea deals with missing repository avatars
  - none = no avatar will be displayed
  - random = random avatar will be generated
  - image = default image will be used (which is set in `REPOSITORY_AVATAR_DEFAULT_IMAGE`)
- `REPOSITORY_AVATAR_FALLBACK_IMAGE`: **/img/repo_default.png**: Image used as default repository avatar (if `REPOSITORY_AVATAR_FALLBACK` is set to image and none was uploaded)
- `AVATAR_MAX_WIDTH`: **4096**: Maximum avatar image width in pixels.
- `AVATAR_MAX_HEIGHT`: **3072**: Maximum avatar image height in pixels.
- `AVATAR_MAX_FILE_SIZE`: **1048576** (1Mb): Maximum avatar image file size in bytes.

## Attachment (`attachment`)

- `ENABLED`: **true**: Enable this to allow uploading attachments.
- `PATH`: **data/attachments**: Path to store attachments.
- `ALLOWED_TYPES`: **see app.example.ini**: Allowed MIME types, e.g. `image/jpeg|image/png`.
   Use `*/*` for all types.
- `MAX_SIZE`: **4**: Maximum size (MB).
- `MAX_FILES`: **5**: Maximum number of attachments that can be uploaded at once.

## Log (`log`)

- `ROOT_PATH`: **\<empty\>**: Root path for log files.
- `MODE`: **console**: Logging mode. For multiple modes, use a comma to separate values. You can configure each mode in per mode log subsections `\[log.modename\]`. By default the file mode will log to `$ROOT_PATH/gitea.log`.
- `LEVEL`: **Info**: General log level. \[Trace, Debug, Info, Warn, Error, Critical, Fatal, None\]
- `STACKTRACE_LEVEL`: **None**: Default log level at which to log create stack traces. \[Trace, Debug, Info, Warn, Error, Critical, Fatal, None\]
- `REDIRECT_MACARON_LOG`: **false**: Redirects the Macaron log to its own logger or the default logger.
- `MACARON`: **file**: Logging mode for the macaron logger, use a comma to separate values. Configure each mode in per mode log subsections `\[log.modename.macaron\]`. By default the file mode will log to `$ROOT_PATH/macaron.log`. (If you set this to `,` it will log to default gitea logger.)
- `ROUTER_LOG_LEVEL`: **Info**: The log level that the router should log at. (If you are setting the access log, its recommended to place this at Debug.)
- `ROUTER`: **console**: The mode or name of the log the router should log to. (If you set this to `,` it will log to default gitea logger.)
NB: You must `REDIRECT_MACARON_LOG` and have `DISABLE_ROUTER_LOG` set to `false` for this option to take effect. Configure each mode in per mode log subsections `\[log.modename.router\]`.
- `ENABLE_ACCESS_LOG`: **false**: Creates an access.log in NCSA common log format, or as per the following template
- `ACCESS`: **file**: Logging mode for the access logger, use a comma to separate values. Configure each mode in per mode log subsections `\[log.modename.access\]`. By default the file mode will log to `$ROOT_PATH/access.log`. (If you set this to `,` it will log to the default gitea logger.)
- `ACCESS_LOG_TEMPLATE`: **`{{.Ctx.RemoteAddr}} - {{.Identity}} {{.Start.Format "[02/Jan/2006:15:04:05 -0700]" }} "{{.Ctx.Req.Method}} {{.Ctx.Req.URL.RequestURI}} {{.Ctx.Req.Proto}}" {{.ResponseWriter.Status}} {{.ResponseWriter.Size}} "{{.Ctx.Req.Referer}}\" \"{{.Ctx.Req.UserAgent}}"`**: Sets the template used to create the access log.
  - The following variables are available:
  - `Ctx`: the `macaron.Context` of the request.
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

- `ENABLED`: **true**: Run cron tasks periodically.
- `RUN_AT_START`: **false**: Run cron tasks at application start-up.

### Cron - Cleanup old repository archives (`cron.archive_cleanup`)

- `ENABLED`: **true**: Enable service.
- `RUN_AT_START`: **true**: Run tasks at start up time (if ENABLED).
- `SCHEDULE`: **@every 24h**: Cron syntax for scheduling repository archive cleanup, e.g. `@every 1h`.
- `OLDER_THAN`: **24h**: Archives created more than `OLDER_THAN` ago are subject to deletion, e.g. `12h`.

### Cron - Update Mirrors (`cron.update_mirrors`)

- `SCHEDULE`: **@every 10m**: Cron syntax for scheduling update mirrors, e.g. `@every 3h`.

### Cron - Repository Health Check (`cron.repo_health_check`)

- `SCHEDULE`: **@every 24h**: Cron syntax for scheduling repository health check.
- `TIMEOUT`: **60s**: Time duration syntax for health check execution timeout.
- `ARGS`: **\<empty\>**: Arguments for command `git fsck`, e.g. `--unreachable --tags`. See more on http://git-scm.com/docs/git-fsck

### Cron - Repository Statistics Check (`cron.check_repo_stats`)

- `RUN_AT_START`: **true**: Run repository statistics check at start time.
- `SCHEDULE`: **@every 24h**: Cron syntax for scheduling repository statistics check.

### Cron - Update Migration Poster ID (`cron.update_migration_poster_id`)

- `SCHEDULE`: **@every 24h** : Interval as a duration between each synchronization, it will always attempt synchronization when the instance starts.

## Git (`git`)

- `PATH`: **""**: The path of git executable. If empty, Gitea searches through the PATH environment.
- `MAX_GIT_DIFF_LINES`: **100**: Max number of lines allowed of a single file in diff view.
- `MAX_GIT_DIFF_LINE_CHARACTERS`: **5000**: Max character count per line highlighted in diff view.
- `MAX_GIT_DIFF_FILES`: **100**: Max number of files shown in diff view.
- `GC_ARGS`: **\<empty\>**: Arguments for command `git gc`, e.g. `--aggressive --auto`. See more on http://git-scm.com/docs/git-gc/
- `ENABLE_AUTO_GIT_WIRE_PROTOCOL`: **true**: If use git wire protocol version 2 when git version >= 2.18, default is true, set to false when you always want git wire protocol version 1
- `PULL_REQUEST_PUSH_MESSAGE`: **true**: Respond to pushes to a non-default branch with a URL for creating a Pull Request (if the repository has them enabled)
- `VERBOSE_PUSH`: **true**: Print status information about pushes as they are being processed.
- `VERBOSE_PUSH_DELAY`: **5s**: Only print verbose information if push takes longer than this delay.

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
- `REFRESH_TOKEN_EXPIRATION_TIME`: **730**: Lifetime of an OAuth2 access token in hours
- `INVALIDATE_REFRESH_TOKEN`: **false**: Check if refresh token got already used
- `JWT_SECRET`: **\<empty\>**: OAuth2 authentication secret for access and refresh tokens, change this a unique string.
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
FILE_EXTENSIONS = .adoc,.asciidoc
RENDER_COMMAND = "asciidoc --out-file=- -"
IS_INPUT_FILE = false
```

- ENABLED: **false** Enable markup support; set to **true** to enable this renderer.
- FILE\_EXTENSIONS: **\<empty\>** List of file extensions that should be rendered by an external
   command. Multiple extentions needs a comma as splitter.
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
```

 - `ELEMENT`: The element this policy applies to. Must be non-empty.
 - `ALLOW_ATTR`: The attribute this policy allows. Must be non-empty.
 - `REGEXP`: A regex to match the contents of the attribute against. Must be present but may be empty for unconditional whitelisting of this attribute.

Multiple sanitisation rules can be defined by adding unique subsections, e.g. `[markup.sanitizer.TeX-2]`.

## Time (`time`)

- `FORMAT`: Time format to diplay on UI. i.e. RFC1123 or 2006-01-02 15:04:05
- `DEFAULT_UI_LOCATION`: Default location of time on the UI, so that we can display correct user's time on UI. i.e. Shanghai/Asia

## Task (`task`)

Task queue configuration has been moved to `queue.task`. However, the below configuration values are kept for backwards compatibility:

- `QUEUE_TYPE`: **channel**: Task queue type, could be `channel` or `redis`.
- `QUEUE_LENGTH`: **1000**: Task queue length, available only when `QUEUE_TYPE` is `channel`.
- `QUEUE_CONN_STR`: **addrs=127.0.0.1:6379 db=0**: Task queue connection string, available only when `QUEUE_TYPE` is `redis`. If redis needs a password, use `addrs=127.0.0.1:6379 password=123 db=0`.

## Migrations (`migrations`)

- `MAX_ATTEMPTS`: **3**: Max attempts per http/https request on migrations.
- `RETRY_BACKOFF`: **3**: Backoff time per http/https request retry (seconds)

## Other (`other`)

- `SHOW_FOOTER_BRANDING`: **false**: Show Gitea branding in the footer.
- `SHOW_FOOTER_VERSION`: **true**: Show Gitea version information in the footer.
- `SHOW_FOOTER_TEMPLATE_LOAD_TIME`: **true**: Show time of template execution in the footer.
