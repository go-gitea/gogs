---
date: "2017-01-01T16:00:00+02:00"
title: "Использование: Командная строка"
slug: "command-line"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Командная строка"
    weight: 10
    identifier: "command-line"
---

## Командная строка

### Использование

`gitea [глобальные параметры] команда [команда или глобальные параметры] [аргументы...]`

### Глобальные параметры

Все глобальные параметры могут быть размещены на уровне команд.

- `--help`, `-h`: Показать справку и выйти. По желанию.
- `--version`, `-v`: Показать версию и выйти. По желанию. (пример: `Gitea version 1.1.0+218-g7b907ed built with: bindata, sqlite`).
- `--custom-path path`, `-C path`: Расположение пользовательской папки Gitea. По желанию. (по умолчанию: `AppWorkPath`/custom или `$GITEA_CUSTOM`).
- `--config path`, `-c path`: Путь к файлу конфигурации Gitea. По желанию. (по умолчанию: `custom`/conf/app.ini).
- `--work-path path`, `-w path`: Gitea `AppWorkPath`. По желанию. (по умолчанию: LOCATION_OF_GITEA_BINARY или `$GITEA_WORK_DIR`)

NB: значения по умолчанию custom-path, config и work-path 
также могут быть изменены во время сборки (при желании).

### Команды

#### сеть

Запуск сервера:

- Параметры:
    - `--port number`, `-p number`: Номер порта. По желанию. (по умолчанию: 3000). Заменяет файл конфигурации.
    - `--pid path`, `-P path`: Путь к pidfile. По желанию.
- Примеры:
    - `gitea web`
    - `gitea web --port 80`
    - `gitea web --config /etc/gitea.ini --pid /some/custom/gitea.pid`
- Примечания:
    - Gitea не следует запускать с правами root. Чтобы привязаться к порту ниже 1024, 
	  вы можете использовать setcap на Linux: `sudo setcap 'cap_net_bind_service=+ep' /path/to/gitea`. 
	  Это нужно будет переделывать каждый раз при обновлении Gitea.

#### админ

Административные операции:

