IMPORT := xorm.io/xorm
export GO111MODULE=on

GO ?= go
GOFMT ?= gofmt -s
TAGS ?=
SED_INPLACE := sed -i

GOFILES := $(shell find . -name "*.go" -type f)
INTEGRATION_PACKAGES := xorm.io/xorm/integrations
PACKAGES ?= $(filter-out $(INTEGRATION_PACKAGES),$(shell $(GO) list ./...))

TEST_COCKROACH_HOST ?= cockroach:26257
TEST_COCKROACH_SCHEMA ?=
TEST_COCKROACH_DBNAME ?= xorm_test
TEST_COCKROACH_USERNAME ?= postgres
TEST_COCKROACH_PASSWORD ?=

TEST_MSSQL_HOST ?= mssql:1433
TEST_MSSQL_DBNAME ?= gitea
TEST_MSSQL_USERNAME ?= sa
TEST_MSSQL_PASSWORD ?= MwantsaSecurePassword1
TEST_MSSQL_DEFAULT_VARCHAR ?= varchar
TEST_MSSQL_DEFAULT_CHAR ?= char
TEST_MSSQL_DO_NVARCHAR_OVERRIDE_TEST ?= true

TEST_MYSQL_HOST ?= mysql:3306
TEST_MYSQL_CHARSET ?= utf8
TEST_MYSQL_DBNAME ?= xorm_test
TEST_MYSQL_USERNAME ?= root
TEST_MYSQL_PASSWORD ?=

TEST_PGSQL_HOST ?= pgsql:5432
TEST_PGSQL_SCHEMA ?=
TEST_PGSQL_DBNAME ?= xorm_test
TEST_PGSQL_USERNAME ?= postgres
TEST_PGSQL_PASSWORD ?= mysecretpassword

TEST_TIDB_HOST ?= tidb:4000
TEST_TIDB_DBNAME ?= xorm_test
TEST_TIDB_USERNAME ?= root
TEST_TIDB_PASSWORD ?=

TEST_CACHE_ENABLE ?= false
TEST_QUOTE_POLICY ?= always

.PHONY: all
all: build

.PHONY: build
build: go-check $(GO_SOURCES)
	$(GO) build $(PACKAGES)

