---
date: "2017-01-01T16:00:00+02:00"
title: "Usage: Backup and Restore"
slug: "backup-and-restore"
weight: 11
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Backup and Restore"
    weight: 11
    identifier: "backup-and-restore"
---

# Backup and Restore

Gitea currently has a `dump` command that will save the installation to a zip file. This
file can be unpacked and used to restore an instance.

**Table of Contents**

{{< toc >}}

## Backup Command (`dump`)

Switch to the user running Gitea: `su git`. Run `./gitea dump -c /path/to/app.ini` in the Gitea installation
directory. There should be some output similar to the following:

```none
2016/12/27 22:32:09 Creating tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:09 Dumping local repositories.../home/git/gitea-repositories
2016/12/27 22:32:22 Dumping database...
2016/12/27 22:32:22 Packing dump files...
2016/12/27 22:32:34 Removing tmp work dir: /tmp/gitea-dump-417443001
2016/12/27 22:32:34 Finish dumping in file gitea-dump-1482906742.zip
```

Inside the `gitea-dump-1482906742.zip` file, will be the following:

- `app.ini` - Optional copy of configuration file if originally stored outside of the default `custom/` directory
- `custom` - All config or customization files in `custom/`.
- `data` - Data directory in <GITEA_WORK_DIR>, except sessions if you are using file session. This directory includes `attachments`, `avatars`, `lfs`, `indexers`, sqlite file if you are using sqlite.
- `gitea-db.sql` - SQL dump of database
- `gitea-repo.zip` - Complete copy of the repository directory.
- `log/` - Various logs. They are not needed for a recovery or migration.

Intermediate backup files are created in a temporary directory specified either with the
`--tempdir` command-line parameter or the `TMPDIR` environment variable.

### Using Docker (`dump`)

There are a few caveats for using the `dump` command with Docker.

The command has to be executed with the `RUN_USER = <OS_USERNAME>` specified in `gitea/conf/app.ini`; and, for the zipping of the backup folder to occur without permission error the command `docker exec` must be executed inside of the `--tempdir`.

Example:

```none
docker exec -u <OS_USERNAME> -it -w <--tempdir> $(docker ps -qf name=^<NAME_OF_DOCKER_CONTAINER>$) bash -c '/app/gitea/gitea dump -c </path/to/app.ini>'
```

\*Note: `--tempdir` refers to the temporary directory of the docker environment used by Gitea; if you have not specified a custom `--tempdir`, then Gitea uses `/tmp` or the `TMPDIR` environment variable of the docker container. For `--tempdir` adjust your `docker exec` command options accordingly.

The result should be a file, stored in the `--tempdir` specified, along the lines of: `gitea-dump-1482906742.zip`

## Restore Command (`restore`)

There is currently no support for a recovery command. It is a manual process that mostly
involves moving files to their correct locations and restoring a database dump.

Example:

```sh
unzip gitea-dump-1610949662.zip
cd gitea-dump-1610949662
mv data/conf/app.ini /etc/gitea/conf/app.ini
mv data/* /var/lib/gitea/data/
mv log/* /var/lib/gitea/log/
mv repos/* /var/lib/gitea/repositories/
chown -R gitea:gitea /etc/gitea/conf/app.ini /var/lib/gitea

# mysql
mysql --default-character-set=utf8mb4 -u$USER -p$PASS $DATABASE <gitea-db.sql
# sqlite3
sqlite3 $DATABASE_PATH <gitea-db.sql
# postgres
psql -U $USER -d $DATABASE < gitea-db.sql

service gitea restart
```

Repository git-hooks should be regenerated if installation method is changed (eg. binary -> Docker), or if Gitea is installed to a different directory than the previous installation.

With Gitea running, and from the directory Gitea's binary is located, execute: `./gitea admin regenerate hooks`

This ensures that application and configuration file paths in repository git-hooks are consistent and applicable to the current installation. If these paths are not updated, repository `push` actions will fail.
