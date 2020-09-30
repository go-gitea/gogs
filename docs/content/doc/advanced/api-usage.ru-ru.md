---
date: "2018-06-24:00:00+02:00"
title: "Использование API"
slug: "api-usage"
weight: 40
toc: true
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Использование API"
    weight: 40
    identifier: "api-usage"
---

# Использование Gitea API

## Включение/настройка доступа к API

По умолчанию, `ENABLE_SWAGGER` true, и
`MAX_RESPONSE_ITEMS` установлено на 50. См. [Памятку по
конфигурации](https://docs.gitea.io/ru-ru/config-cheat-sheet/) для большей
информации.

## Аутентификация через API

Gitea поддерживает эти методы аутентификации API:

- Базовая аутентификация HTTP
- `token=...` параметр в строке запроса URL
- `access_token=...` параметр в строке запроса URL
- `Authorization: token ...` заголовок в заголовках HTTP

Все эти методы принимают один и тот же тип токена ключа API. Вы можете
лучше понять это, посмотрев на код - на момент написания Gitea анализирует
запросы и заголовки, чтобы найти токен в
[modules/auth/auth.go](https://github.com/go-gitea/gitea/blob/6efdcaed86565c91a3dc77631372a9cc45a58e89/modules/auth/auth.go#L47).

Вы можете создать токен ключа API через веб-интерфейс вашей установки Gitea.:
`Настройки | Приложения | Создать новый токен`.

### OAuth2

Жетоны доступа, полученные от Gitea [OAuth2 provider](https://docs.gitea.io/en-us/oauth2-provider) принимаются этими методами:

- `Authorization bearer ...` заголовок в заголовках HTTP
- `token=...` параметр в строке запроса URL
- `access_token=...` параметр в строке запроса URL

### Подробнее о заголовке `Authorization:`

По историческим причинам Gitea нуждается в слове `token` включеным перед
токеном ключа API в заголовок авторизации, например:

```
Authorization: token 65eaa9c8ef52460d22a93307fe0aee76289dc675
```

Например, в команде `curl` это будет выглядеть так:

```
curl -X POST "http://localhost:4000/api/v1/repos/test1/test1/issues" \
    -H "accept: application/json" \
    -H "Authorization: token 65eaa9c8ef52460d22a93307fe0aee76289dc675" \
    -H "Content-Type: application/json" -d "{ \"body\": \"testing\", \"title\": \"test 20\"}" -i
```

Как упоминалось выше, используется тот же токен, который вы использовали бы
в строке `token=` в запросе GET.

## Руководство API:

Справочное руководство по API создаётся автоматически с помощью Swagger и доступно на: 
    `https://gitea.your.host/api/swagger`
    или в 
    [демонстрационном экземпляре gitea](https://try.gitea.io/api/swagger)


## Листинг ваших выпущенных токенов через API

Как упоминалось в
[#3842](https://github.com/go-gitea/gitea/issues/3842#issuecomment-397743346),
`/users/:name/tokens` является особенным и требует от вас аутентификации
с помощью BasicAuth, как показано ниже:

### Использование базовой аутентификации(BasicAuth):

```
$ curl --request GET --url https://yourusername:yourpassword@gitea.your.host/api/v1/users/yourusername/tokens
[{"name":"test","sha1":"..."},{"name":"dev","sha1":"..."}]
```

Начиная с версии 1.8.0 Gitea, если вы используете базовую аутентификацию с API и у вашего пользователя включена двухфакторная аутентификация, вам нужно будет отправить дополнительный заголовок, содержащий одноразовый пароль (6-значный ротационный токен). Примером заголовка является `X-Gitea-OTP: 123456`, где`123456` - это место, куда вы поместите код своего аутентификатора. Вот как будет выглядеть запрос в curl:

```
$ curl -H "X-Gitea-OTP: 123456" --request GET --url https://yourusername:yourpassword@gitea.your.host/api/v1/users/yourusername/tokens
```

## Sudo

API позволяет администраторам выполнять запросы к sudo API от имени другого пользователя. Просто добавьте либо параметр `sudo=`, либо заголовок запроса `Sudo:` с именем пользователя в sudo.
