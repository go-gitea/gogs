---
date: "2018-06-02T11:00:00+02:00"
title: "Использование: Настройка HTTPS"
slug: "https-setup"
weight: 12
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Настройка HTTPS"
    weight: 12
    identifier: "https-setup"
---

# Настройка HTTPS для шифрования подключений к Gitea

## Использование встроенного сервера

Перед тем как включить HTTPS, убедитесь, что у вас есть действующие сертификаты SSL/TLS.
Вы можете использовать самостоятельно созданные сертификаты для оценки и тестирования. Пожалуйста, запустите `gitea cert --host [HOST]`, чтобы сгенерировать самоподписанный сертификат.

Если вы используете Apache или nginx на сервере, рекомендуется прочесть [руководство по обратному прокси]({{< relref "doc/usage/reverse-proxies.ru-ru.md" >}}).

Чтобы использовать встроенную поддержку HTTPS в Gitea, вы должны изменить файл `app.ini`:

```ini
[server]
PROTOCOL  = https
ROOT_URL  = https://git.example.com:3000/
HTTP_PORT = 3000
CERT_FILE = cert.pem
KEY_FILE  = key.pem
```

Чтобы узнать больше о значениях конфигурации, ознакомьтесь с [Config Cheat Sheet](../config-cheat-sheet#server).

### Настройка перенаправления HTTP

Сервер Gitea может прослушивать только один порт; для перенаправления HTTP-запросов на порт HTTPS вам необходимо включить службу перенаправления HTTP:

```ini
[server]
REDIRECT_OTHER_PORT = true
; Port the redirection service should listen on
PORT_TO_REDIRECT = 3080
```

Если вы используете Docker, убедитесь, что этот порт настроен в вашем файле `docker-compose.yml`.

## Использование Let's Encrypt

[Let's Encrypt](https://letsencrypt.org/) - это центр сертификации, который позволяет автоматически запрашивать и обновлять сертификаты SSL/TLS. В дополнение к запуску Gitea на настроенном вами порту, чтобы запросить сертификаты HTTPS, Gitea также должен будет указать порт 80 и настроит для вас автоматическое перенаправление на HTTPS. Let's Encrypt должен будет иметь доступ к Gitea через Интернет, чтобы подтвердить ваше право собственности на домен.

Используя Let's Encrypt **вы должны согласиться** с их [Условиями использования](https://letsencrypt.org/documents/LE-SA-v1.2-November-15-2017.pdf).

```ini
[server]
PROTOCOL=https
DOMAIN=git.example.com
ENABLE_LETSENCRYPT=true
LETSENCRYPT_ACCEPTTOS=true
LETSENCRYPT_DIRECTORY=https
LETSENCRYPT_EMAIL=email@example.com
```

Чтобы узнать больше о значениях конфигурации, ознакомьтесь с [Config Cheat Sheet](../config-cheat-sheet#server).

## Использование обратного прокси

Настройте обратный прокси, как показано на [руководство по обратному прокси](../reverse-proxies).

После этого включите HTTPS, следуя одному из этих руководств:

* [nginx](https://nginx.org/en/docs/http/configuring_https_servers.html)
* [apache2/httpd](https://httpd.apache.org/docs/2.4/ssl/ssl_howto.html)
* [caddy](https://caddyserver.com/docs/tls)

Примечание: Включение HTTPS только на уровне прокси называется [TLS Termination Proxy](https://en.wikipedia.org/wiki/TLS_termination_proxy). Прокси-сервер принимает входящие TLS-соединения, расшифровывает содержимое и передает уже незашифрованное содержимое в Gitea. Обычно это нормально, если и прокси, и экземпляры Gitea находятся либо на одном компьютере, либо на разных машинах в частной сети (при этом прокси доступен для внешней сети). Если ваш экземпляр Gitea отделён от вашего прокси-сервера в общедоступной сети или если вам нужно полное сквозное шифрование, вы также можете [включить поддержку HTTPS прямо в Gitea с помощью встроенного сервера](#using-the-built-in-server) и вместо этого перенаправить соединения через HTTPS.