- Команды:
    - `create-user`
        - Параметры:
            - `--name value`: Имя пользователя. Необходимые. Начиная с gitea 1.9.0, используйте вместо него флаг `--username`.
            - `--username value`: Имя пользователя. Необходимые. Новое в gitea 1.9.0.
            - `--password value`: Пароль. Необходимо.
            - `--email value`: Эл. адрес. Необходимо.
            - `--admin`: Если предоставляется, это делает пользователя администратором. По желанию.
            - `--access-token`: Если предоставляется, для пользователя будет создан токен доступа. По желанию. (по умолчанию: false).
            - `--must-change-password`: Если указано, созданный пользователь должен будет выбрать новый пароль после
		первоначального входа в систему. По желанию. (по умолчанию: true).
            - ``--random-password``: Если предоставлено, случайно сгенерированный пароль будет использоваться в качестве
		пароля созданного пользователя. Значение `--password` будет отброшено. По желанию.
            - `--random-password-length`: Если предоставляется, он будет использоваться для настройки длины случайно
		сгенерированного пароля. По желанию. (по умолчанию: 12)
        - Примеры:
            - `gitea admin create-user --username myname --password asecurepassword --email me@example.com`
    - `change-password`
        - Параметры:
            - `--username value`, `-u value`: Имя пользователя. Необходимо.
            - `--password value`, `-p value`: Новый пароль. Необходимо.
        - Примеры:
            - `gitea admin change-password --username myname --password asecurepassword`
    - `regenerate`
        - Параметры:
            - `hooks`: Восстановить git-hook'и для всех репозиториев
            - `keys`: Восстановить файл authorized_keys
        - Примеры:
            - `gitea admin regenerate hooks`
            - `gitea admin regenerate keys`
    - `auth`:
        - `list`:
            - Описание: перечисляет все существующие внешние источники аутентификации
            - Примеры:
                - `gitea admin auth list`
        - `delete`:
            - Параметры:
                - `--id`: ID удаляемого источника. Необходимо.
            - Примеры:
                - `gitea admin auth delete --id 1`
        - `add-oauth`:
            - Параметры:
                - `--name`: Имя приложения.
                - `--provider`: Провайдер OAuth2.
                - `--key`: ID клиента (ключ).
                - `--secret`: Секрет клиента.
                - `--auto-discover-url`: URL-адрес автоматического обнаружения OpenID Connect (требуется только при использовании OpenID Connect в качестве провайдера).
                - `--use-custom-urls`: Использует настраиваемые URL-адреса для конечных точек GitLab/GitHub OAuth.
                - `--custom-auth-url`: Использует настраиваемый URL-адрес авторизации (опция для GitLab/GitHub).
                - `--custom-token-url`: Использует собственный URL-адрес токена (опция для GitLab/GitHub).
                - `--custom-profile-url`: Использует настраиваемый URL-адрес профиля (опция для GitLab/GitHub).
                - `--custom-email-url`: Использует настраиваемый URL-адрес электронной почты (опция для GitHub).
            - Примеры:
                - `gitea admin auth add-oauth --name external-github --provider github --key OBTAIN_FROM_SOURCE --secret OBTAIN_FROM_SOURCE`
        - `update-oauth`:
            - Параметры:
                - `--id`: Идентификатор обновляемого источника. Необходимо.
                - `--name`: Имя приложения.
                - `--provider`: Провайдер OAuth2.
                - `--key`: Идентификатор клиента (ключ).
                - `--secret`: Секрет клиента.
                - `--auto-discover-url`: URL-адрес автоматического обнаружения OpenID Connect (требуется только при использовании OpenID Connect в качестве провайдера).
                - `--use-custom-urls`: Использует настраиваемые URL-адреса для конечных точек GitLab/GitHub OAuth.
                - `--custom-auth-url`: Использует настраиваемый URL-адрес авторизации (опция для GitLab/GitHub).
                - `--custom-token-url`: Использует собственный URL-адрес токена (опция для GitLab/GitHub).
                - `--custom-profile-url`: Использует настраиваемый URL-адрес профиля (опция для GitLab/GitHub).
                - `--custom-email-url`: Использует настраиваемый URL-адрес электронной почты (опция для GitHub).
            - Примеры:
                - `gitea admin auth update-oauth --id 1 --name external-github-updated`
        - `add-ldap`: Добавить новый источник аутентификации LDAP (через привязку DN)
            - Параметры:
                - `--name value`: Имя для аутентификации. Необходимо.
                - `--not-active`: Деактивировать источник аутентификации.
                - `--security-protocol value`: Имя протокола безопасности. Необходимо.
                - `--skip-tls-verify`: Отключить проверку TLS.
                - `--host value`: Адрес, по которому можно связаться с сервером LDAP. Необходимо.
                - `--port value`: Порт, используемый при подключении к серверу LDAP. Необходимо.
                - `--user-search-base value`: База LDAP, в которой будет выполняться поиск учётных записей пользователей. Необходимо.
                - `--user-filter value`: Фильтр LDAP, объявляющий, как найти запись пользователя, которая пытается аутентифицироваться. Необходимо.
                - `--admin-filter value`: Фильтр LDAP, определяющий, следует ли предоставлять пользователю права администратора.
                - `--restricted-filter value`: Фильтр LDAP, определяющий, следует ли предоставить пользователю статус с ограничениями.
                - `--username-attribute value`: Атрибут записи LDAP пользователя, содержащий имя пользователя.
                - `--firstname-attribute value`: Атрибут записи LDAP пользователя, содержащий имя пользователя.
                - `--surname-attribute value`: Атрибут записи LDAP пользователя, содержащий его фамилию.
                - `--email-attribute value`: Атрибут записи LDAP пользователя, содержащий адрес электронной почты пользователя. Необходимо.
                - `--public-ssh-key-attribute value`: Атрибут записи LDAP пользователя, содержащий открытый ключ ssh пользователя.
                - `--bind-dn value`: DN для привязки к серверу LDAP при поиске пользователя.
                - `--bind-password value`: Пароль для Привязки DN, если есть.
                - `--attributes-in-bind`: Получить атрибуты в контексте привязки DN.
                - `--synchronize-users`: Включить синхронизацию пользователей.
                - `--page-size value`: Размер страницы поиска.
            - Примеры:
                - `gitea admin auth add-ldap --name ldap --security-protocol unencrypted --host mydomain.org --port 389 --user-search-base "ou=Users,dc=mydomain,dc=org" --user-filter "(&(objectClass=posixAccount)(uid=%s))" --email-attribute mail`
        - `update-ldap`: Обновить существующий источник аутентификации LDAP (через привязку DN)
            - Параметры:
                - `--id value`: ID of authentication source. Required.
                - `--name value`: Authentication name.
                - `--not-active`: Deactivate the authentication source.
                - `--security-protocol value`: Security protocol name.
                - `--skip-tls-verify`: Disable TLS verification.
                - `--host value`: The address where the LDAP server can be reached.
                - `--port value`: The port to use when connecting to the LDAP server.
                - `--user-search-base value`: The LDAP base at which user accounts will be searched for.
                - `--user-filter value`: An LDAP filter declaring how to find the user record that is attempting to authenticate.
                - `--admin-filter value`: An LDAP filter specifying if a user should be given administrator privileges.
                - `--restricted-filter value`: An LDAP filter specifying if a user should be given restricted status.
                - `--username-attribute value`: The attribute of the user’s LDAP record containing the user name.
                - `--firstname-attribute value`: The attribute of the user’s LDAP record containing the user’s first name.
                - `--surname-attribute value`: The attribute of the user’s LDAP record containing the user’s surname.
                - `--email-attribute value`: The attribute of the user’s LDAP record containing the user’s email address.
                - `--public-ssh-key-attribute value`: The attribute of the user’s LDAP record containing the user’s public ssh key.
                - `--bind-dn value`: The DN to bind to the LDAP server with when searching for the user.
                - `--bind-password value`: The password for the Bind DN, if any.
                - `--attributes-in-bind`: Fetch attributes in bind DN context.
                - `--synchronize-users`: Enable user synchronization.
                - `--page-size value`: Search page size.
            - Примеры:
                - `gitea admin auth update-ldap --id 1 --name "my ldap auth source"`
                - `gitea admin auth update-ldap --id 1 --username-attribute uid --firstname-attribute givenName --surname-attribute sn`
        - `add-ldap-simple`: Add new LDAP (simple auth) authentication source
            - Параметры:
                - `--name value`: Authentication name. Required.
                - `--not-active`: Deactivate the authentication source.
                - `--security-protocol value`: Security protocol name. Required.
                - `--skip-tls-verify`: Disable TLS verification.
                - `--host value`: The address where the LDAP server can be reached. Required.
                - `--port value`: The port to use when connecting to the LDAP server. Required.
                - `--user-search-base value`: The LDAP base at which user accounts will be searched for.
                - `--user-filter value`: An LDAP filter declaring how to find the user record that is attempting to authenticate. Required.
                - `--admin-filter value`: An LDAP filter specifying if a user should be given administrator privileges.
                - `--restricted-filter value`: An LDAP filter specifying if a user should be given restricted status.
                - `--username-attribute value`: The attribute of the user’s LDAP record containing the user name.
                - `--firstname-attribute value`: The attribute of the user’s LDAP record containing the user’s first name.
                - `--surname-attribute value`: The attribute of the user’s LDAP record containing the user’s surname.
                - `--email-attribute value`: The attribute of the user’s LDAP record containing the user’s email address. Required.
                - `--public-ssh-key-attribute value`: The attribute of the user’s LDAP record containing the user’s public ssh key.
                - `--user-dn value`: The user’s DN. Required.
            - Примеры:
                - `gitea admin auth add-ldap-simple --name ldap --security-protocol unencrypted --host mydomain.org --port 389 --user-dn "cn=%s,ou=Users,dc=mydomain,dc=org" --user-filter "(&(objectClass=posixAccount)(cn=%s))" --email-attribute mail`
        - `update-ldap-simple`: Update existing LDAP (simple auth) authentication source
            - Параметры:
                - `--id value`: ID of authentication source. Required.
                - `--name value`: Authentication name.
                - `--not-active`: Deactivate the authentication source.
                - `--security-protocol value`: Security protocol name.
                - `--skip-tls-verify`: Disable TLS verification.
                - `--host value`: The address where the LDAP server can be reached.
                - `--port value`: The port to use when connecting to the LDAP server.
                - `--user-search-base value`: The LDAP base at which user accounts will be searched for.
                - `--user-filter value`: An LDAP filter declaring how to find the user record that is attempting to authenticate.
                - `--admin-filter value`: An LDAP filter specifying if a user should be given administrator privileges.
                - `--restricted-filter value`: An LDAP filter specifying if a user should be given restricted status.
                - `--username-attribute value`: The attribute of the user’s LDAP record containing the user name.
                - `--firstname-attribute value`: The attribute of the user’s LDAP record containing the user’s first name.
                - `--surname-attribute value`: The attribute of the user’s LDAP record containing the user’s surname.
                - `--email-attribute value`: The attribute of the user’s LDAP record containing the user’s email address.
                - `--public-ssh-key-attribute value`: The attribute of the user’s LDAP record containing the user’s public ssh key.
                - `--user-dn value`: The user’s DN.
            - Примеры:
                - `gitea admin auth update-ldap-simple --id 1 --name "my ldap auth source"`
                - `gitea admin auth update-ldap-simple --id 1 --username-attribute uid --firstname-attribute givenName --surname-attribute sn`

