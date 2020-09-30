---
date: "2019-10-15T10:10:00+05:00"
title: "Использование: настройка электронной почты"
slug: "email-setup"
weight: 12
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Настройка электронной почты"
    weight: 12
    identifier: "email-setup"
---

# Настройка электронной почты

Чтобы использовать встроенную поддержку электронной почты Gitea, обновите раздел [mailer] конфигурационного файла `app.ini`:

## Версия Sendmail 
Используйте команду sendmail операционной системы вместо SMTP. Это распространено на серверах Linux.  
Примечание: Для использования в официальном образе Gitea Docker настройте версию SMTP.
```ini
[mailer]
ENABLED       = true
FROM          = gitea@mydomain.com
MAILER_TYPE   = sendmail
SENDMAIL_PATH = /usr/sbin/sendmail
```

## Версия SMTP
```ini
[mailer]
ENABLED        = true
FROM           = gitea@mydomain.com
MAILER_TYPE    = smtp
HOST           = mail.mydomain.com:587
IS_TLS_ENABLED = true
USER           = gitea@mydomain.com
PASSWD         = `password`
```

- Перезапустите Gitea, чтобы изменения конфигурации вступили в силу.

- Чтобы отправить тестовое электронное письмо для проверки настроек, перейдите в Gitea > Панель управления > Конфигурация > Настройки почты.

Чтобы увидеть полный список настроек, проверьте [Памятку по конфигурации]({{< relref "doc/advanced/config-cheat-sheet.ru-ru.md" >}})

- Обратите внимание: аутентификация поддерживается только в том случае, если обмен данными с SMTP-сервером зашифрован с помощью TLS или 'HOST=localhost'. Шифрование TLS может быть выполнено:
  - Через сервер, поддерживающий TLS, через STARTTLS - обычно через порт 587. (также известный как Opportunistic TLS).
  - Соединение SMTPS (SMTP поверх транспортного уровня безопасности) через порт по умолчанию 465.
  - Принудительное соединение SMTPS с `IS_TLS_ENABLED=true`. (Оба они известны как неявный TLS.)
- Это связано с защитой внутренних библиотек Go от атак STRIPTLS.

### Gmail

Следующая конфигурация должна работать с SMTP-сервером GMail:

```ini
[mailer]
ENABLED        = true
HOST           = smtp.gmail.com:465
FROM           = example@gmail.com
USER           = example@gmail.com
PASSWD         = ***
MAILER_TYPE    = smtp
IS_TLS_ENABLED = true
HELO_HOSTNAME  = example.com
```