.PHONY: clean
clean:
	$(GO) clean -i ./...
	rm -rf *.sql *.log test.db *coverage.out coverage.all integrations/*.sql

.PHONY: coverage
coverage:
	@hash gocovmerge > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/wadey/gocovmerge; \
	fi
	gocovmerge $(shell find . -type f -name "coverage.out") > coverage.all;\

.PHONY: fmt
fmt:
	$(GOFMT) -w $(GOFILES)

.PHONY: fmt-check
fmt-check:
	# get all go files and run go fmt on them
	@diff=$$($(GOFMT) -d $(GOFILES)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: go-check
go-check:
	$(eval GO_VERSION := $(shell printf "%03d%03d%03d" $(shell go version | grep -Eo '[0-9]+\.?[0-9]+?\.?[0-9]?\s' | tr '.' ' ');))
	@if [ "$(GO_VERSION)" -lt "001011000" ]; then \
		echo "Gitea requires Go 1.11.0 or greater to build. You can get it at https://golang.org/dl/"; \
		exit 1; \
	fi

.PHONY: help
help:
	@echo "Make Routines:"
	@echo " -                   equivalent to \"build\""
	@echo " - build             creates the entire project"
	@echo " - clean             delete integration files and build files but not css and js files"
	@echo " - fmt               format the code"
	@echo " - lint            	run code linter revive"
	@echo " - misspell          check if a word is written wrong"
	@echo " - test       		run default unit test"
	@echo " - test-cockroach    run integration tests for cockroach"
	@echo " - test-mysql        run integration tests for mysql"
	@echo " - test-mssql        run integration tests for mssql"
	@echo " - test-postgres     run integration tests for postgres"
	@echo " - test-sqlite3      run integration tests for sqlite"
	@echo " - test-sqlite       run integration tests for pure go sqlite"
	@echo " - test-tidb         run integration tests for tidb"
	@echo " - vet               examines Go source code and reports suspicious constructs"

.PHONY: lint
lint: revive

.PHONY: revive
revive:
	@hash revive > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/mgechev/revive; \
	fi
	revive -config .revive.toml -exclude=./vendor/... ./... || exit 1

.PHONY: misspell
misspell:
	@hash misspell > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/client9/misspell/cmd/misspell; \
	fi
	misspell -w -i unknwon $(GOFILES)

.PHONY: misspell-check
misspell-check:
	@hash misspell > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/client9/misspell/cmd/misspell; \
	fi
	misspell -error -i unknwon,destory $(GOFILES)

.PHONY: test
test: go-check
	$(GO) test $(PACKAGES)

.PNONY: test-cockroach
test-cockroach: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -db=postgres -schema='$(TEST_COCKROACH_SCHEMA)' -cache=$(TEST_CACHE_ENABLE) \
	-conn_str="postgres://$(TEST_COCKROACH_USERNAME):$(TEST_COCKROACH_PASSWORD)@$(TEST_COCKROACH_HOST)/$(TEST_COCKROACH_DBNAME)?sslmode=disable&experimental_serial_normalization=sql_sequence" \
	-ignore_update_limit=true -coverprofile=cockroach.$(TEST_COCKROACH_SCHEMA).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PHONY: test-cockroach\#%
test-cockroach\#%: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -run $* -db=postgres -schema='$(TEST_COCKROACH_SCHEMA)' -cache=$(TEST_CACHE_ENABLE) \
	-conn_str="postgres://$(TEST_COCKROACH_USERNAME):$(TEST_COCKROACH_PASSWORD)@$(TEST_COCKROACH_HOST)/$(TEST_COCKROACH_DBNAME)?sslmode=disable&experimental_serial_normalization=sql_sequence" \
	-ignore_update_limit=true -coverprofile=cockroach.$(TEST_COCKROACH_SCHEMA).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PNONY: test-mssql
test-mssql: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -db=mssql -cache=$(TEST_CACHE_ENABLE) -quote=$(TEST_QUOTE_POLICY) \
	-conn_str="server=$(TEST_MSSQL_HOST);user id=$(TEST_MSSQL_USERNAME);password=$(TEST_MSSQL_PASSWORD);database=$(TEST_MSSQL_DBNAME)" \
	-default_varchar=$(TEST_MSSQL_DEFAULT_VARCHAR) -default_char=$(TEST_MSSQL_DEFAULT_CHAR) \
	-do_nvarchar_override_test=$(TEST_MSSQL_DO_NVARCHAR_OVERRIDE_TEST) \
	-coverprofile=mssql.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PNONY: test-mssql\#%
test-mssql\#%: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -run $* -db=mssql -cache=$(TEST_CACHE_ENABLE) -quote=$(TEST_QUOTE_POLICY) \
	-conn_str="server=$(TEST_MSSQL_HOST);user id=$(TEST_MSSQL_USERNAME);password=$(TEST_MSSQL_PASSWORD);database=$(TEST_MSSQL_DBNAME)" \
	-default_varchar=$(TEST_MSSQL_DEFAULT_VARCHAR) -default_char=$(TEST_MSSQL_DEFAULT_CHAR) \
	-do_nvarchar_override_test=$(TEST_MSSQL_DO_NVARCHAR_OVERRIDE_TEST) \
	-coverprofile=mssql.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PNONY: test-mymysql
test-mymysql: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -db=mymysql -cache=$(TEST_CACHE_ENABLE) -quote=$(TEST_QUOTE_POLICY) \
	-conn_str="tcp:$(TEST_MYSQL_HOST)*$(TEST_MYSQL_DBNAME)/$(TEST_MYSQL_USERNAME)/$(TEST_MYSQL_PASSWORD)" \
	-coverprofile=mymysql.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PNONY: test-mymysql\#%
test-mymysql\#%: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -run $* -db=mymysql -cache=$(TEST_CACHE_ENABLE) -quote=$(TEST_QUOTE_POLICY) \
	-conn_str="tcp:$(TEST_MYSQL_HOST)*$(TEST_MYSQL_DBNAME)/$(TEST_MYSQL_USERNAME)/$(TEST_MYSQL_PASSWORD)" \
	-coverprofile=mymysql.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PNONY: test-mysql
test-mysql: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -db=mysql -cache=$(TEST_CACHE_ENABLE) -quote=$(TEST_QUOTE_POLICY) \
	-conn_str="$(TEST_MYSQL_USERNAME):$(TEST_MYSQL_PASSWORD)@tcp($(TEST_MYSQL_HOST))/$(TEST_MYSQL_DBNAME)?charset=$(TEST_MYSQL_CHARSET)" \
	-coverprofile=mysql.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PHONY: test-mysql\#%
test-mysql\#%: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -run $* -db=mysql -cache=$(TEST_CACHE_ENABLE) -quote=$(TEST_QUOTE_POLICY) \
	-conn_str="$(TEST_MYSQL_USERNAME):$(TEST_MYSQL_PASSWORD)@tcp($(TEST_MYSQL_HOST))/$(TEST_MYSQL_DBNAME)?charset=$(TEST_MYSQL_CHARSET)" \
	-coverprofile=mysql.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PNONY: test-postgres
test-postgres: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -db=postgres -schema='$(TEST_PGSQL_SCHEMA)' -cache=$(TEST_CACHE_ENABLE) \
	-conn_str="postgres://$(TEST_PGSQL_USERNAME):$(TEST_PGSQL_PASSWORD)@$(TEST_PGSQL_HOST)/$(TEST_PGSQL_DBNAME)?sslmode=disable" \
	-quote=$(TEST_QUOTE_POLICY) -coverprofile=postgres.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PHONY: test-postgres\#%
test-postgres\#%: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -run $* -db=postgres -schema='$(TEST_PGSQL_SCHEMA)' -cache=$(TEST_CACHE_ENABLE) \
	-conn_str="postgres://$(TEST_PGSQL_USERNAME):$(TEST_PGSQL_PASSWORD)@$(TEST_PGSQL_HOST)/$(TEST_PGSQL_DBNAME)?sslmode=disable" \
	-quote=$(TEST_QUOTE_POLICY) -coverprofile=postgres.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PHONY: test-sqlite3
test-sqlite3: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -cache=$(TEST_CACHE_ENABLE) -db=sqlite3 -conn_str="./test.db?cache=shared&mode=rwc" \
	 -quote=$(TEST_QUOTE_POLICY) -coverprofile=sqlite3.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PHONY: test-sqlite3-schema
test-sqlite3-schema: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -schema=xorm -cache=$(TEST_CACHE_ENABLE) -db=sqlite3 -conn_str="./test.db?cache=shared&mode=rwc" \
	 -quote=$(TEST_QUOTE_POLICY) -coverprofile=sqlite3.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PHONY: test-sqlite3\#%
test-sqlite3\#%: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -run $* -cache=$(TEST_CACHE_ENABLE) -db=sqlite3 -conn_str="./test.db?cache=shared&mode=rwc" \
	 -quote=$(TEST_QUOTE_POLICY) -coverprofile=sqlite3.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PHONY: test-sqlite
test-sqlite: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -cache=$(TEST_CACHE_ENABLE) -db=sqlite -conn_str="./test.db?cache=shared&mode=rwc" \
	 -quote=$(TEST_QUOTE_POLICY) -coverprofile=sqlite.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PHONY: test-sqlite-schema
test-sqlite-schema: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -schema=xorm -cache=$(TEST_CACHE_ENABLE) -db=sqlite -conn_str="./test.db?cache=shared&mode=rwc" \
	 -quote=$(TEST_QUOTE_POLICY) -coverprofile=sqlite.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PHONY: test-sqlite\#%
test-sqlite\#%: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -run $* -cache=$(TEST_CACHE_ENABLE) -db=sqlite -conn_str="./test.db?cache=shared&mode=rwc" \
	 -quote=$(TEST_QUOTE_POLICY) -coverprofile=sqlite.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic


.PNONY: test-tidb
test-tidb: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -db=mysql -cache=$(TEST_CACHE_ENABLE) -ignore_select_update=true \
	-conn_str="$(TEST_TIDB_USERNAME):$(TEST_TIDB_PASSWORD)@tcp($(TEST_TIDB_HOST))/$(TEST_TIDB_DBNAME)" \
	-quote=$(TEST_QUOTE_POLICY) -coverprofile=tidb.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PHONY: test-tidb\#%
test-tidb\#%: go-check
	$(GO) test $(INTEGRATION_PACKAGES) -v -race -run $* -db=mysql -cache=$(TEST_CACHE_ENABLE) -ignore_select_update=true \
	-conn_str="$(TEST_TIDB_USERNAME):$(TEST_TIDB_PASSWORD)@tcp($(TEST_TIDB_HOST))/$(TEST_TIDB_DBNAME)" \
	-quote=$(TEST_QUOTE_POLICY) -coverprofile=tidb.$(TEST_QUOTE_POLICY).$(TEST_CACHE_ENABLE).coverage.out -covermode=atomic

.PHONY: vet
vet:
	$(GO) vet $(shell $(GO) list ./...)