#### сертификат

Создаёт самоподписанный сертификат SSL. Выводит в `cert.pem` и` key.pem`
в текущем каталоге и перезаписывает все существующие файлы.

- Параметры:
    - `--host value`: Разделенные запятыми имена хостов и IPS, для которых действует этот сертификат.
      Подстановочные знаки поддерживаются. Необходимо.
    - `--ecdsa-curve value`: Кривая ECDSA для генерации ключа. По желанию. Допустимые
      варианты P224, P256, P384, P521.
    - `--rsa-bits value`: Размер генерируемого ключа RSA. По желанию. Игнорируется, если задано --ecdsa-curve.
	(по умолчанию: 2048).
    - `--start-date value`: Дата создания. По желанию. (формат: `Jan 1 15:04:05 2011`).
    - `--duration value`: Срок действия сертификата. По желанию. (по умолчанию: 8760h0m0s)
    - `--ca`: Если предоставляется, этот сертификат генерирует собственный центр сертификации. По желанию.
- Примеры:
    - `gitea cert --host git.example.com,example.com,www.example.com --ca`

#### дамп

Сохраняет все файлы и базы данных в zip-архив. Выводится в файл типа `gitea-dump-1482906742.zip`
в текущий каталог.

- Параметры:
    - `--file name`, `-f name`: Будет создано имя файла дампа с расширением. По желанию. (по умолчанию: gitea-dump-[timestamp].zip).
    - `--tempdir path`, `-t path`: Путь к используемому временному каталогу. По желанию. (по умолчанию: /tmp).
    - `--skip-repository`, `-R`: Пропустить дампинг репозитория. По желанию.
    - `--database`, `-d`: Укажите синтаксис SQL базы данных. По желанию.
    - `--verbose`, `-V`: Если предоставляется, отображаются дополнительные сведения. По желанию.
