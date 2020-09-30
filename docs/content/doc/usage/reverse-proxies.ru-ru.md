---
date: "2018-05-22T11:00:00+00:00"
title: "Использование: Обратные прокси"
slug: "reverse-proxies"
weight: 17
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Обратные прокси"
    weight: 16
    identifier: "reverse-proxies"
---

##  Использование Nginx в качестве обратного прокси
Если вы хотите, чтобы Nginx обслуживал ваш экземпляр Gitea, добавьте следующий раздел `server` в раздел `http` раздела `nginx.conf`:

```
server {
    listen 80;
    server_name git.example.com;

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

## Использование Nginx с подпутём в качестве обратного прокси

Если у вас уже есть сайт и вы хотите, чтобы Gitea совместно использовала доменное имя, вы можете настроить Nginx для обслуживания Gitea по подпутём, добавив следующий раздел `server` внутри раздела `http` файла `nginx.conf`:

```
server {
    listen 80;
    server_name git.example.com;

    location /git/ { # Note: Trailing slash
        proxy_pass http://localhost:3000/; # Note: Trailing slash
    }
}
```

Затем установите `[server] ROOT_URL = http://git.example.com/git/` в вашей конфигурации.

##  Использование Nginx в качестве обратного прокси и непосредственное обслуживание статических ресурсов
Мы можем настроить производительность при разделении запросов на статические и динамические категории.

Файлы CSS, файлы JavaScript, изображения и веб-шрифты являются статическим содержимым.
Первая страница, представление репозитория или список задач - это динамическое содержимое.

