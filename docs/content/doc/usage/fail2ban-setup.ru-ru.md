---
date: "2018-05-11T11:00:00+02:00"
title: "Использование: Настройка fail2ban"
slug: "fail2ban-setup"
weight: 16
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Настройка Fail2ban"
    weight: 16
    identifier: "fail2ban-setup"
---

# Настройка Fail2ban для блокировки пользователей после неудачных попыток входа в систему

**Помните, что fail2ban является мощным средством и может вызвать множество проблем, если вы
сделаете это неправильно, поэтому обязательно проверьте это, прежде чем полагаться на него, чтобы не заблокировать себя.**

Gitea возвращает HTTP 200 для неправильного входа в веб-журналы, но если у вас есть параметры входа в 
`app.ini`, тогда вы сможете уйти в `log/gitea.log`, что даёт вам что-то вроде этого
при плохой аутентификации:

```log
2018/04/26 18:15:54 [I] Failed authentication attempt for user from xxx.xxx.xxx.xxx
```

Добавьте наш фильтр в `/etc/fail2ban/filter.d/gitea.conf`:

```ini
# gitea.conf
[Definition]
failregex =  .*Failed authentication attempt for .* from <HOST>
ignoreregex =
```

Добавьте наш jail в `/etc/fail2ban/jail.d/gitea.conf`:

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

Если вы используете Docker, вам также потребуется добавить дополнительный jail для обработки **FORWARD** 
цепи в **iptables**. Настроить в `/etc/fail2ban/jail.d/gitea-docker.conf`:

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

Тогда просто запустите `service fail2ban restart` чтобы применить ваши изменения. Вы можете проверить,
принял ли fail2ban вашу конфигурацию, используя `service fail2ban status`.

Убедитесь, что прочтите fail2ban и настройте его в соответствии с вашими потребностями, это блокирует кого-то
на **15 минут** (со всех портов), если они не проходят аутентификацию 10 раз в час.

Если вы запускаете Gitea через обратный прокси с Nginx (например, с Docker), вам необходимо добавить это
в свою конфигурацию Nginx, чтобы IP-адреса не отображались как 127.0.0.1: 

```
proxy_set_header X-Real-IP $remote_addr;
```
