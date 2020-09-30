---
date: "2017-07-21T12:00:00+02:00"
title: "Запуск службы в Linux"
slug: "linux-service"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Служба в Linux"
    weight: 20
    identifier: "linux-service"
---

### Запустите Gitea как службу Linux

Вы можете запустить Gitea как службу, используя systemd или supervisor. Приведённые ниже шаги протестированы на Ubuntu 16.04, но они должны работать в любых дистрибутивах Linux (с небольшими изменениями).

#### Использование systemd

Скопируйте образец [gitea.service](https://github.com/go-gitea/gitea/blob/master/contrib/systemd/gitea.service) в `/etc/systemd/system/gitea.service`, затем отредактируйте файл в своём любимом редакторе.

Раскомментируйте все службы, которые необходимо включить на этом хосте, например MySQL.

Измените пользователя, домашний каталог и другие необходимые значения запуска. Измените
PORT или снимите флаг -p, если используется порт по умолчанию.

Включить и запустить Gitea при запуске:
```
sudo systemctl enable gitea
sudo systemctl start gitea
```

Если у вас systemd версии 220 или новее, вы можете сразу включить и сразу запустить Gitea, один раз по:
```
sudo systemctl enable gitea --now
```

#### Использование supervisor

Установите supervisor, выполнив команду ниже в терминале:
```
sudo apt install supervisor
```

Создайте каталог журналов для журналов supervisor:
```
# assuming Gitea is installed in /home/git/gitea/
mkdir /home/git/gitea/log/supervisor
```

Добавьте конфигурацию из образца
[конфигурация supervisord](https://github.com/go-gitea/gitea/blob/master/contrib/supervisor/gitea) в `/etc/supervisor/supervisord.conf`.

Используя ваш любимый редактор, измените пользователя (git) и домашние
(/home/git) настройки в соответствии со средой развёртывания. Измените PORT
или удалите флаг -p, если используется порт по умолчанию.

Наконец, включите и запустите supervisor при загрузке:
```
sudo systemctl enable supervisor
sudo systemctl start supervisor
```

Если у вас есть systemd версии 220 или новее, вы можете включить и сразу запустить supervisor с помощью:
```
sudo systemctl enable supervisor --now
```