- Примеры:
    - `gitea dump`
    - `gitea dump --verbose`

#### генерация

Создаёт случайные значения и токены для использования в файле конфигурации. Полезно для генерации значений
для автоматического развёртывания.

- Команды:
    - `secret`:
        - Параметры:
            - `INTERNAL_TOKEN`: Токен, используемый для внутренней аутентификации вызова API.
            - `JWT_SECRET`: LFS & OAUTH2 Секрет аутентификации JWT (LFS_JWT_SECRET является псевдонимом для этой опции для обратной совместимости).
            - `SECRET_KEY`: Глобальный секретный ключ.
        - Примеры:
            - `gitea generate secret INTERNAL_TOKEN`
            - `gitea generate secret JWT_SECRET`
            - `gitea generate secret SECRET_KEY`

#### ключи

Предоставляет SSHD AuthorizedKeysCommand. Необходимо настроить в файле конфигурации sshd:

```ini
...
# The value of -e and the AuthorizedKeysCommandUser should match the
# username running gitea
AuthorizedKeysCommandUser git
AuthorizedKeysCommand /path/to/gitea keys -e git -u %u -t %t -k %k
```

Команда вернёт соответствующую строку authorized_keys для
предоставленного ключа. Вы также должны установить значение
`SSH_CREATE_AUTHORIZED_KEYS_FILE=false` в разделе `[server]` в
`app.ini`.

