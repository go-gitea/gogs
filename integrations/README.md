# Integrations tests

Integration tests can be run with make commands for the
appropriate backends, namely:
```shell
make test-mysql
make test-pgsql
make test-sqlite
```

Make sure to perform a clean build before running tests:
```
make clean build
```

## Run all tests via local drone
```
drone exec --local --build-event "pull_request"
```

## Run sqlite integrations tests
Start tests
```
make test-sqlite
```

## Run mysql integrations tests
Setup a mysql database inside docker
```
docker run -e "MYSQL_DATABASE=test" -e "MYSQL_ALLOW_EMPTY_PASSWORD=yes" -p 3306:3306 --rm --name mysql mysql:latest #(just ctrl-c to stop db and clean the container)
docker run -p 9200:9200 -p 9300:9300 -e "discovery.type=single-node" --rm --name elasticsearch elasticsearch:7.6.0 #(in a second terminal, just ctrl-c to stop db and clean the container)
```
Start tests based on the database container
```
TEST_MYSQL_HOST=localhost:3306 TEST_MYSQL_DBNAME=test TEST_MYSQL_USERNAME=root TEST_MYSQL_PASSWORD='' make test-mysql
```

## Run pgsql integrations tests
Setup a pgsql database inside docker
```
docker run -e "POSTGRES_DB=test" -p 5432:5432 --rm --name pgsql postgres:latest #(just ctrl-c to stop db and clean the container)
```
Start tests based on the database container
```
TEST_PGSQL_HOST=localhost:5432 TEST_PGSQL_DBNAME=test TEST_PGSQL_USERNAME=postgres TEST_PGSQL_PASSWORD=postgres make test-pgsql
```

## Run mssql integrations tests
Setup a mssql database inside docker
```
docker run -e "ACCEPT_EULA=Y" -e "MSSQL_PID=Standard" -e "SA_PASSWORD=MwantsaSecurePassword1" -p 1433:1433 --rm --name mssql microsoft/mssql-server-linux:latest #(just ctrl-c to stop db and clean the container)
```
Start tests based on the database container
```
TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=gitea_test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-mssql
```

## Running individual tests

Example command to run GPG test:

For sqlite:

```
make test-sqlite#GPG
```

For other databases(replace MSSQL to MYSQL, MYSQL8, PGSQL):

```
TEST_MSSQL_HOST=localhost:1433 TEST_MSSQL_DBNAME=test TEST_MSSQL_USERNAME=sa TEST_MSSQL_PASSWORD=MwantsaSecurePassword1 make test-mssql#GPG
```

## Setting timeouts for declaring long-tests and long-flushes

We appreciate that some testing machines may not be very powerful and
the default timeouts for declaring a slow test or a slow clean-up flush
may not be appropriate.

You can either:

* Within the test ini file set the following section:

```ini
[integration-tests]
SLOW_TEST = 10s ; 10s is the default value
SLOW_FLUSH = 5S ; 5s is the default value
```

* Set the following environment variables:

```bash
GITEA_SLOW_TEST_TIME="10s" GITEA_SLOW_FLUSH_TIME="5s" make test-sqlite
```
