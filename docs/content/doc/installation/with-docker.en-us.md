---
date: "2020-03-19T19:27:00+02:00"
title: "Installation with Docker"
slug: "install-with-docker"
weight: 10
toc: true
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

## Basics

The most simple setup just creates a volume and a network and starts the `gitea/gitea:latest`
image as a service. Since there is no database available, one can be initialized using SQLite3.
Create a directory like `gitea` and paste the following content into a file named `docker-compose.yml`.
Note that the volume should be owned by the user/group with the UID/GID specified in the config file.
If you don't give the volume correct permissions, the container may not start.
Also be aware that the tag `:latest` will install the current development version.
For a stable release you can use `:1` or specify a certain release like `:{{< version >}}`.

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

## Custom port

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
-      - "3000:3000"
-      - "222:22"
+      - "8080:3000"
+      - "2221:22"
```

## MySQL database

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
+      - DB_TYPE=mysql
+      - DB_HOST=db:3306
+      - DB_NAME=gitea
+      - DB_USER=gitea
+      - DB_PASSWD=gitea
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
+    image: mysql:5.7
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

## PostgreSQL database

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
+      - DB_TYPE=postgres
+      - DB_HOST=db:5432
+      - DB_NAME=gitea
+      - DB_USER=gitea
+      - DB_PASSWD=gitea
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
+    image: postgres:9.6
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
-      - ./gitea:/data
+      - gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    ports:
      - "3000:3000"
      - "222:22"
```

MySQL or PostgreSQL containers will need to be created separately.

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
* `DOMAIN`: **localhost**: Domain name of this server, used for the displayed http clone URL in Gitea's UI.
* `SSH_DOMAIN`: **localhost**: Domain name of this server, used for the displayed ssh clone URL in Gitea's UI. If the install page is enabled, SSH Domain Server takes DOMAIN value in the form (which overwrite this setting on save).
* `SSH_PORT`: **22**: SSH port displayed in clone URL.
* `SSH_LISTEN_PORT`: **%(SSH\_PORT)s**: Port for the built-in SSH server.
* `DISABLE_SSH`: **false**: Disable SSH feature when it's not available. If you want to disable SSH feature, you should set SSH port to `0` when installing Gitea.
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
* `USER_UID`: **1000**: The UID (Unix user ID) of the user that runs Gitea within the container. Match this to the UID of the owner of the `/data` volume if using host volumes (this is not necessary with named volumes).
* `USER_GID`: **1000**: The GID (Unix group ID) of the user that runs Gitea within the container. Match this to the GID of the owner of the `/data` volume if using host volumes (this is not necessary with named volumes).

# Customization

Customization files described [here](https://docs.gitea.io/en-us/customizing-gitea/) should
be placed in `/data/gitea` directory. If using host volumes, it's quite easy to access these
files; for named volumes, this is done through another container or by direct access at
`/var/lib/docker/volumes/gitea_gitea/_data`. The configuration file will be saved at
`/data/gitea/conf/app.ini` after the installation.

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

# SSH Container Passthrough

Since SSH is running inside the container, you'll have to pass SSH from the host to the
container if you wish to use SSH support. If you wish to do this without running the container
SSH on a non-standard port (or move your host port to a non-standard port), you can forward
SSH connections destined for the container with a little extra setup.

This guide assumes that you have created a user on the host called `git` which shares the same
UID/GID as the container values `USER_UID`/`USER_GID`. You should also create the directory
`/var/lib/gitea` on the host, owned by the `git` user and mounted in the container, e.g.

```
version: "3"

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
        - /var/lib/gitea:/data
        - /etc/timezone:/etc/timezone:ro
        - /etc/localtime:/etc/localtime:ro
      ports:
        - "3000:3000"
        - "127.0.0.1:2222:22"
```

You can see that we're also exposing the container SSH port to port 2222 on the host, and binding this
to 127.0.0.1 to prevent it being accessible external to the host machine itself.

On the **host**, you should create the file `/app/gitea/gitea` with the following contents and
make it executable (`chmod +x /app/gitea/gitea`):

```
#!/bin/sh
ssh -p 2222 -o StrictHostKeyChecking=no git@127.0.0.1 ${SSH_ORIGINAL_COMMAND}
```

Your `git` user needs to have an SSH key generated:

```
sudo -u git ssh-keygen -t rsa -b 4096 -C "Gitea Host Key"
```

Now, proceed with one of the points given below:

- symlink the container `.ssh/authorized_keys` file to your git user `.ssh/authorized_keys`.
This can be done on the host as the `/var/lib/gitea` directory is mounted inside the container under `/data`:

```
ln -s /var/lib/gitea/git/.ssh/authorized_keys /home/git/.ssh/authorized_keys
```

Then echo the `git` user SSH key into the authorized_keys file so the host can talk to the container over SSH:

```
echo "no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty $(cat /home/git/.ssh/id_rsa.pub)" >> /var/lib/gitea/git/.ssh/authorized_keys
```

Lastly, Gitea makes `authorized_keys` backups by default. This could be a problem
as the symbolic link made to `authorized_keys` previously could end up pointing
to an old backup. To resolve this, please put the following into your Gitea
config:

```
[ssh]
SSH_AUTHORIZED_KEYS_BACKUP=false
```

- mount your `.ssh` directory directly into the container i.e. add the
  following to the `volumes` section of your Docker container config:

```
- /home/git/.ssh/:/data/git/.ssh/
```

Now you should be able to use Git over SSH to your container without disrupting SSH access to the host.

Please note: SSH container passthrough will work only if using opensshd in container, and will not work if
`AuthorizedKeysCommand` is used in combination with setting `SSH_CREATE_AUTHORIZED_KEYS_FILE=false` to disable
authorized files key generation.
