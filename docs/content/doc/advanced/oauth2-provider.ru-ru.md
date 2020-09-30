---
date: "2019-04-19:44:00+01:00"
title: "Провайдер OAuth2"
slug: "oauth2-provider"
weight: 41
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Провайдер OAuth2"
    weight: 41
    identifier: "oauth2-provider"
---


# Провайдер OAuth2

Gitea поддерживает роль поставщика OAuth2, позволяющего сторонним приложениям получать доступ к своим ресурсам с согласия пользователя. Эта функция доступна начиная с версии 1.8.0.

## Конечные точки


Конечные точки         | URL
-----------------------|----------------------------
Конечная точка авторизации | `/login/oauth/authorize`
Конечная точка токена доступа | `/login/oauth/access_token`


## Поддерживаемые гранты OAuth2

На данный момент Gitea поддерживает только [**Предоставление кода авторизации**](https://tools.ietf.org/html/rfc6749#section-1.3.1) стандарт с дополнительной поддержкой расширения [Proof Key for Code Exchange (PKCE)](https://tools.ietf.org/html/rfc7636).
 

Чтобы использовать Предоставление кода авторизации в качестве стороннего приложения, необходимо зарегистрировать новое приложение через раздел "Настройки" (`/user/settings/applications`) в настройках.

## Scopes

В настоящее время Gitea не поддерживает scope (см. [#4300](https://github.com/go-gitea/gitea/issues/4300)) и всем сторонним приложениям будет предоставлен доступ ко всем ресурсам пользователя и его/её организаций.

## Пример

**Примечание:** В этом примере не используется PKCE.

1. Перенаправить пользователю на конечную точку авторизации, чтобы получить его согласие на доступ к ресурсам:

```curl
https://[YOUR-GITEA-URL]/login/oauth/authorize?client_id=CLIENT_ID&redirect_uri=REDIRECT_URI& response_type=code&state=STATE
``` 

`CLIENT_ID` можно получить, зарегистрировав приложение в настройках. `STATE` - это случайная строка, которая будет отправлена обратно в ваше приложение после авторизации пользователя. Параметр `state` не является обязательным, но должен использоваться для предотвращения атак CSRF.


![Authorization Page](/authorize.png)

Теперь пользователю будет предложено авторизовать ваше приложение. Если они авторизуют его, пользователь будет перенаправлен на `REDIRECT_URL`, например:

```curl
https://[REDIRECT_URI]?code=RETURNED_CODE&state=STATE
```

2. Используя предоставленный `code` из перенаправления, вы можете запросить новое приложение и обновить токен. Конечные точки токенов доступа принимают запросы POST с телом `application/json` и `application/x-www-form-urlencoded`, например:

```curl
POST https://[YOUR-GITEA-URL]/login/oauth/access_token
```

```json
{
	"client_id": "YOUR_CLIENT_ID",
	"client_secret": "YOUR_CLIENT_SECRET",
	"code": "RETURNED_CODE",
	"grant_type": "authorization_code",
	"redirect_uri": "REDIRECT_URI"
}
```

Отклик:
```json
{  
"access_token":"eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJnbnQiOjIsInR0IjowLCJleHAiOjE1NTUxNzk5MTIsImlhdCI6MTU1NTE3NjMxMn0.0-iFsAwBtxuckA0sNZ6QpBQmywVPz129u75vOM7wPJecw5wqGyBkmstfJHAjEOqrAf_V5Z-1QYeCh_Cz4RiKug",  
"token_type":"bearer",  
"expires_in":3600,  
"refresh_token":"eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJnbnQiOjIsInR0IjoxLCJjbnQiOjEsImV4cCI6MTU1NzgwNDMxMiwiaWF0IjoxNTU1MTc2MzEyfQ.S_HZQBy4q9r5SEzNGNIoFClT43HPNDbUdHH-GYNYYdkRfft6XptJBkUQscZsGxOW975Yk6RbgtGvq1nkEcklOw"  
}
```

`CLIENT_SECRET` - это уникальный секретный код, созданный для этого приложения. Обратите внимание, что секрет будет виден только после того, как вы создадите/зарегистрируете приложение в Gitea, и не может быть восстановлен. Если вы потеряете секрет, вам необходимо восстановить секрет в настройках приложения.

`REDIRECT_URI` в запросе `access_token` должен совпадать с `REDIRECT_URI` в запросе authorize.

3. Используйте `access_token`, чтобы сделать [Запросы API](https://docs.gitea.io/ru-ru/api-usage#oauth2) для доступа к ресурсам пользователя.
