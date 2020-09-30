---
date: "2018-05-11T11:00:00+02:00"
title: "Usage: Setup fail2ban"
slug: "fail2ban-setup"
weight: 16
toc: true
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
on a bad authentication:

```log
2018/04/26 18:15:54 [I] Failed authentication attempt for user from xxx.xxx.xxx.xxx
```

Add our filter in `/etc/fail2ban/filter.d/gitea.conf`:

```ini
# gitea.conf
[Definition]
failregex =  .*Failed authentication attempt for .* from <HOST>
ignoreregex =
```

Add our jail in `/etc/fail2ban/jail.d/gitea.conf`:

```ini
[gitea]
enabled = true
filter = gitea
logpath = /home/git/gitea/log/gitea.log
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
