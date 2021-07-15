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
For a stable release you could use `:latest-rootless`, `:1-rootless` or specify a certain release like `:{{< version >}}-rootless`, but if you'd like to use the latest development version then `:dev-rootless` would be an appropriate tag.

```yaml
version: "2"

services:
  server:
    image: gitea/gitea:{{< version >}}-rootless
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
    image: gitea/gitea:{{< version >}}-rootless
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
    image: gitea/gitea:{{< version >}}-rootless
+    environment:
+      - GITEA__database__DB_TYPE=mysql
+      - GITEA__database__HOST=db:3306
+      - GITEA__database__NAME=gitea
+      - GITEA__database__USER=gitea
+      - GITEA__database__PASSWD=gitea
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
+    image: mysql:8
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
    image: gitea/gitea:{{< version >}}-rootless
    environment:
+      - GITEA__database__DB_TYPE=postgres
+      - GITEA__database__HOST=db:5432
+      - GITEA__database__NAME=gitea
+      - GITEA__database__USER=gitea
+      - GITEA__database__PASSWD=gitea
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
+    image: postgres:13
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
    image: gitea/gitea:{{< version >}}-rootless
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
    image: gitea/gitea:{{< version >}}-rootless
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
- Use image gitea/gitea:{{< version >}}-rootless

## Managing Deployments With Environment Variables

In addition to the environment variables above, any settings in `app.ini` can be set or overridden with an environment variable of the form: `GITEA__SECTION_NAME__KEY_NAME`. These settings are applied each time the docker container starts. Full information [here](https://github.com/go-gitea/gitea/tree/main/contrib/environment-to-ini).

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