NB: opensshd требует, чтобы программа gitea принадлежала пользователю root и не
была доступна для записи группе или другим лицам. Программа должна быть указана
по абсолютному пути.
NB: Gitea должна быть запущена для выполнения этой команды.

#### мигрирация
Переносит базу данных. Эту команду можно использовать для запуска других команд перед первым запуском сервера.
Эта команда идемпотентна.

#### конвертация
Преобразует существующую базу данных MySQL из utf8 в utf8mb4.

#### доктор
Диагностируйте проблемы текущего экземпляра gitea в соответствии с заданной конфигурацией.
В настоящее время есть контрольный список ниже:

- Проверьте правильность идентификатора файла authorized_keys OpenSSH
Когда ваш экземпляр gitea поддерживает OpenSSH, двоичный путь к вашему экземпляру gitea будет записан в `authorized_keys` 
когда в вашем экземпляре gitea добавлен или изменен какой-либо открытый ключ.
Иногда, если вы переместили или переименовали свой двоичный файл gitea при обновлении, но не запустили `Обновите файл '.ssh/authorized_keys' с ключами Gitea SSH. (Не требуется для встроенного SSH-сервера.)` oв вашей панели администратора. Тогда все pull/push через SSH работать не будут.
Эта проверка поможет вам проверить, работает ли он правильно.

Для участников, если вы хотите добавить больше проверок, вы можете написать добавить новую функцию, например `func(ctx *cli.Context) ([]string, error)` и
добавьте его в `doctor.go`.

```go
var checklist = []check{
	{
		title: "Check if OpenSSH authorized_keys file id correct",
		f:     runDoctorLocationMoved,
    },
    // more checks please append here
}
```

Эта функция получит контекст командной строки и вернёт список деталей о проблемах или ошибках.

#### менеджер

Управление запущенными серверными операциями:

