[English](README.md)

# Интеграционные тесты

Интеграционные тесты можно запускать с помощью команд make
для соответствующих задних частей, а именно:
```shell
make test-mysql
make test-pgsql
make test-sqlite
```

Обязательно выполните чистую сборку перед запуском тестов:
```
make clean build
```

## Запустить все тесты через локальный дрон
```
drone exec --local --build-event "pull_request"
```

## Запуск теста интеграции sqlite
Старт теста
```
make test-sqlite
```

## Запустите тесты интеграции mysql
Настройка базы данных mysql внутри docker
```
docker run -e "MYSQL_DATABASE=test" -e "MYSQL_ALLOW_EMPTY_PASSWORD=yes" -p 3306:3306 --rm --name mysql mysql:latest #(just ctrl-c to stop db and clean the container)
docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" --rm --name elasticsearch elasticsearch:7.6.0 #(in a secound terminal, just ctrl-c to stop db and clean the container)
```
Запуск тестов на основе контейнера базы данных
```
TEST_MYSQL_HOST=localhost:3306 TEST_MYSQL_DBNAME=test TEST_MYSQL_USERNAME=root TEST_MYSQL_PASSWORD='' make test-mysql
```

## Запустите тесты интеграции pgsql
Настройте базу данных pgsql внутри docker
```
docker run -e "POSTGRES_DB=test" -p 5432:5432 --rm --name pgsql postgres:latest #(just ctrl-c to stop db and clean the container)
```
Запуск тестов на основе контейнера базы данных
```
TEST_PGSQL_HOST=localhost:5432 TEST_PGSQL_DBNAME=test TEST_PGSQL_USERNAME=postgres TEST_PGSQL_PASSWORD=postgres make test-pgsql
```

## Запустите тесты интеграции mssql
Настройте базу данных mssql внутри docker
```
docker run -e "ACCEPT_EULA=Y" -e "MSSQL_PID=Standard" -e "SA_PASSWORD=MwantsaSecurePassword1" -p 1433:1433 --rm --name mssql microsoft/mssql-server-linux:latest #(just ctrl-c to stop db and clean the container)
```
Запуск тестов на основе контейнера базы данных
```
TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=gitea_test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-mssql
```

## Проведение индивидуальных тестов

Пример команды для запуска теста GPG:

Для sqlite:

```
make test-sqlite#GPG
```

Для других баз данных (замените MSSQL на MYSQL, MYSQL8, PGSQL):

```
TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-mssql#GPG
```

## Установка таймаутов для объявления длинных тестов и длинных сбросов

Мы понимаем, что некоторые тестовые машины могут быть не очень мощными,
и тайм-ауты по умолчанию для объявления медленного теста или медленной
очистки могут не подходить.

Вы также можете:

* В тестовом ini файле установите следующий раздел:

```ini
[integration-tests]
SLOW_TEST = 10s ; 10s is the default value
SLOW_FLUSH = 5S ; 5s is the default value
```

* Установите следующие переменные среды:

```bash
GITEA_SLOW_TEST_TIME="10s" GITEA_SLOW_FLUSH_TIME="5s" make test-sqlite
```
