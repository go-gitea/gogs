---
date: "2017-06-19T12:00:00+02:00"
title: "Installation from binary"
slug: "install-from-binary"
weight: 10
toc: false
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "From binary"
    weight: 20
    identifier: "install-from-binary"
---

# Installation from binary

All downloads come with SQLite, MySQL and PostgreSQL support, and are built with
embedded assets. This can be different for older releases.

**Table of Contents**

{{< toc >}}

## Download

Choose the file matching the destination platform from the [downloads page](https://dl.gitea.io/gitea/), copy the URL and replace the URL within the commands below:

```sh
wget -O gitea https://dl.gitea.io/gitea/{{< version >}}/gitea-{{< version >}}-linux-amd64
chmod +x gitea
```

## Verify GPG signature
Gitea signs all binaries with a [GPG key](https://keys.openpgp.org/search?q=teabot%40gitea.io) to prevent against unwanted modification of binaries.
To validate the binary, download the signature file which ends in `.asc` for the binary you downloaded and use the gpg command line tool.

```sh
gpg --keyserver keys.openpgp.org --recv 7C9E68152594688862D62AF62D9AE806EC1592E2
gpg --verify gitea-{{< version >}}-linux-amd64.asc gitea-{{< version >}}-linux-amd64
```

Look for the text `Good signature from "Teabot <teabot@gitea.io>"` to assert a good binary,
despite warnings like `This key is not certified with a trusted signature!`.

## Recommended server configuration

**NOTE:** Many of the following directories can be configured using [Environment Variables]({{< relref "doc/advanced/environment-variables.en-us.md" >}}) as well!
Of note, configuring `GITEA_WORK_DIR` will tell Gitea where to base its working directory, as well as ease installation.

### Prepare environment

Check that Git is installed on the server. If it is not, install it first.
```sh
git --version
```

Create user to run Gitea (ex. `git`)
```sh
adduser \
   --system \
   --shell /bin/bash \
   --gecos 'Git Version Control' \
   --group \
   --disabled-password \
   --home /home/git \
   git
```

### Create required directory structure

```sh
mkdir -p /var/lib/gitea/{custom,data,log}
chown -R git:git /var/lib/gitea/
chmod -R 750 /var/lib/gitea/
mkdir /etc/gitea
chown root:git /etc/gitea
chmod 770 /etc/gitea
```

**NOTE:** `/etc/gitea` is temporary set with write rights for user `git` so that Web installer could write configuration file. After installation is done, it is recommended to set rights to read-only using:
```
chmod 750 /etc/gitea
chmod 640 /etc/gitea/app.ini
```
If you don't want the web installer to be able to write the config file at all, it is also possible to make the config file read-only for the gitea user (owner/group `root:git`, mode `0640`), and set `INSTALL_LOCK = true`. In that case all database configuration details must be set beforehand in the config file, as well as the `SECRET_KEY` and `INTERNAL_TOKEN` values. See the [command line documentation]({{< relref "doc/usage/command-line.en-us.md" >}}) for information on using `gitea generate secret INTERNAL_TOKEN`.

### Configure Gitea's working directory

**NOTE:** If you plan on running Gitea as a Linux service, you can skip this step as the service file allows you to set `WorkingDirectory`. Otherwise, consider setting this environment variable (semi-)permanently so that Gitea consistently uses the correct working directory.
```
export GITEA_WORK_DIR=/var/lib/gitea/
```

### Copy Gitea binary to global location

```
cp gitea /usr/local/bin/gitea
```

## Running Gitea

After the above steps, two options to run Gitea are:

### 1. Creating a service file to start Gitea automatically (recommended)

See how to create [Linux service]({{< relref "run-as-service-in-ubuntu.en-us.md" >}})

### 2. Running from command-line/terminal

```
GITEA_WORK_DIR=/var/lib/gitea/ /usr/local/bin/gitea web -c /etc/gitea/app.ini
```

## Updating to a new version

You can update to a new version of Gitea by stopping Gitea, replacing the binary at `/usr/local/bin/gitea` and restarting the instance.
The binary file name should not be changed during the update to avoid problems
in existing repositories.

It is recommended you do a [backup]({{< relref "doc/usage/backup-and-restore.en-us.md" >}}) before updating your installation.

If you have carried out the installation steps as described above, the binary should
have the generic name `gitea`. Do not change this, i.e. to include the version number.

### 1. Restarting gitea with systemd (recommended)

As explained before, we recommend to use systemd as service manager. In this case ```systemctl restart gitea``` should be enough.

### 2. Restarting gitea without systemd

To restart your gitea instance, we recommend to use SIGHUP signal. If you know your gitea PID use ```kill -1 $GITEA_PID``` otherwise you can use ```killall -1 gitea``` or ```pkill -1 gitea```

To gracefully stop the gitea instance, a simple ```kill $GITEA_PID``` or ```killall gitea``` is enough.

**NOTE:** We don't recommend to use SIGKILL signal (know also as `-9`), you may be forcefully stopping some of Gitea internal tasks and it will not gracefully stop (tasks in queues, indexers processes, etc.)

See below for troubleshooting instructions to repair broken repositories after
an update of your Gitea version.

## Troubleshooting

### Old glibc versions

Older Linux distributions (such as Debian 7 and CentOS 6) may not be able to load the
Gitea binary, usually producing an error such as ```./gitea: /lib/x86_64-linux-gnu/libc.so.6:
version `GLIBC\_2.14' not found (required by ./gitea)```. This is due to the integrated
SQLite support in the binaries provided by dl.gitea.io. In this situation, it is usually
possible to [install from source]({{< relref "from-source.en-us.md" >}}) without sqlite
support.

### Running Gitea on another port

For errors like `702 runWeb()] [E] Failed to start server: listen tcp 0.0.0.0:3000:
bind: address already in use` Gitea needs to be started on another free port. This
is possible using `./gitea web -p $PORT`. It's possible another instance of Gitea
is already running.

### Running Gitea on Raspbian

As of v1.8, there is a problem with the arm7 version of Gitea and it doesn't run on Raspberry Pi and similar devices.

It is therefore recommended to switch to the arm6 version which has been tested and shown to work on Raspberry Pi and similar devices.

<!---
please remove after fixing the arm7 bug
--->
### Git error after updating to a new version of Gitea

If the binary file name has been changed during the update to a new version of Gitea,
git hooks in existing repositories will not work any more. In that case, a git
error will be displayed when pushing to the repository.

```
remote: ./hooks/pre-receive.d/gitea: line 2: [...]: No such file or directory
```

The `[...]` part of the error message will contain the path to your previous Gitea
binary.

To solve this, go to the admin options and run the task `Resynchronize pre-receive,
update and post-receive hooks of all repositories` to update all hooks to contain
the new binary path. Please note that this overwrite all git hooks including ones
with customizations made.

If you aren't using the built-in to Gitea SSH server you will also need to re-write
the authorized key file by running the `Update the '.ssh/authorized_keys' file with
Gitea SSH keys.` task in the admin options.
