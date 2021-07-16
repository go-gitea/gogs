---
date: "2018-05-11T11:00:00+02:00"
title: "Usage: Setup fail2ban"
slug: "fail2ban-setup"
weight: 16
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Fail2ban setup"
    weight: 16
    identifier: "fail2ban-setup"
---

# Fail2ban setup to block users after failed login attempts

**Remember that fail2ban is powerful and can cause lots of issues if you do it incorrectly, so make
sure to test this before relying on it so you don't lock yourself out.**

Gitea returns an HTTP 200 for bad logins in the web logs, but if you have logging options on in
`app.ini`, then you should be able to go off of `log/gitea.log`, which gives you something like this
on a bad authentication from the web or CLI using SSH or HTTP respectively:

```log
2018/04/26 18:15:54 [I] Failed authentication attempt for user from xxx.xxx.xxx.xxx
```

```log
2020/10/15 16:05:09 modules/ssh/ssh.go:143:publicKeyHandler() [W] Failed authentication attempt from xxx.xxx.xxx.xxx
```
(DEPRECATED: This may be a false positive as the user may still go on to correctly authenticate.)

```log
2020/10/15 16:05:09 modules/ssh/ssh.go:155:publicKeyHandler() [W] Failed authentication attempt from xxx.xxx.xxx.xxx
```
(DEPRECATED: This may be a false positive as the user may still go on to correctly authenticate.)

```log
2020/10/15 16:05:09 modules/ssh/ssh.go:198:publicKeyHandler() [W] Failed authentication attempt from xxx.xxx.xxx.xxx
```
(DEPRECATED: This may be a false positive as the user may still go on to correctly authenticate.)

```log
2020/10/15 16:05:09 modules/ssh/ssh.go:213:publicKeyHandler() [W] Failed authentication attempt from xxx.xxx.xxx.xxx
```
(DEPRECATED: This may be a false positive as the user may still go on to correctly authenticate.)

```log
2020/10/15 16:05:09 modules/ssh/ssh.go:227:publicKeyHandler() [W] Failed authentication attempt from xxx.xxx.xxx.xxx
```
(DEPRECATED: This may be a false positive as the user may still go on to correctly authenticate.)

```log
2020/10/15 16:05:09 modules/ssh/ssh.go:249:sshConnectionFailed() [W] Failed authentication attempt from xxx.xxx.xxx.xxx
```
(From 1.15 this new message will available and doesn't have any of the false positive results that above messages from publicKeyHandler do. This will only be logged if the user has completely failed authentication.)

```log
2020/10/15 16:08:44 ...s/context/context.go:204:HandleText() [E] invalid credentials from xxx.xxx.xxx.xxx
```

Add our filter in `/etc/fail2ban/filter.d/gitea.conf`:

```ini
# gitea.conf
[Definition]
failregex =  .*(Failed authentication attempt|invalid credentials|Attempted access of unknown user).* from <HOST>
ignoreregex =
```

Add our jail in `/etc/fail2ban/jail.d/gitea.conf`:

```ini
[gitea]
enabled = true
filter = gitea
logpath = /var/lib/gitea/log/gitea.log
maxretry = 10
findtime = 3600
bantime = 900
action = iptables-allports
```

If you're using Docker, you'll also need to add an additional jail to handle the **FORWARD**
chain in **iptables**. Configure it in `/etc/fail2ban/jail.d/gitea-docker.conf`:

```ini
[gitea-docker]
enabled = true
filter = gitea
logpath = /home/git/gitea/log/gitea.log
maxretry = 10
findtime = 3600
bantime = 900
action = iptables-allports[chain="FORWARD"]
```

Then simply run `service fail2ban restart` to apply your changes. You can check to see if
fail2ban has accepted your configuration using `service fail2ban status`.

Make sure and read up on fail2ban and configure it to your needs, this bans someone
for **15 minutes** (from all ports) when they fail authentication 10 times in an hour.

If you run Gitea behind a reverse proxy with Nginx (for example with Docker), you need to add
this to your Nginx configuration so that IPs don't show up as 127.0.0.1:

```
proxy_set_header X-Real-IP $remote_addr;
```

The security options in `app.ini` need to be adjusted to allow the interpretation of the headers
as well as the list of IP addresses and networks that describe trusted proxy servers
(See the [configuration cheat sheet](https://docs.gitea.io/en-us/config-cheat-sheet/#security-security) for more information).

```
REVERSE_PROXY_LIMIT = 1
REVERSE_PROXY_TRUSTED_PROXIES = 127.0.0.1/8 ; 172.17.0.0/16 for the docker default network
```
