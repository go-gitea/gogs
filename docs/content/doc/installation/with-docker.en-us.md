---
date: "2020-03-19T19:27:00+02:00"
title: "Installation with Docker"
slug: "install-with-docker"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "With Docker"
    weight: 10
    identifier: "install-with-docker"
---

# Installation with Docker

Gitea provides automatically updated Docker images within its Docker Hub organization. It is
possible to always use the latest stable tag or to use another service that handles updating
Docker images.

This reference setup guides users through the setup based on `docker-compose`, but the installation
of `docker-compose` is out of scope of this documentation. To install `docker-compose` itself, follow
the official [install instructions](https://docs.docker.com/compose/install/).

**Table of Contents**

{{< toc >}}

## Basics

The most simple setup just creates a volume and a network and starts the `gitea/gitea:latest`
image as a service. Since there is no database available, one can be initialized using SQLite3.
Create a directory like `gitea` and paste the following content into a file named `docker-compose.yml`.
Note that the volume should be owned by the user/group with the UID/GID specified in the config file.
If you don't give the volume correct permissions, the container may not start.
For a stable release you can use `:latest`, `:1` or specify a certain release like `:{{< version >}}`, but if you'd like to use the latest development version of Gitea then you could use the `:dev` tag.

```yaml
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:{{< version >}}
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
```

## Ports

To bind the integrated openSSH daemon and the webserver on a different port, adjust
the port section. It's common to just change the host port and keep the ports within
the container like they are.

```diff
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:{{< version >}}
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
-     - "3000:3000"
-     - "222:22"
+     - "8080:3000"
+     - "2221:22"
```

## Databases

### MySQL database

To start Gitea in combination with a MySQL database, apply these changes to the
`docker-compose.yml` file created above.

```diff
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:{{< version >}}
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
+     - GITEA__database__DB_TYPE=mysql
+     - GITEA__database__HOST=db:3306
+     - GITEA__database__NAME=gitea
+     - GITEA__database__USER=gitea
+     - GITEA__database__PASSWD=gitea
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
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
+    networks:
+      - gitea
+    volumes:
+      - ./mysql:/var/lib/mysql
```

### PostgreSQL database

To start Gitea in combination with a PostgreSQL database, apply these changes to
the `docker-compose.yml` file created above.

```diff
version: "3"

networks:
  gitea:
    external: false

services:
  server:
    image: gitea/gitea:{{< version >}}
    container_name: gitea
    environment:
      - USER_UID=1000
      - USER_GID=1000
+     - GITEA__database__DB_TYPE=postgres
+     - GITEA__database__HOST=db:5432
+     - GITEA__database__NAME=gitea
+     - GITEA__database__USER=gitea
+     - GITEA__database__PASSWD=gitea
    restart: always
    networks:
      - gitea
    volumes:
      - ./gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
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
+    networks:
+      - gitea
+    volumes:
+      - ./postgres:/var/lib/postgresql/data
```

## Named volumes

To use named volumes instead of host volumes, define and use the named volume
within the `docker-compose.yml` configuration. This change will automatically
create the required volume. You don't need to worry about permissions with
named volumes; Docker will deal with that automatically.

```diff
version: "3"

networks:
  gitea:
    external: false

+volumes:
+  gitea:
+    driver: local
+
services:
  server:
    image: gitea/gitea:{{< version >}}
    container_name: gitea
    restart: always
    networks:
      - gitea
    volumes:
-     - ./gitea:/data
+     - gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
```

MySQL or PostgreSQL containers will need to be created separately.

## Startup

To start this setup based on `docker-compose`, execute `docker-compose up -d`,
to launch Gitea in the background. Using `docker-compose ps` will show if Gitea
started properly. Logs can be viewed with `docker-compose logs`.

To shut down the setup, execute `docker-compose down`. This will stop
and kill the containers. The volumes will still exist.

Notice: if using a non-3000 port on http, change app.ini to match
`LOCAL_ROOT_URL = http://localhost:3000/`.

## Installation

After starting the Docker setup via `docker-compose`, Gitea should be available using a
favorite browser to finalize the installation. Visit http://server-ip:3000 and follow the
installation wizard. If the database was started with the `docker-compose` setup as
documented above, please note that `db` must be used as the database hostname.

## Configure the user inside Gitea using environment variables 

- `USER`: **git**: The username of the user that runs Gitea within the container.
- `USER_UID`: **1000**: The UID (Unix user ID) of the user that runs Gitea within the container. Match this to the UID of the owner of the `/data` volume if using host volumes (this is not necessary with named volumes).
- `USER_GID`: **1000**: The GID (Unix group ID) of the user that runs Gitea within the container. Match this to the GID of the owner of the `/data` volume if using host volumes (this is not necessary with named volumes).

## Customization

Customization files described [here](https://docs.gitea.io/en-us/customizing-gitea/) should
be placed in `/data/gitea` directory. If using host volumes, it's quite easy to access these
files; for named volumes, this is done through another container or by direct access at
`/var/lib/docker/volumes/gitea_gitea/_data`. The configuration file will be saved at
`/data/gitea/conf/app.ini` after the installation.

## Upgrading

:exclamation::exclamation: **Make sure you have volumed data to somewhere outside Docker container** :exclamation::exclamation:

To upgrade your installation to the latest release:

```bash
# Edit `docker-compose.yml` to update the version, if you have one specified
# Pull new images
docker-compose pull
# Start a new container, automatically removes old one
docker-compose up -d
```

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

## SSH Container Passthrough

Since SSH is running inside the container, SSH needs to be passed through from the host to the container if SSH support is desired. One option would be to run the container SSH on a non-standard port (or moving the host port to a non-standard port). Another option which might be more straightforward is to forward SSH connections from the host to the container. This setup is explained in the following.

This guide assumes that you have created a user on the host called `git` which shares the same `UID`/ `GID` as the container values `USER_UID`/ `USER_GID`. These values can be set as environment variables in the `docker-compose.yml`:

```bash
environment:
  - USER_UID=1000
  - USER_GID=1000
```

Next mount `/home/git/.ssh` of the host into the container. Otherwise the SSH authentication cannot work inside the container.

```bash
volumes:
  - /home/git/.ssh/:/data/git/.ssh
```

Now a SSH key pair needs to be created on the host. This key pair will be used to authenticate the `git` user on the host to the container.

```bash
sudo -u git ssh-keygen -t rsa -b 4096 -C "Gitea Host Key"
```

In the next step a file named `/app/gitea/gitea` (with executable permissions) needs to be created on the host. This file will issue the SSH forwarding from the host to the container. Add the following contents to `/app/gitea/gitea`:

```bash
ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 "SSH_ORIGINAL_COMMAND=\"$SSH_ORIGINAL_COMMAND\" $0 $@"
```

To make the forwarding work, the SSH port of the container (22) needs to be mapped to the host port 2222 in `docker-compose.yml` . Since this port does not need to be exposed to the outside world, it can be mapped to the `localhost` of the host machine:

```bash
ports:
  # [...]
  - "127.0.0.1:2222:22"
```

In addition, `/home/git/.ssh/authorized_keys` on the host needs to be modified. It needs to act in the same way as `authorized_keys` within the Gitea container. Therefore add the public key of the key you created above ("Gitea Host Key") to `~/git/.ssh/authorized_keys`.
This can be done via `echo "$(cat /home/git/.ssh/id_rsa.pub)" >> /home/git/.ssh/authorized_keys`.
Important: The pubkey from the `git` user needs to be added "as is" while all other pubkeys added via the Gitea web interface will be prefixed with `command="/app [...]`.

The file should then look somewhat like

```bash
# SSH pubkey from git user
ssh-rsa <Gitea Host Key>

# other keys from users
command="/app/gitea/gitea --config=/data/gitea/conf/app.ini serv key-1",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty <user pubkey>
```

Here is a detailed explanation what is happening when a SSH request is made:

1. A SSH request is made against the host (usually port 22) using the `git` user, e.g. `git clone git@domain:user/repo.git`.
2. In `/home/git/.ssh/authorized_keys` , the command executes the `/app/gitea/gitea` script.
3. `/app/gitea/gitea` forwards the SSH request to port 2222 which is mapped to the SSH port (22) of the container.
4. Due to the existence of the public key of the `git` user in `/home/git/.ssh/authorized_keys` the authentication host → container succeeds and the SSH request get forwarded to Gitea running in the docker container.

If a new SSH key is added in the Gitea web interface, it will be appended to `.ssh/authorized_keys` in the same way as the already existing key.

**Notes**

SSH container passthrough will work only if

- `opensshd` is used in the container
- if `AuthorizedKeysCommand` is _not used_ in combination with `SSH_CREATE_AUTHORIZED_KEYS_FILE=false` to disable authorized files key generation
- `LOCAL_ROOT_URL` is not changed