- Команды:
  - `shutdown`:      Изящно завершите запущенный процесс
  - `restart`:       Изящно перезапустите запущенный процесс - (не реализовано для серверов Windows)
  - `flush-queues`:  Очистить очереди в запущенном процессе
    - Параметры:
      - `--timeout value`: Тайм-аут для процесса промывки (по умолчанию: 1м0с)
      - `--non-blocking`: Установите значение true, чтобы не ждать завершения сброса перед возвратом
  - `logging`:       Настройте команды ведения журнала
    - Команды:
      - `pause`:   Приостановить ведение журнала
        - Примечания:
          - Уровень ведения журнала будет временно повышен до INFO, если он ниже этого уровня.
          - Gitea будет буферизовать журналы до определённого момента и сбросить их после этого момента.
      - `resume`:  Возобновить ведение журнала
      - `release-and-reopen`: Заставить Gitea освободить и повторно открыть файлы и соединения, используемые для ведения журнала (эквивалент отправки SIGUSR1 в Gitea).
      - `remove name`: Удалить названный логгер
        - Параметры:
          - `--group group`, `-g group`: Задайте группу, из которой нужно удалить сублоггера. (по умолчанию `default`)
      - `add`:     Добавить логгер
        - Команды:
          - `console`: Добавить логгер консоли
            - Параметры:
              - `--group value`, `-g value`: Группа для добавления логгера - по умолчанию будет "default"
              - `--name value`, `-n value`: Имя нового логгера - по умолчанию будет режим
              - `--level value`, `-l value`: Уровень ведения журнала для нового логгера
              - `--stacktrace-level value`, `-L value`: Уровень ведения журнала Stacktrace
              - `--flags value`, `-F value`: Флаги для логгера
              - `--expression value`, `-e value`: Выражение соответствия для логгера
              - `--prefix value`, `-p value`: Префикс для логгера
              - `--color`: Используйте цвет в журналах
              - `--stderr`: Вывод журналов консоли в stderr - актуально только для консоли
          - `file`: Добавить логгер файлов
            - Параметры:
              - `--group value`, `-g value`: Группа для добавления логгера - по умолчанию будет "default"
              - `--name value`, `-n value`:  Имя нового логгера - по умолчанию будет режим
              - `--level value`, `-l value`: Уровень ведения журнала для нового логгера
              - `--stacktrace-level value`, `-L value`: Уровень ведения журнала Stacktrace
              - `--flags value`, `-F value`: Флаги для логгера
              - `--expression value`, `-e value`: Выражение соответствия для логгера
              - `--prefix value`, `-p value`: Префикс для логгера
              - `--color`: Используйте цвет в журналах
              - `--filename value`, `-f value`: Имя файла для логгера - 
              - `--rotate`, `-r`: Повернуть журналы
              - `--max-size value`, `-s value`: Максимальный размер в байтах до поворота
              - `--daily`, `-d`: Ежедневно поворачивать журналы
              - `--max-days value`, `-D value`: Максимальное количество ежедневных журналов для хранения
              - `--compress`, `-z`: Сжать повёрнутые журналы
              - `--compression-level value`, `-Z value`: Уровень сжатия для использования
          - `conn`: Добавить логгер сетевых подключений
            - Параметры:
              - `--group value`, `-g value`: Группа для добавления логгера - по умолчанию будет "default"
              - `--name value`, `-n value`:  Имя нового логгера - по умолчанию будет режим
              - `--level value`, `-l value`: Уровень ведения журнала для нового логгера
              - `--stacktrace-level value`, `-L value`: Уровень ведения журнала Stacktrace
              - `--flags value`, `-F value`: Флаги для логгера
              - `--expression value`, `-e value`: Выражение соответствия для логгера
              - `--prefix value`, `-p value`: Префикс для логгера
              - `--color`: Используйте цвет в журналах
              - `--reconnect-on-message`, `-R`: Повторно подключаться к хосту для каждого сообщения
              - `--reconnect`, `-r`: Подключиться к хосту при разрыве соединения
              - `--protocol value`, `-P value`: Установите протокол для использования: tcp, unix или udp (по умолчанию tcp)
              - `--address value`, `-a value`: Адрес хоста и порт для подключения (по умолчанию: 7020)
          - `smtp`: Добавить логгер SMTP
            - Параметры:
              - `--group value`, `-g value`: Группа для добавления логгера - по умолчанию будет "default"
              - `--name value`, `-n value`: Имя нового логгера - по умолчанию будет режим
              - `--level value`, `-l value`: Уровень ведения журнала для нового логгера
              - `--stacktrace-level value`, `-L value`: Уровень ведения журнала Stacktrace
              - `--flags value`, `-F value`: Флаги для логгера
              - `--expression value`, `-e value`: Выражение соответствия для логгера
              - `--prefix value`, `-p value`: Префикс для логгера
              - `--color`: Используйте цвет в журналах
              - `--username value`, `-u value`: Имя пользователя почтового сервера
              - `--password value`, `-P value`: Пароль почтового сервера
              - `--host value`, `-H value`: Хост почтового сервера (по умолчанию: 127.0.0.1:25)
              - `--send-to value`, `-s value`: Электронный(е) адрес(а) для отправки
              - `--subject value`, `-S value`: Заголовок темы отправленных писем
