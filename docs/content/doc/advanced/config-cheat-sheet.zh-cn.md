---
date: "2016-12-26T16:00:00+02:00"
title: "配置说明"
slug: "config-cheat-sheet"
weight: 20
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "配置说明"
    weight: 20
    identifier: "config-cheat-sheet"
---

# 配置说明

这是针对Gitea配置文件的说明，你可以了解Gitea的强大配置。需要说明的是，你的所有改变请修改 `custom/conf/app.ini` 文件而不是源文件。所有默认值可以通过 [app.example.ini](https://github.com/go-gitea/gitea/blob/master/custom/conf/app.example.ini) 查看到。如果你发现 `%(X)s` 这样的内容，请查看 [ini](https://github.com/go-ini/ini/#recursive-values) 这里的说明。标注了 :exclamation: 的配置项表明除非你真的理解这个配置项的意义，否则最好使用默认值。

{{< toc >}}

## Overall (`DEFAULT`)

- `APP_NAME`: 应用名称，改成你希望的名字。
- `RUN_USER`: 运行Gitea的用户，推荐使用 `git`；如果在你自己的个人电脑使用改成你自己的用户名。如果设置不正确，Gitea可能崩溃。
- `RUN_MODE`: 从性能考虑，如果在产品级的服务上改成 `prod`。如果您使用安装向导安装的那么会自动设置为 `prod`。

## Repository (`repository`)

- `ROOT`: 存放git工程的根目录。这里必须填绝对路径，默认值是 `~/<username>/gitea-repositories`。
- `SCRIPT_TYPE`: 服务器支持的Shell类型，通常是 `bash`，但有些服务器也有可能是 `sh`。
- `ANSI_CHARSET`: 默认字符编码。
- `FORCE_PRIVATE`: 强制所有git工程必须私有。
- `DEFAULT_PRIVATE`: 默认创建的git工程为私有。 可以是`last`, `private` 或 `public`。默认值是 `last`表示用户最后创建的Repo的选择。
- `DEFAULT_PUSH_CREATE_PRIVATE`: **true**:  通过 ``push-to-create`` 方式创建的仓库是否默认为私有仓库.
- `MAX_CREATION_LIMIT`: 全局最大每个用户创建的git工程数目， `-1` 表示没限制。
- `PULL_REQUEST_QUEUE_LENGTH`: 小心：合并请求测试队列的长度，尽量放大。

## UI (`ui`)

- `EXPLORE_PAGING_NUM`: 探索页面每页显示的仓库数量。
- `ISSUE_PAGING_NUM`: 工单页面每页显示的工单数量。
- `MEMBERS_PAGING_NUM`: **20**: 组织成员页面每页显示的成员数量。
- `FEED_MAX_COMMIT_NUM`: 活动流页面显示的最大提交数量。

### UI - Admin (`ui.admin`)

- `USER_PAGING_NUM`: 用户管理页面每页显示的用户数量。
- `REPO_PAGING_NUM`: 仓库管理页面每页显示的仓库数量。
- `NOTICE_PAGING_NUM`: 系统提示页面每页显示的提示数量。
- `ORG_PAGING_NUM`: 组织管理页面每页显示的组织数量。

## Markdown (`markdown`)

- `ENABLE_HARD_LINE_BREAK`: 是否启用硬换行扩展。

## Server (`server`)

- `PROTOCOL`: 可选 `http` 或 `https`。
- `DOMAIN`: 服务器域名。
- `ROOT_URL`: Gitea服务器的对外 URL。
- `HTTP_ADDR`: HTTP 监听地址。
- `HTTP_PORT`: HTTP 监听端口。
- `DISABLE_SSH`: 是否禁用SSH。
- `START_SSH_SERVER`: 是否启用内部SSH服务器。
- `SSH_PORT`: SSH端口，默认为 `22`。
- `OFFLINE_MODE`: 针对静态和头像文件禁用 CDN。
- `DISABLE_ROUTER_LOG`: 关闭日志中的路由日志。
- `CERT_FILE`: 启用HTTPS的证书文件。
- `KEY_FILE`: 启用HTTPS的密钥文件。
- `STATIC_ROOT_PATH`: 存放模板和静态文件的根目录，默认是 Gitea 的根目录。
- `STATIC_CACHE_TIME`: **6h**: 静态资源文件，包括 `custom/`, `public/` 和所有上传的头像的浏览器缓存时间。
- `ENABLE_GZIP`: 启用实时生成的数据启用 GZIP 压缩，不包括静态资源。
- `LANDING_PAGE`: 未登录用户的默认页面，可选 `home` 或 `explore`。

- `LFS_START_SERVER`: 是否启用 git-lfs 支持. 可以为 `true` 或 `false`， 默认是 `false`。
- `LFS_JWT_SECRET`: LFS 认证密钥，改成自己的。
- `LFS_CONTENT_PATH`: **已废弃**, 存放 lfs 命令上传的文件的地方，默认是 `data/lfs`。

## Database (`database`)

- `DB_TYPE`: 数据库类型，可选 `mysql`, `postgres`, `mssql`, `tidb` 或 `sqlite3`。
- `HOST`: 数据库服务器地址和端口。
- `NAME`: 数据库名称。
- `USER`: 数据库用户名。
- `PASSWD`: 数据库用户密码。
- `SSL_MODE`: MySQL 或 PostgreSQL数据库是否启用SSL模式。
- `CHARSET`: **utf8mb4**: 仅当数据库为 MySQL 时有效, 可以为 "utf8" 或 "utf8mb4"。注意：如果使用 "utf8mb4"，你的 MySQL InnoDB 版本必须在 5.6 以上。
- `PATH`: Tidb 或者 SQLite3 数据文件存放路径。
- `LOG_SQL`: **true**: 显示生成的SQL，默认为真。
- `MAX_IDLE_CONNS` **0**: 最大空闲数据库连接
- `CONN_MAX_LIFETIME` **3s**: 数据库连接最大存活时间

## Indexer (`indexer`)

- `ISSUE_INDEXER_TYPE`: **bleve**: 工单索引类型，当前支持 `bleve`, `db` 和 `elasticsearch`，当为 `db` 时其它工单索引项可不用设置。
- `ISSUE_INDEXER_CONN_STR`: ****: 工单索引连接字符串，仅当 ISSUE_INDEXER_TYPE 为 `elasticsearch` 时有效。例如: http://elastic:changeme@localhost:9200
- `ISSUE_INDEXER_NAME`: **gitea_issues**: 工单索引名称，仅当 ISSUE_INDEXER_TYPE 为 `elasticsearch` 时有效。
- `ISSUE_INDEXER_PATH`: **indexers/issues.bleve**: 工单索引文件存放路径，当索引类型为 `bleve` 时有效。
- `ISSUE_INDEXER_QUEUE_TYPE`: **levelqueue**: 工单索引队列类型，当前支持 `channel`， `levelqueue` 或 `redis`。
- `ISSUE_INDEXER_QUEUE_DIR`: **indexers/issues.queue**: 当 `ISSUE_INDEXER_QUEUE_TYPE` 为 `levelqueue` 时，保存索引队列的磁盘路径。
- `ISSUE_INDEXER_QUEUE_CONN_STR`: **addrs=127.0.0.1:6379 db=0**: 当 `ISSUE_INDEXER_QUEUE_TYPE` 为 `redis` 时，保存Redis队列的连接字符串。
- `ISSUE_INDEXER_QUEUE_BATCH_NUMBER`: **20**: 队列处理中批量提交数量。

- `REPO_INDEXER_ENABLED`: **false**: 是否启用代码搜索（启用后会占用比较大的磁盘空间，如果是bleve可能需要占用约6倍存储空间）。
- `REPO_INDEXER_TYPE`: **bleve**: 代码搜索引擎类型，可以为 `bleve` 或者 `elasticsearch`。
- `REPO_INDEXER_PATH`: **indexers/repos.bleve**: 用于代码搜索的索引文件路径。
- `REPO_INDEXER_CONN_STR`: ****: 代码搜索引擎连接字符串，当 `REPO_INDEXER_TYPE` 为 `elasticsearch` 时有效。例如： http://elastic:changeme@localhost:9200
- `REPO_INDEXER_NAME`: **gitea_codes**: 代码搜索引擎的名字，当 `REPO_INDEXER_TYPE` 为 `elasticsearch` 时有效。

- `UPDATE_BUFFER_LEN`: **20**: 代码索引请求的缓冲区长度。
- `MAX_FILE_SIZE`: **1048576**: 进行解析的源代码文件的最大长度，小于该值时才会索引。

## Security (`security`)

- `INSTALL_LOCK`: 是否允许运行安装向导，(跟管理员账号有关，十分重要)。
- `SECRET_KEY`: 全局服务器安全密钥 **最好改成你自己的** (当你运行安装向导的时候会被设置为一个随机值)。
- `LOGIN_REMEMBER_DAYS`: Cookie 保存时间，单位天。
- `COOKIE_USERNAME`: 保存用户名的 cookie 名称。
- `COOKIE_REMEMBER_NAME`: 保存自动登录信息的 cookie 名称。
- `REVERSE_PROXY_AUTHENTICATION_USER`: 反向代理认证的 HTTP 头名称。

## Service (`service`)

- `ACTIVE_CODE_LIVE_MINUTES`: 登录验证码失效时间，单位分钟。
- `RESET_PASSWD_CODE_LIVE_MINUTES`: 重置密码失效时间，单位分钟。
- `REGISTER_EMAIL_CONFIRM`: 启用注册邮件激活，前提是 `Mailer` 已经启用。
- `REGISTER_MANUAL_CONFIRM`: **false**: 新注册用户必须由管理员手动激活,启用此选项需取消`REGISTER_EMAIL_CONFIRM`.
- `DISABLE_REGISTRATION`: 禁用注册，启用后只能用管理员添加用户。
- `SHOW_REGISTRATION_BUTTON`: 是否显示注册按钮。
- `REQUIRE_SIGNIN_VIEW`: 是否所有页面都必须登录后才可访问。
- `ENABLE_CACHE_AVATAR`: 是否缓存来自 Gravatar 的头像。
- `ENABLE_NOTIFY_MAIL`: 是否发送工单创建等提醒邮件，需要 `Mailer` 被激活。
- `ENABLE_REVERSE_PROXY_AUTHENTICATION`: 允许反向代理认证，更多细节见：https://github.com/gogits/gogs/issues/165
- `ENABLE_REVERSE_PROXY_AUTO_REGISTRATION`: 允许通过反向认证做自动注册。
- `ENABLE_CAPTCHA`: 注册时使用图片验证码。

### Service - Expore (`service.explore`)

- `REQUIRE_SIGNIN_VIEW`: **false**: 仅允许已登录的用户查看探索页面。
- `DISABLE_USERS_PAGE`: **false**: 不显示用户探索页面。

## Webhook (`webhook`)

- `QUEUE_LENGTH`: 说明: Hook 任务队列长度。
- `DELIVER_TIMEOUT`: 请求webhooks的超时时间，单位秒。
- `SKIP_TLS_VERIFY`: 是否允许不安全的证书。
- `PAGING_NUM`: 每页显示的Webhook 历史数量。
- `PROXY_URL`: ****: 代理服务器网址，支持 http://, https//, socks://, 为空将使用环境变量中的 http_proxy/https_proxy 设置。
- `PROXY_HOSTS`: ****: 逗号分隔的需要代理的域名或IP地址。支持 * 号匹配符，使用 ** 匹配所有域名和IP地址。

## Mailer (`mailer`)

- `ENABLED`: 是否启用邮件服务。
- `DISABLE_HELO`: 禁用 HELO 命令。
- `HELO_HOSTNAME`: 自定义主机名来回应 HELO 命令。
- `HOST`: SMTP 主机地址和端口 (例如：smtp.gitea.io:587)。
- `FROM`: 邮件发送地址，RFC 5322. 这里可以填一个邮件地址或者 "Name" \<email@example.com\> 格式。
- `USER`: 用户名(通常就是邮件地址)。
- `PASSWD`: 密码。
- `SKIP_VERIFY`: 忽略证书验证。

说明：实际上 Gitea 仅仅支持基于 STARTTLS 的 SMTP。

## Cache (`cache`)

- `ENABLED`: **true**: 是否启用。
- `ADAPTER`: **memory**: 缓存引擎，可以为 `memory`, `redis` 或 `memcache`。
- `INTERVAL`: **60**: 只对内存缓存有效，GC间隔，单位秒。
- `HOST`: **\<empty\>**: 针对redis和memcache有效，主机地址和端口。
    - Redis: `network=tcp,addr=127.0.0.1:6379,password=macaron,db=0,pool_size=100,idle_timeout=180`
    - Memache: `127.0.0.1:9090;127.0.0.1:9091`
- `ITEM_TTL`: **16h**: 缓存项目失效时间，设置为 0 则禁用缓存。

## Cache - LastCommitCache settings (`cache.last_commit`)

- `ENABLED`: **true**: 是否启用。
- `ITEM_TTL`: **8760h**: 缓存项目失效时间，设置为 0 则禁用缓存。
- `COMMITS_COUNT`: **1000**: 仅当仓库的提交数大于时才启用缓存。

## Session (`session`)

- `PROVIDER`: Session 内容存储方式，可选 `memory`, `file`, `redis` 或 `mysql`。
- `PROVIDER_CONFIG`: 如果是文件，那么这里填根目录；其他的要填主机地址和端口。
- `COOKIE_SECURE`: 强制使用 HTTPS 作为session访问。
- `GC_INTERVAL_TIME`: Session失效时间。

## Picture (`picture`)

- `GRAVATAR_SOURCE`: 头像来源，可以是 `gravatar`, `duoshuo` 或者类似 `http://cn.gravatar.com/avatar/` 的来源
- `DISABLE_GRAVATAR`: 开启则只使用内部头像。
- `ENABLE_FEDERATED_AVATAR`: 启用头像联盟支持 (参见 http://www.libravatar.org)

- `AVATAR_STORAGE_TYPE`: **local**: 头像存储类型，可以为 `local` 或 `minio`，分别支持本地文件系统和 minio 兼容的API。
- `AVATAR_UPLOAD_PATH`: **data/avatars**: 存储头像的文件系统路径。
- `AVATAR_MAX_WIDTH`: **4096**: 头像最大宽度，单位像素。
- `AVATAR_MAX_HEIGHT`: **3072**: 头像最大高度，单位像素。
- `AVATAR_MAX_FILE_SIZE`: **1048576** (1Mb): 头像最大大小。

- `REPOSITORY_AVATAR_STORAGE_TYPE`: **local**: 仓库头像存储类型，可以为 `local` 或 `minio`，分别支持本地文件系统和 minio 兼容的API。
- `REPOSITORY_AVATAR_UPLOAD_PATH`: **data/repo-avatars**: 存储仓库头像的路径。
- `REPOSITORY_AVATAR_FALLBACK`: **none**: 当头像丢失时的处理方式
  - none = 不显示头像
  - random = 显示随机生成的头像
  - image = 显示默认头像，通过 `REPOSITORY_AVATAR_FALLBACK_IMAGE` 设置
- `REPOSITORY_AVATAR_FALLBACK_IMAGE`: **/img/repo_default.png**: 默认仓库头像

## Attachment (`attachment`)

- `ENABLED`: 是否允许用户上传附件。
- `ALLOWED_TYPES`: 允许上传的附件类型。比如：`image/jpeg|image/png`，用 `*/*` 表示允许任何类型。
- `MAX_SIZE`: 附件最大限制，单位 MB，比如： `4`。
- `MAX_FILES`: 一次最多上传的附件数量，比如： `5`。
- `STORAGE_TYPE`: **local**: 附件存储类型，`local` 将存储到本地文件夹， `minio` 将存储到 s3 兼容的对象存储服务中。
- `PATH`: **data/attachments**: 附件存储路径，仅当 `STORAGE_TYPE` 为 `local` 时有效。
- `MINIO_ENDPOINT`: **localhost:9000**: Minio 终端，仅当 `STORAGE_TYPE` 是 `minio` 时有效。
- `MINIO_ACCESS_KEY_ID`: Minio accessKeyID ，仅当 `STORAGE_TYPE` 是 `minio` 时有效。
- `MINIO_SECRET_ACCESS_KEY`: Minio secretAccessKey，仅当 `STORAGE_TYPE` 是 `minio` 时有效。
- `MINIO_BUCKET`: **gitea**: Minio bucket to store the attachments，仅当 `STORAGE_TYPE` 是 `minio` 时有效。
- `MINIO_LOCATION`: **us-east-1**: Minio location to create bucket，仅当 `STORAGE_TYPE` 是 `minio` 时有效。
- `MINIO_BASE_PATH`: **attachments/**: Minio base path on the bucket，仅当 `STORAGE_TYPE` 是 `minio` 时有效。
- `MINIO_USE_SSL`: **false**: Minio enabled ssl，仅当 `STORAGE_TYPE` 是 `minio` 时有效。

关于 `ALLOWED_TYPES`， 在 (*)unix 系统中可以使用`file -I <filename>` 来快速获得对应的 `MIME type`。

```shell
$ file -I test00.tar.xz
test00.tar.xz: application/x-xz; charset=binary

$ file --mime test00.xlsx
test00.xlsx: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet; charset=binary

file -I test01.xls
test01.xls: application/vnd.ms-excel; charset=binary
```


## Log (`log`)

- `ROOT_PATH`: 日志文件根目录。
- `MODE`: 日志记录模式，默认是为 `console`。如果要写到多个通道，用逗号分隔
- `LEVEL`: 日志级别，默认为`Trace`。

## Cron (`cron`)

- `ENABLED`: 是否在后台运行定期任务。
- `RUN_AT_START`: 是否启动时自动运行。
- `SCHEDULE` 所接受的格式
   - 完整 crontab 控制, 例如 `* * * * * ?`
   - 描述符, 例如 `@midnight`, `@every 1h30m` ...
   - 更多细节参见 [cron api文档](https://pkg.go.dev/github.com/gogs/cron@v0.0.0-20171120032916-9f6c956d3e14)


### Cron - Update Mirrors (`cron.update_mirrors`)

- `SCHEDULE`: 自动同步镜像仓库的Cron语法，比如：`@every 1h`。

### Cron - Repository Health Check (`cron.repo_health_check`)

- `SCHEDULE`: 仓库健康监测的Cron语法，比如：`@every 24h`。
- `TIMEOUT`: 仓库健康监测的超时时间，比如：`60s`.
- `ARGS`: 执行 `git fsck` 命令的参数，比如：`--unreachable --tags`。

### Cron - Repository Statistics Check (`cron.check_repo_stats`)

- `RUN_AT_START`: 是否启动时自动运行仓库统计。
- `SCHEDULE`: 仓库统计时的Cron 语法，比如：`@every 24h`.

### Cron - Update Migration Poster ID (`cron.update_migration_poster_id`)

- `SCHEDULE`: **@every 24h** : 每次同步的间隔时间。此任务总是在启动时自动进行。

## Git (`git`)

- `MAX_GIT_DIFF_LINES`: 比较视图中，一个文件最多显示行数。
- `MAX_GIT_DIFF_LINE_CHARACTERS`: 比较视图中一行最大字符数。
- `MAX_GIT_DIFF_FILES`: 比较视图中的最大现实文件数目。
- `GC_ARGS`: 执行 `git gc` 命令的参数, 比如： `--aggressive --auto`。

## Git - 超时设置 (`git.timeout`)

- `DEFAUlT`: **360**: Git操作默认超时时间，单位秒
- `MIGRATE`: **600**: 迁移外部仓库时的超时时间，单位秒
- `MIRROR`: **300**: 镜像外部仓库的超时时间，单位秒
- `CLONE`: **300**: 内部仓库间克隆的超时时间，单位秒
- `PULL`: **300**: 内部仓库间拉取的超时时间，单位秒
- `GC`: **60**: git仓库GC的超时时间，单位秒
- `ENABLE_AUTO_GIT_WIRE_PROTOCOL`: **true**: 是否根据 Git Wire Protocol协议支持情况自动切换版本，当 git 版本在 2.18 及以上时会自动切换到版本2。为 `false` 则不切换。

## API (`api`)

- `ENABLE_SWAGGER`: **true**: 是否启用swagger路由 /api/swagger, /api/v1/swagger etc. endpoints. True 或 false; 默认是  true.
- `MAX_RESPONSE_ITEMS`: **50**: 一个页面最大的项目数。
- `DEFAULT_PAGING_NUM`: **30**: API中默认分页条数。
- `DEFAULT_GIT_TREES_PER_PAGE`: **1000**: GIT TREES API每页的默认最大项数.
- `DEFAULT_MAX_BLOB_SIZE`: **10485760**: BLOBS API默认最大大小.

## Markup (`markup`)

外部渲染工具支持，你可以用你熟悉的文档渲染工具. 比如一下将新增一个名字为 `asciidoc` 的渲染工具which is followed `markup.` ini section. And there are some config items below.

```ini
[markup.asciidoc]
ENABLED = false
NEED_POSTPROCESS = true
FILE_EXTENSIONS = .adoc,.asciidoc
RENDER_COMMAND = "asciidoc --out-file=- -"
IS_INPUT_FILE = false
```

- ENABLED: 是否启用，默认为false。
- NEED\_POSTPROCESS: **true** 设置为 true 则会替换渲染文件中的内部链接和Commit ID 等。
- FILE_EXTENSIONS: 关联的文档的扩展名，多个扩展名用都好分隔。
- RENDER_COMMAND: 工具的命令行命令及参数。
- IS_INPUT_FILE: 输入方式是最后一个参数为文件路径还是从标准输入读取。

## Time (`time`)

- `FORMAT`: 显示在界面上的时间格式。比如： RFC1123 或者 2006-01-02 15:04:05
- `DEFAULT_UI_LOCATION`: 默认显示在界面上的时区，默认为本地时区。比如： Asia/Shanghai

## Task (`task`)

- `QUEUE_TYPE`: **channel**: 任务队列类型，可以为 `channel` 或 `redis`。
- `QUEUE_LENGTH`: **1000**: 任务队列长度，当 `QUEUE_TYPE` 为 `channel` 时有效。
- `QUEUE_CONN_STR`: **addrs=127.0.0.1:6379 db=0**: 任务队列连接字符串，当 `QUEUE_TYPE` 为 `redis` 时有效。如果redis有密码，则可以 `addrs=127.0.0.1:6379 password=123 db=0`。

## Migrations (`migrations`)

- `MAX_ATTEMPTS`: **3**: 在迁移过程中的 http/https 请求重试次数。
- `RETRY_BACKOFF`: **3**: 等待下一次重试的时间，单位秒。
- `ALLOWED_DOMAINS`: **\<empty\>**: 迁移仓库的域名白名单，默认为空，表示允许从任意域名迁移仓库，多个域名用逗号分隔。
- `BLOCKED_DOMAINS`: **\<empty\>**: 迁移仓库的域名黑名单，默认为空，多个域名用逗号分隔。如果 `ALLOWED_DOMAINS` 不为空，此选项将会被忽略。
- `ALLOW_LOCALNETWORKS`: **false**: Allow private addresses defined by RFC 1918

## LFS (`lfs`)

LFS 的存储配置。 如果 `STORAGE_TYPE` 为空，则此配置将从 `[storage]` 继承。如果不为 `local` 或者 `minio` 而为 `xxx`， 则从 `[storage.xxx]` 继承。当继承时， `PATH` 默认为 `data/lfs`，`MINIO_BASE_PATH` 默认为 `lfs/`。

- `STORAGE_TYPE`: **local**: LFS 的存储类型，`local` 将存储到磁盘，`minio` 将存储到 s3 兼容的对象服务。
- `SERVE_DIRECT`: **false**: 允许直接重定向到存储系统。当前，仅 Minio/S3 是支持的。
- `PATH`: 存放 lfs 命令上传的文件的地方，默认是 `data/lfs`。
- `MINIO_ENDPOINT`: **localhost:9000**: Minio 地址，仅当 `LFS_STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_ACCESS_KEY_ID`: Minio accessKeyID，仅当 `LFS_STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_SECRET_ACCESS_KEY`: Minio secretAccessKey，仅当 `LFS_STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_BUCKET`: **gitea**: Minio bucket，仅当 `LFS_STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_LOCATION`: **us-east-1**: Minio location ，仅当 `LFS_STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_BASE_PATH`: **lfs/**: Minio base path ，仅当 `LFS_STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_USE_SSL`: **false**: Minio 是否启用 ssl ，仅当 `LFS_STORAGE_TYPE` 为 `minio` 时有效。

## Storage (`storage`)

Attachments, lfs, avatars and etc 的默认存储配置。

- `STORAGE_TYPE`: **local**: 附件存储类型，`local` 将存储到本地文件夹， `minio` 将存储到 s3 兼容的对象存储服务中。
- `SERVE_DIRECT`: **false**: 允许直接重定向到存储系统。当前，仅 Minio/S3 是支持的。
- `MINIO_ENDPOINT`: **localhost:9000**: Minio 终端，仅当 `STORAGE_TYPE` 是 `minio` 时有效。
- `MINIO_ACCESS_KEY_ID`: Minio accessKeyID ，仅当 `STORAGE_TYPE` 是 `minio` 时有效。
- `MINIO_SECRET_ACCESS_KEY`: Minio secretAccessKey，仅当 `STORAGE_TYPE` 是 `minio` 时有效。
- `MINIO_BUCKET`: **gitea**: Minio bucket to store the attachments，仅当 `STORAGE_TYPE` 是 `minio` 时有效。
- `MINIO_LOCATION`: **us-east-1**: Minio location to create bucket，仅当 `STORAGE_TYPE` 是 `minio` 时有效。
- `MINIO_USE_SSL`: **false**: Minio enabled ssl，仅当 `STORAGE_TYPE` 是 `minio` 时有效。

你也可以自定义一个存储的名字如下：

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

然后你在 `[attachment]`, `[lfs]` 等中可以把这个名字用作 `STORAGE_TYPE` 的值。

## Repository Archive Storage (`storage.repo-archive`)

Repository archive 的存储配置。 如果 `STORAGE_TYPE` 为空，则此配置将从 `[storage]` 继承。如果不为 `local` 或者 `minio` 而为 `xxx`， 则从 `[storage.xxx]` 继承。当继承时， `PATH` 默认为 `data/repo-archive`，`MINIO_BASE_PATH` 默认为 `repo-archive/`。

- `STORAGE_TYPE`: **local**: Repository archive 的存储类型，`local` 将存储到磁盘，`minio` 将存储到 s3 兼容的对象服务。
- `SERVE_DIRECT`: **false**: 允许直接重定向到存储系统。当前，仅 Minio/S3 是支持的。
- `PATH`: 存放 Repository archive 上传的文件的地方，默认是 `data/repo-archive`。
- `MINIO_ENDPOINT`: **localhost:9000**: Minio 地址，仅当 `STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_ACCESS_KEY_ID`: Minio accessKeyID，仅当 `STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_SECRET_ACCESS_KEY`: Minio secretAccessKey，仅当 `STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_BUCKET`: **gitea**: Minio bucket，仅当 `STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_LOCATION`: **us-east-1**: Minio location ，仅当 `STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_BASE_PATH`: **repo-archive/**: Minio base path ，仅当 `STORAGE_TYPE` 为 `minio` 时有效。
- `MINIO_USE_SSL`: **false**: Minio 是否启用 ssl ，仅当 `STORAGE_TYPE` 为 `minio` 时有效。

## Other (`other`)

- `SHOW_FOOTER_BRANDING`: 为真则在页面底部显示Gitea的字样。
- `SHOW_FOOTER_VERSION`: 为真则在页面底部显示Gitea的版本。
