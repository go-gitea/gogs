---
date: "2020-02-09T20:00:00+02:00"
title: "Installation with Docker (rootless)"
slug: "install-with-docker-rootless"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "With Docker Rootless"
    weight: 10
    identifier: "install-with-docker-rootless"
---

# Installation with Docker

Gitea provides automatically updated Docker images within its Docker Hub organization. It is
possible to always use the latest stable tag or to use another service that handles updating
Docker images.

The rootless image use Gitea internal ssh to provide git protocol and doesn't support openssh.

This reference setup guides users through the setup based on `docker-compose`, but the installation
of `docker-compose` is out of scope of this documentation. To install `docker-compose` itself, follow
the official [install instructions](https://docs.docker.com/compose/install/).

## Basics

The most simple setup just creates a volume and a network and starts the `gitea/gitea:latest-rootless`
image as a service. Since there is no database available, one can be initialized using SQLite3.
Create a directory for `data` and `config` then paste the following content into a file named `docker-compose.yml`.
Note that the volume should be owned by the user/group with the UID/GID specified in the config file. By default Gitea in docker will use uid:1000 gid:1000. If needed you can set ownership on those folders with the command: `sudo chown 1000:1000 config/ data/`
If you don't give the volume correct permissions, the container may not start.
Also be aware that the tag `:latest-rootless` will install the current development version.
For a stable release you can use `:1-rootless` or specify a certain release like `:{{< version >}}-rootless`.

```yaml
version: "2"

services:
  server:
    image: gitea/gitea:latest-rootless
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
```

## Custom port

To bind the integrated ssh and the webserver on a different port, adjust
the port section. It's common to just change the host port and keep the ports within
the container like they are.

```diff
version: "2"

services:
  server:
    image: gitea/gitea:latest-rootless
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea  
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
-      - "3000:3000"
-      - "2222:2222"
+      - "80:3000"
+      - "22:2222"
```

## MySQL database

To start Gitea in combination with a MySQL database, apply these changes to the
`docker-compose.yml` file created above.

```diff
version: "2"

services:
  server:
    image: gitea/gitea:latest-rootless
+    environment:
+      - DB_TYPE=mysql
+      - DB_HOST=db:3306
+      - DB_NAME=gitea
+      - DB_USER=gitea
+      - DB_PASSWD=gitea
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea  
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
+    depends_on:
+      - db
+
+  db:
+    image: mysql:5.7
+    restart: always
+    environment:
+      - MYSQL_ROOT_PASSWORD=gitea
+      - MYSQL_USER=gitea
+      - MYSQL_PASSWORD=gitea
+      - MYSQL_DATABASE=gitea
+    volumes:
+      - ./mysql:/var/lib/mysql
```

## PostgreSQL database

To start Gitea in combination with a PostgreSQL database, apply these changes to
the `docker-compose.yml` file created above.

```diff
version: "2"

services:
  server:
    image: gitea/gitea:latest-rootless
    environment:
+      - DB_TYPE=postgres
+      - DB_HOST=db:5432
+      - DB_NAME=gitea
+      - DB_USER=gitea
+      - DB_PASSWD=gitea
    restart: always
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea  
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
+    depends_on:
+      - db
+
+  db:
+    image: postgres:9.6
+    restart: always
+    environment:
+      - POSTGRES_USER=gitea
+      - POSTGRES_PASSWORD=gitea
+      - POSTGRES_DB=gitea
+    volumes:
+      - ./postgres:/var/lib/postgresql/data
```

## Named volumes

To use named volumes instead of host volumes, define and use the named volume
within the `docker-compose.yml` configuration. This change will automatically
create the required volume. You don't need to worry about permissions with
named volumes; Docker will deal with that automatically.

```diff
version: "2"

+volumes:
+  gitea:
+    driver: local
+
services:
  server:
    image: gitea/gitea:latest-rootless
    restart: always
    volumes:
-      - ./data:/var/lib/gitea
+      - gitea-data:/var/lib/gitea
-      - ./config:/etc/gitea
+      - gitea-config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
```

MySQL or PostgreSQL containers will need to be created separately.

## Custom user

You can choose to use a custom user (following --user flag definition https://docs.docker.com/engine/reference/run/#user).
As an example to clone the host user `git` definition use the command `id -u git` and add it to `docker-compose.yml` file:
Please make sure that the mounted folders are writable by the user.

```diff
version: "2"

services:
  server:
    image: gitea/gitea:latest-rootless
    restart: always
+    user: 1001
    volumes:
      - ./data:/var/lib/gitea
      - ./config:/etc/gitea
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "2222:2222"
```

## Start

To start this setup based on `docker-compose`, execute `docker-compose up -d`,
to launch Gitea in the background. Using `docker-compose ps` will show if Gitea
started properly. Logs can be viewed with `docker-compose logs`.

To shut down the setup, execute `docker-compose down`. This will stop
and kill the containers. The volumes will still exist.

Notice: if using a non-3000 port on http, change app.ini to match
`LOCAL_ROOT_URL = http://localhost:3000/`.

## Install

After starting the Docker setup via `docker-compose`, Gitea should be available using a
favorite browser to finalize the installation. Visit http://server-ip:3000 and follow the
installation wizard. If the database was started with the `docker-compose` setup as
documented above, please note that `db` must be used as the database hostname.

## Environments variables

You can configure some of Gitea's settings via environment variables:

(Default values are provided in **bold**)

* `APP_NAME`: **"Gitea: Git with a cup of tea"**: Application name, used in the page title.
* `RUN_MODE`: **prod**: Application run mode, affects performance and debugging. Either "dev", "prod" or "test".
* `SSH_DOMAIN`: **localhost**: Domain name of this server, used for the displayed clone URL in Gitea's UI.
* `SSH_PORT`: **2222**: SSH port displayed in clone URL.
* `SSH_LISTEN_PORT`: **%(SSH\_PORT)s**: Port for the built-in SSH server.
* `DISABLE_SSH`: **false**: Disable SSH feature when it's not available.
* `HTTP_PORT`: **3000**: HTTP listen port.
* `ROOT_URL`: **""**: Overwrite the automatically generated public URL. This is useful if the internal and the external URL don't match (e.g. in Docker).
* `LFS_START_SERVER`: **false**: Enables git-lfs support.
* `DB_TYPE`: **sqlite3**: The database type in use \[mysql, postgres, mssql, sqlite3\].
* `DB_HOST`: **localhost:3306**: Database host address and port.
* `DB_NAME`: **gitea**: Database name.
* `DB_USER`: **root**: Database username.
* `DB_PASSWD`: **"\<empty>"**: Database user password. Use \`your password\` for quoting if you use special characters in the password.
* `INSTALL_LOCK`: **false**: Disallow access to the install page.
* `SECRET_KEY`: **""**: Global secret key. This should be changed. If this has a value and `INSTALL_LOCK` is empty, `INSTALL_LOCK` will automatically set to `true`.
* `DISABLE_REGISTRATION`: **false**: Disable registration, after which only admin can create accounts for users.
* `REQUIRE_SIGNIN_VIEW`: **false**: Enable this to force users to log in to view any page.
* `GITEA_ADMIN_USER`: If defined, admin username to create at the very first app run. Will be honored if and only if:
  - `GITEA_ADMIN_EMAIL` is defined as well
  - No users exist already
* `GITEA_ADMIN_EMAIL`: Email address of the user admin to create (see `GITEA_ADMIN_USER`)
* `GITEA_ADMIN_PASSWORD`: Password of the user admin to create (see `GITEA_ADMIN_USER`). If not defined, a random generated password will be set.

# Customization

Customization files described [here](https://docs.gitea.io/en-us/customizing-gitea/) should
be placed in `/var/lib/gitea/custom` directory. If using host volumes, it's quite easy to access these
files; for named volumes, this is done through another container or by direct access at
`/var/lib/docker/volumes/gitea_gitea/_/var_lib_gitea`. The configuration file will be saved at
`/etc/gitea/app.ini` after the installation.

# Upgrading

:exclamation::exclamation: **Make sure you have volumed data to somewhere outside Docker container** :exclamation::exclamation:

To upgrade your installation to the latest release:
```
# Edit `docker-compose.yml` to update the version, if you have one specified
# Pull new images
docker-compose pull
# Start a new container, automatically removes old one
docker-compose up -d
```

# Upgrading from standard image

- Backup your setup
- Change volume mountpoint from /data to /var/lib/gitea
- If you used a custom app.ini move it to a new volume mounted to /etc/gitea
- Rename folder (inside volume) gitea to custom
- Edit app.ini if needed
  - Set START_SSH_SERVER = true
- Use image gitea/gitea:latest-rootless

## Managing Deployments With Environment Variables

In addition to the environment variables above, any settings in `app.ini` can be set or overridden with an environment variable of the form: `GITEA__SECTION_NAME__KEY_NAME`. These settings are applied each time the docker container starts. Full information [here](https://github.com/go-gitea/gitea/tree/master/contrib/environment-to-ini).

These environment variables can be passed to the docker container in `docker-compose.yml`. The following example will enable an smtp mail server if the required env variables `GITEA__mailer__FROM`, `GITEA__mailer__HOST`, `GITEA__mailer__PASSWD` are set on the host or in a `.env` file in the same directory as `docker-compose.yml`:

```bash
...
services:
  server:
    environment:
    - GITEA__mailer__ENABLED=true
    - GITEA__mailer__FROM=${GITEA__mailer__FROM:?GITEA__mailer__FROM not set}
    - GITEA__mailer__MAILER_TYPE=smtp
    - GITEA__mailer__HOST=${GITEA__mailer__HOST:?GITEA__mailer__HOST not set}
    - GITEA__mailer__IS_TLS_ENABLED=true
    - GITEA__mailer__USER=${GITEA__mailer__USER:-apikey}
    - GITEA__mailer__PASSWD="""${GITEA__mailer__PASSWD:?GITEA__mailer__PASSWD not set}"""
```

To set required TOKEN and SECRET values, consider using gitea's built-in [generate utility functions](https://docs.gitea.io/en-us/command-line/#generate).

# SSH Container Passthrough (not tested)

This should be possible by forcing `authorized_keys` generation via `gitea admin regenerate keys`.

We should use directly [SSH AuthorizedKeysCommand](https://docs.gitea.io/en-us/command-line/#keys) when it will be based on internal api.