Nginx может напрямую обслуживать статические ресурсы и передавать в gitea только динамические запросы.
Nginx оптимизирован для обслуживания статического контента, в то время как проксирование больших ответов может быть противоположным этому.
 (см. https://serverfault.com/q/587386).

Загрузите снимок исходного репозитория Gitea в `/path/to/gitea/`.
После этого запустите `make frontend` в каталоге репозитория, чтобы сгенерировать статические ресурсы. Для этой задачи нас интересует только каталог `public/`, поэтому вы можете удалить остальные.
(Вам понадобится [Node with npm](https://nodejs.org/ru/download/) и `make` установлен для генерации статических ресурсов)

В зависимости от масштаба вашей пользовательской базы вы можете разделить трафик на два разных сервера
 или использовать cdn для статических файлов.

### с использованием одного узла и одного домена

Установите `[server] STATIC_URL_PREFIX = /_/static` в вашей конфигурации.

```
server {
    listen 80;
    server_name git.example.com;

    location /_/static {
        alias /path/to/gitea/public;
    }

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

### используя два узла и два домена

Установите `[server] STATIC_URL_PREFIX = http://cdn.example.com/gitea` в вашей конфигурации.

```
# application server running gitea
server {
    listen 80;
    server_name git.example.com;

    location / {
        proxy_pass http://localhost:3000;
    }
}
```

```
# static content delivery server
server {
    listen 80;
    server_name cdn.example.com;

    location /gitea {
        alias /path/to/gitea/public;
    }

    location / {
        return 404;
    }
}
```

## Использование Apache HTTPD в качестве обратного прокси

Если вы хотите, чтобы Apache HTTPD обслуживал ваш экземпляр Gitea, вы можете добавить следующее в свою конфигурацию Apache HTTPD (обычно находится в `/etc/apache2/httpd.conf` в Ubuntu):

```
<VirtualHost *:80>
    ...
    ProxyPreserveHost On
    ProxyRequests off
    AllowEncodedSlashes NoDecode
    ProxyPass / http://localhost:3000/ nocanon
    ProxyPassReverse / http://localhost:3000/
</VirtualHost>
```

Примечание: Должны быть включены следующие режимы Apache HTTPD: `proxy`, `proxy_http`

Если вы хотите использовать Let's Encrypt с проверкой webroot, добавьте строку `ProxyPass /.well-known !` до `ProxyPass` чтобы отключить проксирование этих запросов в Gitea.

## Использование Apache HTTPD с дополнительным путём в качестве обратного прокси

Если у вас уже есть сайт и вы хотите, чтобы Gitea совместно использовала доменное имя, вы можете настроить Apache HTTPD для обслуживания Gitea по дополнительному пути, добавив следующую конфигурацию Apache HTTPD (обычно расположенную `/etc/apache2/httpd.conf` в Ubuntu):

```
<VirtualHost *:80>
    ...
    <Proxy *>
         Order allow,deny
         Allow from all
    </Proxy>
    AllowEncodedSlashes NoDecode
    # Note: no trailing slash after either /git or port
    ProxyPass /git http://localhost:3000 nocanon
    ProxyPassReverse /git http://localhost:3000
</VirtualHost>
```

Тогда установите `[server] ROOT_URL = http://git.example.com/git/` в вашей конфигурации.

Примечание: Необходимо включить следующие режимы Apache HTTPD: `proxy`, `proxy_http`

## Использование Caddy в качестве обратного прокси

Если вы хотите, чтобы Caddy обслуживал ваш экземпляр Gitea, вы можете добавить следующий блок сервера в свой Caddyfile:

```
git.example.com {
    reverse_proxy localhost:3000
}
```

Если вы всё ещё используете Caddy v1, используйте:

```
git.example.com {
    proxy / localhost:3000
}
```

## Использование Caddy с дополнительным путём в качестве обратного прокси

Если у вас уже есть сайт и вы хотите, чтобы Gitea совместно использовала доменное имя, вы можете настроить Caddy для обслуживания Gitea по подпутью, добавив следующее в свой серверный блок в файле Caddyfile:

```
git.example.com {
    route /git/* {
        uri strip_prefix /git
        reverse_proxy localhost:3000
    }
}
```

Или для Caddy v1:

```
git.example.com {
    proxy /git/ localhost:3000
}
```

Тогда установите `[server] ROOT_URL = http://git.example.com/git/` в вашей конфигурации.

## Использование IIS в качестве обратного прокси

Если вы хотите запустить Gitea с IIS. Вам нужно будет настроить IIS с URL Rewrite в качестве обратного прокси.

1. Настройте пустой веб-сайт в IIS, скажем, с именем, `Gitea Proxy`.
2. Выполните первые два шага в [Microsoft's Technical Community Guide to Setup IIS with URL Rewrite](https://techcommunity.microsoft.com/t5/iis-support-blog/setup-iis-with-url-rewrite-as-a-reverse-proxy-for-real-world/ba-p/846222#M343). То есть:
  - Установите маршрутизацию запросов приложений (сокращенно ARR) с помощью установщика веб-платформы Microsoft 5.1 (WebPI) или загрузив расширение из [IIS.net]( https://www.iis.net/downloads/microsoft/application-request-routing)
  - После того, как модуль будет установлен в IIS, вы увидите новый значок в консоли администрирования IIS под названием URL Rewrite.
  - Откройте консоль диспетчера IIS и щелкните веб-сайт `Gitea Proxy` в дереве слева. Выберите и дважды щёлкните значок перезаписи URL-адреса на средней панели, чтобы загрузить интерфейс перезаписи URL-адреса.
  - Выберите действие `Добавить правило` на правой панели консоли управления и выберите `Правило обратного прокси` в категории `Правила для входящего и исходящего трафика`.
  - В разделе `Правила для входящих подключений` укажите в качестве имени хоста сервера, на котором работает Gitea со своим портом. например если вы используете Gitea на локальном хосте с портом 3000, следующее должно работать: `127.0.0.1:3000`
  - Включить разгрузку SSL 
  - В правилах исходящего трафика убедитесь, что установлен параметр `Rewrite the domain name of the links in HTTP response`, и установите поле`From:`, как указано выше, и поле`To:`на ваше внешнее имя хоста, скажем: `git.example.com`
  - Теперь отредактируйте `web.config` для вашего веб-сайта, чтобы он соответствовал следующему: (изменив`127.0.0.1:3000` и `git.example.com` по мере необходимости)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<configuration>
    <system.webServer>
        <rewrite>
            <rules>
                <rule name="ReverseProxyInboundRule1" stopProcessing="true">
                    <match url="(.*)" />
                    <action type="Rewrite" url="http://127.0.0.1:3000/{R:1}" />
                    <serverVariables>
                        <set name="HTTP_X_ORIGINAL_ACCEPT_ENCODING" value="HTTP_ACCEPT_ENCODING" />
                        <set name="HTTP_ACCEPT_ENCODING" value="" />
                    </serverVariables>
                </rule>
            </rules>
            <outboundRules>
                <rule name="ReverseProxyOutboundRule1" preCondition="ResponseIsHtml1">
                    <!-- set the pattern correctly here - if you only want to accept http or https -->
                    <!-- change the pattern and the action value as appropriate -->
                    <match filterByTags="A, Form, Img" pattern="^http(s)?://127.0.0.1:3000/(.*)" />
                    <action type="Rewrite" value="http{R:1}://git.example.com/{R:2}" />
                </rule>
                <rule name="RestoreAcceptEncoding" preCondition="NeedsRestoringAcceptEncoding">
                    <match serverVariable="HTTP_ACCEPT_ENCODING" pattern="^(.*)" />
                    <action type="Rewrite" value="{HTTP_X_ORIGINAL_ACCEPT_ENCODING}" />
                </rule>
                <preConditions>
                    <preCondition name="ResponseIsHtml1">
                        <add input="{RESPONSE_CONTENT_TYPE}" pattern="^text/html" />
                    </preCondition>
                    <preCondition name="NeedsRestoringAcceptEncoding">
                        <add input="{HTTP_X_ORIGINAL_ACCEPT_ENCODING}" pattern=".+" />
                    </preCondition>
                </preConditions>
            </outboundRules>
        </rewrite>
        <urlCompression doDynamicCompression="true" />
    </system.webServer>
</configuration>
```
