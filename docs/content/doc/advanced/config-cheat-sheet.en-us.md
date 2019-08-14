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
accurately recorded in [app.ini.sample](https://github.com/go-gitea/gitea/blob/master/custom/conf/app.ini.sample)
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
- `ANSI_CHARSET`: **\<empty\>**: The default charset for an unrecognized charset.
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

### Repository - Pull Request (`repository.pull-request`)

- `WORK_IN_PROGRESS_PREFIXES`: **WIP:,\[WIP\]**: List of prefixes used in Pull Request
 title to mark them as Work In Progress

### Repository - Issue (`repository.issue`)

- `LOCK_REASONS`: **Too heated,Off-topic,Resolved,Spam**: A list of reasons why a Pull Request or Issue can be locked

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
- `FEED_MAX_COMMIT_NUM`: **5**: Number of maximum commits shown in one activity feed.
- `GRAPH_MAX_COMMIT_NUM`: **100**: Number of maximum commits shown in the commit graph.
- `DEFAULT_THEME`: **gitea**: \[gitea, arc-green\]: Set the default theme for the Gitea install.
- `THEMES`:  **gitea,arc-green**: All available themes. Allow users select personalized themes
  regardless of the value of `DEFAULT_THEME`.
- `DEFAULT_SHOW_FULL_NAME`: false: Whether the full name of the users should be shown where possible. If the full name isn't set, the username will be used.

### UI - Admin (`ui.admin`)

- `USER_PAGING_NUM`: **50**: Number of users that are shown in one page.
- `REPO_PAGING_NUM`: **50**: Number of repos that are shown in one page.
- `NOTICE_PAGING_NUM`: **25**: Number of notices that are shown in one page.
- `ORG_PAGING_NUM`: **50**: Number of organizations that are shown in one page.

## Markdown (`markdown`)

- `ENABLE_HARD_LINE_BREAK`: **false**: Enable Markdown's hard line break extension.

## Server (`server`)

- `PROTOCOL`: **http**: \[http, https, fcgi, unix\]
- `DOMAIN`: **localhost**: Domain name of this server.
- `ROOT_URL`: **%(PROTOCOL)s://%(DOMAIN)s:%(HTTP\_PORT)s/**:
   Overwrite the automatically generated public URL.
   This is useful if the internal and the external URL don't match (e.g. in Docker).
- `HTTP_ADDR`: **0.0.0.0**: HTTP listen address.
   - If `PROTOCOL` is set to `fcgi`, Gitea will listen for FastCGI requests on TCP socket
     defined by `HTTP_ADDR` and `HTTP_PORT` configuration settings.
   - If `PROTOCOL` is set to `unix`, this should be the name of the Unix socket file to use.
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
- `CERT_FILE`: **custom/https/cert.pem**: Cert file path used for HTTPS.
- `KEY_FILE`: **custom/https/key.pem**: Key file path used for HTTPS.
- `STATIC_ROOT_PATH`: **./**: Upper level of template and static files path.
- `ENABLE_GZIP`: **false**: Enables application-level GZIP support.
- `LANDING_PAGE`: **home**: Landing page for unauthenticated users  \[home, explore\].
- `LFS_START_SERVER`: **false**: Enables git-lfs support.
- `LFS_CONTENT_PATH`: **./data/lfs**: Where to store LFS files.
- `LFS_JWT_SECRET`: **\<empty\>**: LFS authentication secret, change this a unique string.
- `LFS_HTTP_AUTH_EXPIRY`: **20m**: LFS authentication validity period in time.Duration, pushes taking longer than this may fail.
- `REDIRECT_OTHER_PORT`: **false**: If true and `PROTOCOL` is https, allows redirecting http requests on `PORT_TO_REDIRECT` to the https port Gitea listens on.
- `PORT_TO_REDIRECT`: **80**: Port for the http redirection service to listen on. Used when `REDIRECT_OTHER_PORT` is true.
- `ENABLE_LETSENCRYPT`: **false**: If enabled you must set `DOMAIN` to valid internet facing domain (ensure DNS is set and port 80 is accessible by letsencrypt validation server).
   By using Lets Encrypt **you must consent** to their [terms of service](https://letsencrypt.org/documents/LE-SA-v1.2-November-15-2017.pdf).
- `LETSENCRYPT_ACCEPTTOS`: **false**: This is an explicit check that you accept the terms of service for Let's Encrypt.
- `LETSENCRYPT_DIRECTORY`: **https**: Directory that Letsencrypt will use to cache information such as certs and private keys.
- `LETSENCRYPT_EMAIL`: **email@example.com**: Email used by Letsencrypt to notify about problems with issued certificates. (No default)

## Database (`database`)

- `DB_TYPE`: **mysql**: The database type in use \[mysql, postgres, mssql, sqlite3\].
- `HOST`: **127.0.0.1:3306**: Database host address and port or absolute path for unix socket \[mysql, postgres\] (ex: /var/run/mysqld/mysqld.sock).
- `NAME`: **gitea**: Database name.
- `USER`: **root**: Database username.
- `PASSWD`: **\<empty\>**: Database user password. Use \`your password\` for quoting if you use special characters in the password.
- `SSL_MODE`: **disable**: For PostgreSQL and MySQL only.
- `CHARSET`: **utf8**: For MySQL only, either "utf8" or "utf8mb4", default is "utf8". NOTICE: for "utf8mb4" you must use MySQL InnoDB > 5.6. Gitea is unable to check this.
- `PATH`: **data/gitea.db**: For SQLite3 only, the database file path.
- `LOG_SQL`: **true**: Log the executed SQL.
- `DB_RETRIES`: **10**: How many ORM init / DB connect attempts allowed.
- `DB_RETRY_BACKOFF`: **3s**: time.Duration to wait before trying another ORM init / DB connect attempt, if failure occured.

## Indexer (`indexer`)

- `ISSUE_INDEXER_TYPE`: **bleve**: Issue indexer type, currently support: bleve or db, if it's db, below issue indexer item will be invalid.
- `ISSUE_INDEXER_PATH`: **indexers/issues.bleve**: Index file used for issue search.
- `ISSUE_INDEXER_QUEUE_TYPE`: **levelqueue**: Issue indexer queue, currently supports:`channel`, `levelqueue`, `redis`.
- `ISSUE_INDEXER_QUEUE_DIR`: **indexers/issues.queue**: When `ISSUE_INDEXER_QUEUE_TYPE` is `levelqueue`, this will be the queue will be saved path.
- `ISSUE_INDEXER_QUEUE_CONN_STR`: **addrs=127.0.0.1:6379 db=0**: When `ISSUE_INDEXER_QUEUE_TYPE` is `redis`, this will store the redis connection string.
- `ISSUE_INDEXER_QUEUE_BATCH_NUMBER`: **20**: Batch queue number.

- `REPO_INDEXER_ENABLED`: **false**: Enables code search (uses a lot of disk space, about 6 times more than the repository size).
- `REPO_INDEXER_PATH`: **indexers/repos.bleve**: Index file used for code search.
- `UPDATE_BUFFER_LEN`: **20**: Buffer length of index request.
- `MAX_FILE_SIZE`: **1048576**: Maximum size in bytes of files to be indexed.

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
- `IMPORT_LOCAL_PATHS`: **false**: Set to `false` to prevent all users (including admin) from importing local path on server.
- `INTERNAL_TOKEN`: **\<random at every install if no uri set\>**: Secret used to validate communication within Gitea binary.
- `INTERNAL_TOKEN_URI`: **<empty>**: Instead of defining internal token in the configuration, this configuration option can be used to give Gitea a path to a file that contains the internal token (example value: `file:/etc/gitea/internal_token`)
- `PASSWORD_HASH_ALGO`: **pbkdf2**: The hash algorithm to use \[pbkdf2, argon2, scrypt, bcrypt\].
- `CSRF_COOKIE_HTTP_ONLY`: **true**: Set false to allow JavaScript to read CSRF cookie.

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
- `REQUIRE_SIGNIN_VIEW`: **false**: Enable this to force users to log in to view any page.
- `ENABLE_NOTIFY_MAIL`: **false**: Enable this to send e-mail to watchers of a repository when
   something happens, like creating issues. Requires `Mailer` to be enabled.
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
- `ENABLE_USER_HEATMAP`: **true**: Enable this to display the heatmap on users profiles.
- `EMAIL_DOMAIN_WHITELIST`: **\<empty\>**: If non-empty, list of domain names that can only be used to register
  on this instance.
- `SHOW_REGISTRATION_BUTTON`: **! DISABLE\_REGISTRATION**: Show Registration Button
- `AUTO_WATCH_NEW_REPOS`: **true**: Enable this to let all organisation users watch new repos when they are created
- `DEFAULT_ORG_VISIBILITY`: **public**: Set default visibility mode for organisations, either "public", "limited" or "private".

## Webhook (`webhook`)

- `QUEUE_LENGTH`: **1000**: Hook task queue length. Use caution when editing this value.
- `DELIVER_TIMEOUT`: **5**: Delivery timeout (sec) for shooting webhooks.
- `SKIP_TLS_VERIFY`: **false**: Allow insecure certification.
- `PAGING_NUM`: **10**: Number of webhook history events that are shown in one page.

## Mailer (`mailer`)

- `ENABLED`: **false**: Enable to use a mail service.
- `DISABLE_HELO`: **\<empty\>**: Disable HELO operation.
- `HELO_HOSTNAME`: **\<empty\>**: Custom hostname for HELO operation.
- `HOST`: **\<empty\>**: SMTP mail host address and port (example: smtp.gitea.io:587).
- `FROM`: **\<empty\>**: Mail from address, RFC 5322. This can be just an email address, or
   the "Name" \<email@example.com\> format.
- `USER`: **\<empty\>**: Username of mailing user (usually the sender's e-mail address).
- `PASSWD`: **\<empty\>**: Password of mailing user.  Use \`your password\` for quoting if you use special characters in the password.
- `SKIP_VERIFY`: **\<empty\>**: Do not verify the self-signed certificates.
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
- ``IS_TLS_ENABLED`` :  **false** : Decide if SMTP connections should use TLS.

## Cache (`cache`)

- `ADAPTER`: **memory**: Cache engine adapter, either `memory`, `redis`, or `memcache`.
- `INTERVAL`: **60**: Garbage Collection interval (sec), for memory cache only.
- `HOST`: **\<empty\>**: Connection string for `redis` and `memcache`.
   - Redis: `network=tcp,addr=127.0.0.1:6379,password=macaron,db=0,pool_size=100,idle_timeout=180`
   - Memache: `127.0.0.1:9090;127.0.0.1:9091`

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
- `ALLOWED_TYPES`: **see app.ini.sample**: Allowed MIME types, e.g. `image/jpeg|image/png`.
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
- `ACCESS_LOG_TEMPLATE`: **`{{.Ctx.RemoteAddr}} - {{.Identity}} {{.Start.Format "[02/Jan/2006:15:04:05 -0700]" }} "{{.Ctx.Req.Method}} {{.Ctx.Req.RequestURI}} {{.Ctx.Req.Proto}}" {{.ResponseWriter.Status}} {{.ResponseWriter.Size}} "{{.Ctx.Req.Referer}}\" \"{{.Ctx.Req.UserAgent}}"`**: Sets the template used to create the access log.
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

- `SCHEDULE`: **every 24h**: Cron syntax for scheduling repository health check.
- `TIMEOUT`: **60s**: Time duration syntax for health check execution timeout.
- `ARGS`: **\<empty\>**: Arguments for command `git fsck`, e.g. `--unreachable --tags`. See more on http://git-scm.com/docs/git-fsck

### Cron - Repository Statistics Check (`cron.check_repo_stats`)

- `RUN_AT_START`: **true**: Run repository statistics check at start time.
- `SCHEDULE`: **@every 24h**: Cron syntax for scheduling repository statistics check.

## Git (`git`)

- `PATH`: **""**: The path of git executable. If empty, Gitea searches through the PATH environment.
- `MAX_GIT_DIFF_LINES`: **100**: Max number of lines allowed of a single file in diff view.
- `MAX_GIT_DIFF_LINE_CHARACTERS`: **5000**: Max character count per line highlighted in diff view.
- `MAX_GIT_DIFF_FILES`: **100**: Max number of files shown in diff view.
- `GC_ARGS`: **\<empty\>**: Arguments for command `git gc`, e.g. `--aggressive --auto`. See more on http://git-scm.com/docs/git-gc/
- `ENABLE_AUTO_GIT_WIRE_PROTOCOL`: **true**: If use git wire protocol version 2 when git version >= 2.18, default is true, set to false when you always want git wire protocol version 1

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

## i18n (`i18n`)

- `LANGS`: **en-US,zh-CN,zh-HK,zh-TW,de-DE,fr-FR,nl-NL,lv-LV,ru-RU,ja-JP,es-ES,pt-BR,pl-PL,bg-BG,it-IT,fi-FI,tr-TR,cs-CZ,sr-SP,sv-SE,ko-KR**: List of locales shown in language selector
- `NAMES`: **English,简体中文,繁體中文（香港）,繁體中文（台灣）,Deutsch,français,Nederlands,latviešu,русский,日本語,español,português do Brasil,polski,български,italiano,suomi,Türkçe,čeština,српски,svenska,한국어**: Visible names corresponding to the locales

### i18n - Datepicker Language (`i18n.datelang`)
Maps locales to the languages used by the datepicker plugin

- `en-US`: **en**
- `zh-CN`: **zh**
- `zh-HK`: **zh-HK**
- `zh-TW`: **zh-TW**
- `de-DE`: **de**
- `fr-FR`: **fr**
- `nl-NL`: **nl**
- `lv-LV`: **lv**
- `ru-RU`: **ru**
- `ja-JP`: **ja**
- `es-ES`: **es**
- `pt-BR`: **pt-BR**
- `pl-PL`: **pl**
- `bg-BG`: **bg**
- `it-IT`: **it**
- `fi-FI`: **fi**
- `tr-TR`: **tr**
- `cs-CZ`: **cs-CZ**
- `sr-SP`: **sr**
- `sv-SE`: **sv**
- `ko-KR`: **ko**

## U2F (`U2F`)
- `APP_ID`: **`ROOT_URL`**: Declares the facet of the application. Requires HTTPS.
- `TRUSTED_FACETS`: List of additional facets which are trusted. This is not support by all browsers.

## Markup (`markup`)

Gitea can support Markup using external tools. The example below will add a markup named `asciidoc`.

```ini
[markup.asciidoc]
ENABLED = false
FILE_EXTENSIONS = .adoc,.asciidoc
RENDER_COMMAND = "asciidoc --out-file=- -"
IS_INPUT_FILE = false
```

- ENABLED: **false** Enable markup support.
- FILE\_EXTENSIONS: **\<empty\>** List of file extensions that should be rendered by an external
   command. Multiple extentions needs a comma as splitter.
- RENDER\_COMMAND: External command to render all matching extensions.
- IS\_INPUT\_FILE: **false** Input is not a standard input but a file param followed `RENDER_COMMAND`.

Two special environment variables are passed to the render command:
- `GITEA_PREFIX_SRC`, which contains the current URL prefix in the `src` path tree. To be used as prefix for links.
- `GITEA_PREFIX_RAW`, which contains the current URL prefix in the `raw` path tree. To be used as prefix for image paths.

## HSTS (`hsts`)

- ENABLED: **false** Enables hsts. True or false, default is false.
- MAX_AGE: **8760h** Max age of the time, default is 365 days.
- INCLUDE_SUB_DOMAINS: **false** require sub domains use hsts, default is false.
- SEND_PRELOAD_DIRECTIVE: **false** send preload, default is false.

## Other (`other`)

- `SHOW_FOOTER_BRANDING`: **false**: Show Gitea branding in the footer.
- `SHOW_FOOTER_VERSION`: **true**: Show Gitea version information in the footer.
- `SHOW_FOOTER_TEMPLATE_LOAD_TIME`: **true**: Show time of template execution in the footer.
