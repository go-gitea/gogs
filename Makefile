ifeq ($(USE_REPO_TEST_DIR),1)

# This rule replaces the whole Makefile when we're trying to use /tmp repository temporary files
location = $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST))
self := $(location)

%:
	@tmpdir=`mktemp --tmpdir -d` ; \
	echo Using temporary directory $$tmpdir for test repositories ; \
	USE_REPO_TEST_DIR= $(MAKE) -f $(self) --no-print-directory REPO_TEST_DIR=$$tmpdir/ $@ ; \
	STATUS=$$? ; rm -r "$$tmpdir" ; exit $$STATUS

else

# This is the "normal" part of the Makefile

DIST := dist
DIST_DIRS := $(DIST)/binaries $(DIST)/release
IMPORT := code.gitea.io/gitea
export GO111MODULE=on

GO ?= go
SHASUM ?= shasum -a 256
HAS_GO = $(shell hash $(GO) > /dev/null 2>&1 && echo "GO" || echo "NOGO" )
COMMA := ,

XGO_VERSION := go-1.16.x
MIN_GO_VERSION := 001014000
MIN_NODE_VERSION := 012017000

DOCKER_IMAGE ?= gitea/gitea
DOCKER_TAG ?= latest
DOCKER_REF := $(DOCKER_IMAGE):$(DOCKER_TAG)

ifeq ($(HAS_GO), GO)
	GOPATH ?= $(shell $(GO) env GOPATH)
	export PATH := $(GOPATH)/bin:$(PATH)

	CGO_EXTRA_CFLAGS := -DSQLITE_MAX_VARIABLE_NUMBER=32766
	CGO_CFLAGS ?= $(shell $(GO) env CGO_CFLAGS) $(CGO_EXTRA_CFLAGS)
endif

ifeq ($(OS), Windows_NT)
	GOFLAGS := -v -buildmode=exe
	EXECUTABLE ?= gitea.exe
else ifeq ($(OS), Windows)
	GOFLAGS := -v -buildmode=exe
	EXECUTABLE ?= gitea.exe
else
	GOFLAGS := -v
	EXECUTABLE ?= gitea
endif

ifeq ($(shell sed --version 2>/dev/null | grep -q GNU && echo gnu),gnu)
	SED_INPLACE := sed -i
else
	SED_INPLACE := sed -i ''
endif

GOFMT ?= gofmt -s

EXTRA_GOFLAGS ?=

MAKE_VERSION := $(shell $(MAKE) -v | head -n 1)
MAKE_EVIDENCE_DIR := .make_evidence

ifeq ($(RACE_ENABLED),true)
	GOFLAGS += -race
	GOTESTFLAGS += -race
endif

STORED_VERSION_FILE := VERSION

ifneq ($(DRONE_TAG),)
	VERSION ?= $(subst v,,$(DRONE_TAG))
	GITEA_VERSION ?= $(VERSION)
else
	ifneq ($(DRONE_BRANCH),)
		VERSION ?= $(subst release/v,,$(DRONE_BRANCH))
	else
		VERSION ?= main
	endif

	STORED_VERSION=$(shell cat $(STORED_VERSION_FILE) 2>/dev/null)
	ifneq ($(STORED_VERSION),)
		GITEA_VERSION ?= $(STORED_VERSION)
	else
		GITEA_VERSION ?= $(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')
	endif
endif

LDFLAGS := $(LDFLAGS) -X "main.MakeVersion=$(MAKE_VERSION)" -X "main.Version=$(GITEA_VERSION)" -X "main.Tags=$(TAGS)"

LINUX_ARCHS ?= linux/amd64,linux/386,linux/arm-5,linux/arm-6,linux/arm64

GO_PACKAGES ?= $(filter-out code.gitea.io/gitea/models/migrations code.gitea.io/gitea/integrations/migration-test code.gitea.io/gitea/integrations,$(shell $(GO) list -mod=vendor ./... | grep -v /vendor/))

FOMANTIC_WORK_DIR := web_src/fomantic

WEBPACK_SOURCES := $(shell find web_src/js web_src/less -type f)
WEBPACK_CONFIGS := webpack.config.js
WEBPACK_DEST := public/js/index.js public/css/index.css
WEBPACK_DEST_ENTRIES := public/js public/css public/fonts public/img/webpack public/serviceworker.js

BINDATA_DEST := modules/public/bindata.go modules/options/bindata.go modules/templates/bindata.go
BINDATA_HASH := $(addsuffix .hash,$(BINDATA_DEST))

SVG_DEST_DIR := public/img/svg

AIR_TMP_DIR := .air

TAGS ?=
TAGS_SPLIT := $(subst $(COMMA), ,$(TAGS))
TAGS_EVIDENCE := $(MAKE_EVIDENCE_DIR)/tags

TEST_TAGS ?= sqlite sqlite_unlock_notify

TAR_EXCLUDES := .git data indexers queues log node_modules $(EXECUTABLE) $(FOMANTIC_WORK_DIR)/node_modules $(DIST) $(MAKE_EVIDENCE_DIR) $(AIR_TMP_DIR)

GO_DIRS := cmd integrations models modules routers build services vendor tools

GO_SOURCES := $(wildcard *.go)
GO_SOURCES += $(shell find $(GO_DIRS) -type f -name "*.go" -not -path modules/options/bindata.go -not -path modules/public/bindata.go -not -path modules/templates/bindata.go)

ifeq ($(filter $(TAGS_SPLIT),bindata),bindata)
	GO_SOURCES += $(BINDATA_DEST)
endif

GO_SOURCES_OWN := $(filter-out vendor/% %/bindata.go, $(GO_SOURCES))

#To update swagger use: GO111MODULE=on go get -u github.com/go-swagger/go-swagger/cmd/swagger
SWAGGER := $(GO) run -mod=vendor github.com/go-swagger/go-swagger/cmd/swagger
SWAGGER_SPEC := templates/swagger/v1_json.tmpl
SWAGGER_SPEC_S_TMPL := s|"basePath": *"/api/v1"|"basePath": "{{AppSubUrl \| JSEscape \| Safe}}/api/v1"|g
SWAGGER_SPEC_S_JSON := s|"basePath": *"{{AppSubUrl \| JSEscape \| Safe}}/api/v1"|"basePath": "/api/v1"|g
SWAGGER_EXCLUDE := code.gitea.io/sdk
SWAGGER_NEWLINE_COMMAND := -e '$$a\'

TEST_MYSQL_HOST ?= mysql:3306
TEST_MYSQL_DBNAME ?= testgitea
TEST_MYSQL_USERNAME ?= root
TEST_MYSQL_PASSWORD ?=
TEST_MYSQL8_HOST ?= mysql8:3306
TEST_MYSQL8_DBNAME ?= testgitea
TEST_MYSQL8_USERNAME ?= root
TEST_MYSQL8_PASSWORD ?=
TEST_PGSQL_HOST ?= pgsql:5432
TEST_PGSQL_DBNAME ?= testgitea
TEST_PGSQL_USERNAME ?= postgres
TEST_PGSQL_PASSWORD ?= postgres
TEST_PGSQL_SCHEMA ?= gtestschema
TEST_MSSQL_HOST ?= mssql:1433
TEST_MSSQL_DBNAME ?= gitea
TEST_MSSQL_USERNAME ?= sa
TEST_MSSQL_PASSWORD ?= MwantsaSecurePassword1

.PHONY: all
all: build

.PHONY: help
help:
	@echo "Make Routines:"
	@echo " - \"\"                               equivalent to \"build\""
	@echo " - build                            build everything"
	@echo " - frontend                         build frontend files"
	@echo " - backend                          build backend files"
	@echo " - watch                            watch everything and continuously rebuild"
	@echo " - watch-frontend                   watch frontend files and continuously rebuild"
	@echo " - watch-backend                    watch backend files and continuously rebuild"
	@echo " - clean                            delete backend and integration files"
	@echo " - clean-all                        delete backend, frontend and integration files"
	@echo " - lint                             lint everything"
	@echo " - lint-frontend                    lint frontend files"
	@echo " - lint-backend                     lint backend files"
	@echo " - checks                           run various consistency checks"
	@echo " - checks-frontend                  check frontend files"
	@echo " - checks-backend                   check backend files"
	@echo " - test                             test everything"
	@echo " - test-frontend                    test frontend files"
	@echo " - test-backend                     test backend files"
	@echo " - webpack                          build webpack files"
	@echo " - svg                              build svg files"
	@echo " - fomantic                         build fomantic files"
	@echo " - generate                         run \"go generate\""
	@echo " - fmt                              format the Go code"
	@echo " - generate-license                 update license files"
	@echo " - generate-gitignore               update gitignore files"
	@echo " - generate-manpage                 generate manpage"
	@echo " - generate-swagger                 generate the swagger spec from code comments"
	@echo " - swagger-validate                 check if the swagger spec is valid"
	@echo " - golangci-lint                    run golangci-lint linter"
	@echo " - revive                           run revive linter"
	@echo " - misspell                         check for misspellings"
	@echo " - vet                              examines Go source code and reports suspicious constructs"
	@echo " - test[\#TestSpecificName]    	    run unit test"
	@echo " - test-sqlite[\#TestSpecificName]  run integration test for sqlite"
	@echo " - pr#<index>                       build and start gitea from a PR with integration test data loaded"

.PHONY: go-check
go-check:
	$(eval GO_VERSION := $(shell printf "%03d%03d%03d" $(shell $(GO) version | grep -Eo '[0-9]+\.[0-9.]+' | tr '.' ' ');))
	@if [ "$(GO_VERSION)" -lt "$(MIN_GO_VERSION)" ]; then \
		echo "Gitea requires Go 1.14 or greater to build. You can get it at https://golang.org/dl/"; \
		exit 1; \
	fi

.PHONY: git-check
git-check:
	@if git lfs >/dev/null 2>&1 ; then : ; else \
		echo "Gitea requires git with lfs support to run tests." ; \
		exit 1; \
	fi

.PHONY: node-check
node-check:
	$(eval NODE_VERSION := $(shell printf "%03d%03d%03d" $(shell node -v | cut -c2- | tr '.' ' ');))
	$(eval MIN_NODE_VER_FMT := $(shell printf "%g.%g.%g" $(shell echo $(MIN_NODE_VERSION) | grep -o ...)))
	$(eval NPM_MISSING := $(shell hash npm > /dev/null 2>&1 || echo 1))
	@if [ "$(NODE_VERSION)" -lt "$(MIN_NODE_VERSION)" -o "$(NPM_MISSING)" = "1" ]; then \
		echo "Gitea requires Node.js $(MIN_NODE_VER_FMT) or greater and npm to build. You can get it at https://nodejs.org/en/download/"; \
		exit 1; \
	fi

.PHONY: clean-all
clean-all: clean
	rm -rf $(WEBPACK_DEST_ENTRIES) node_modules

.PHONY: clean
clean:
	$(GO) clean -i ./...
	rm -rf $(EXECUTABLE) $(DIST) $(BINDATA_DEST) $(BINDATA_HASH) \
		integrations*.test \
		integrations/gitea-integration-pgsql/ integrations/gitea-integration-mysql/ integrations/gitea-integration-mysql8/ integrations/gitea-integration-sqlite/ \
		integrations/gitea-integration-mssql/ integrations/indexers-mysql/ integrations/indexers-mysql8/ integrations/indexers-pgsql integrations/indexers-sqlite \
		integrations/indexers-mssql integrations/mysql.ini integrations/mysql8.ini integrations/pgsql.ini integrations/mssql.ini man/

.PHONY: fmt
fmt:
	@echo "Running go fmt..."
	@$(GOFMT) -w $(GO_SOURCES_OWN)

.PHONY: vet
vet:
	@echo "Running go vet..."
	@$(GO) vet $(GO_PACKAGES)
	@GOOS= GOARCH= $(GO) build -mod=vendor code.gitea.io/gitea-vet
	@$(GO) vet -vettool=gitea-vet $(GO_PACKAGES)

.PHONY: $(TAGS_EVIDENCE)
$(TAGS_EVIDENCE):
	@mkdir -p $(MAKE_EVIDENCE_DIR)
	@echo "$(TAGS)" > $(TAGS_EVIDENCE)

ifneq "$(TAGS)" "$(shell cat $(TAGS_EVIDENCE) 2>/dev/null)"
TAGS_PREREQ := $(TAGS_EVIDENCE)
endif

.PHONY: generate-swagger
generate-swagger:
	$(SWAGGER) generate spec -x "$(SWAGGER_EXCLUDE)" -o './$(SWAGGER_SPEC)'
	$(SED_INPLACE) '$(SWAGGER_SPEC_S_TMPL)' './$(SWAGGER_SPEC)'
	$(SED_INPLACE) $(SWAGGER_NEWLINE_COMMAND) './$(SWAGGER_SPEC)'

.PHONY: swagger-check
swagger-check: generate-swagger
	@diff=$$(git diff '$(SWAGGER_SPEC)'); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make generate-swagger' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi

.PHONY: swagger-validate
swagger-validate:
	$(SED_INPLACE) '$(SWAGGER_SPEC_S_JSON)' './$(SWAGGER_SPEC)'
	$(SWAGGER) validate './$(SWAGGER_SPEC)'
	$(SED_INPLACE) '$(SWAGGER_SPEC_S_TMPL)' './$(SWAGGER_SPEC)'

.PHONY: errcheck
errcheck:
	@hash errcheck > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		GO111MODULE=off $(GO) get -u github.com/kisielk/errcheck; \
	fi
	@echo "Running errcheck..."
	@errcheck $(GO_PACKAGES)

.PHONY: revive
revive:
	@hash revive > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		GO111MODULE=off $(GO) get -u github.com/mgechev/revive; \
	fi
	@revive -config .revive.toml -exclude=./vendor/... ./...

.PHONY: misspell-check
misspell-check:
	@hash misspell > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		GO111MODULE=off $(GO) get -u github.com/client9/misspell/cmd/misspell; \
	fi
	@echo "Running misspell-check..."
	@misspell -error -i unknwon $(GO_SOURCES_OWN)

.PHONY: misspell
misspell:
	@hash misspell > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		GO111MODULE=off $(GO) get -u github.com/client9/misspell/cmd/misspell; \
	fi
	@echo "Running go misspell..."
	@misspell -w -i unknwon $(GO_SOURCES_OWN)

.PHONY: fmt-check
fmt-check:
	# get all go files and run go fmt on them
	@diff=$$($(GOFMT) -d $(GO_SOURCES_OWN)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi

.PHONY: checks
checks: checks-frontend checks-backend

.PHONY: checks-frontend
checks-frontend: svg-check

.PHONY: checks-backend
checks-backend: misspell-check test-vendor swagger-check swagger-validate

.PHONY: lint
lint: lint-frontend lint-backend

.PHONY: lint-frontend
lint-frontend: node_modules
	npx eslint --color --max-warnings=0 web_src/js build templates *.config.js
	npx stylelint --color --max-warnings=0 web_src/less
	npx editorconfig-checker templates

.PHONY: lint-backend
lint-backend: golangci-lint revive vet

.PHONY: watch
watch:
	bash tools/watch.sh

.PHONY: watch-frontend
watch-frontend: node-check node_modules
	rm -rf $(WEBPACK_DEST_ENTRIES)
	NODE_ENV=development npx webpack --watch --progress

.PHONY: watch-backend
watch-backend: go-check
	@hash air > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		GO111MODULE=off $(GO) get -u github.com/cosmtrek/air; \
	fi
	air -c .air.conf

.PHONY: test
test: test-frontend test-backend

.PHONY: test-backend
test-backend:
	@echo "Running go test with $(GOTESTFLAGS) -tags '$(TEST_TAGS)'..."
	@$(GO) test $(GOTESTFLAGS) -mod=vendor -tags='$(TEST_TAGS)' $(GO_PACKAGES)

.PHONY: test-frontend
test-frontend: node_modules
	@NODE_OPTIONS="--experimental-vm-modules --no-warnings" npx jest --color

.PHONY: test-check
test-check:
	@echo "Running test-check...";
	@diff=$$(git status -s); \
	if [ -n "$$diff" ]; then \
		echo "make test-backend has changed files in the source tree:"; \
		echo "$${diff}"; \
		echo "You should change the tests to create these files in a temporary directory."; \
		echo "Do not simply add these files to .gitignore"; \
		exit 1; \
	fi

.PHONY: test\#%
test\#%:
	@echo "Running go test with -tags '$(TEST_TAGS)'..."
	@$(GO) test -mod=vendor $(GOTESTFLAGS) -tags='$(TEST_TAGS)' -run $(subst .,/,$*) $(GO_PACKAGES)

.PHONY: coverage
coverage:
	GO111MODULE=on $(GO) run -mod=vendor build/gocovmerge.go integration.coverage.out $(shell find . -type f -name "coverage.out") > coverage.all

.PHONY: unit-test-coverage
unit-test-coverage:
	@echo "Running unit-test-coverage $(GOTESTFLAGS) -tags '$(TEST_TAGS)'..."
	@$(GO) test $(GOTESTFLAGS) -mod=vendor -tags='$(TEST_TAGS)' -cover -coverprofile coverage.out $(GO_PACKAGES) && echo "\n==>\033[32m Ok\033[m\n" || exit 1

.PHONY: vendor
vendor:
	$(GO) mod tidy && $(GO) mod vendor

.PHONY: test-vendor
test-vendor: vendor
	@diff=$$(git diff vendor/); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make vendor' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi

generate-ini-sqlite:
	sed -e 's|{{REPO_TEST_DIR}}|${REPO_TEST_DIR}|g' \
			integrations/sqlite.ini.tmpl > integrations/sqlite.ini

.PHONY: test-sqlite
test-sqlite: integrations.sqlite.test generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test

.PHONY: test-sqlite\#%
test-sqlite\#%: integrations.sqlite.test generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test -test.run $(subst .,/,$*)

.PHONY: test-sqlite-migration
test-sqlite-migration:  migrations.sqlite.test migrations.individual.sqlite.test generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/sqlite.ini ./migrations.sqlite.test
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/sqlite.ini ./migrations.individual.sqlite.test

generate-ini-mysql:
	sed -e 's|{{TEST_MYSQL_HOST}}|${TEST_MYSQL_HOST}|g' \
		-e 's|{{TEST_MYSQL_DBNAME}}|${TEST_MYSQL_DBNAME}|g' \
		-e 's|{{TEST_MYSQL_USERNAME}}|${TEST_MYSQL_USERNAME}|g' \
		-e 's|{{TEST_MYSQL_PASSWORD}}|${TEST_MYSQL_PASSWORD}|g' \
		-e 's|{{REPO_TEST_DIR}}|${REPO_TEST_DIR}|g' \
			integrations/mysql.ini.tmpl > integrations/mysql.ini

.PHONY: test-mysql
test-mysql: integrations.mysql.test generate-ini-mysql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mysql.ini ./integrations.mysql.test

.PHONY: test-mysql\#%
test-mysql\#%: integrations.mysql.test generate-ini-mysql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mysql.ini ./integrations.mysql.test -test.run $(subst .,/,$*)

.PHONY: test-mysql-migration
test-mysql-migration: migrations.mysql.test migrations.individual.mysql.test generate-ini-mysql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mysql.ini ./migrations.mysql.test
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mysql.ini ./migrations.individual.mysql.test

generate-ini-mysql8:
	sed -e 's|{{TEST_MYSQL8_HOST}}|${TEST_MYSQL8_HOST}|g' \
		-e 's|{{TEST_MYSQL8_DBNAME}}|${TEST_MYSQL8_DBNAME}|g' \
		-e 's|{{TEST_MYSQL8_USERNAME}}|${TEST_MYSQL8_USERNAME}|g' \
		-e 's|{{TEST_MYSQL8_PASSWORD}}|${TEST_MYSQL8_PASSWORD}|g' \
		-e 's|{{REPO_TEST_DIR}}|${REPO_TEST_DIR}|g' \
			integrations/mysql8.ini.tmpl > integrations/mysql8.ini

.PHONY: test-mysql8
test-mysql8: integrations.mysql8.test generate-ini-mysql8
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mysql8.ini ./integrations.mysql8.test

.PHONY: test-mysql8\#%
test-mysql8\#%: integrations.mysql8.test generate-ini-mysql8
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mysql8.ini ./integrations.mysql8.test -test.run $(subst .,/,$*)

.PHONY: test-mysql8-migration
test-mysql8-migration: migrations.mysql8.test migrations.individual.mysql8.test generate-ini-mysql8
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mysql8.ini ./migrations.mysql8.test
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mysql8.ini ./migrations.individual.mysql8.test

generate-ini-pgsql:
	sed -e 's|{{TEST_PGSQL_HOST}}|${TEST_PGSQL_HOST}|g' \
		-e 's|{{TEST_PGSQL_DBNAME}}|${TEST_PGSQL_DBNAME}|g' \
		-e 's|{{TEST_PGSQL_USERNAME}}|${TEST_PGSQL_USERNAME}|g' \
		-e 's|{{TEST_PGSQL_PASSWORD}}|${TEST_PGSQL_PASSWORD}|g' \
		-e 's|{{TEST_PGSQL_SCHEMA}}|${TEST_PGSQL_SCHEMA}|g' \
		-e 's|{{REPO_TEST_DIR}}|${REPO_TEST_DIR}|g' \
			integrations/pgsql.ini.tmpl > integrations/pgsql.ini

.PHONY: test-pgsql
test-pgsql: integrations.pgsql.test generate-ini-pgsql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/pgsql.ini ./integrations.pgsql.test

.PHONY: test-pgsql\#%
test-pgsql\#%: integrations.pgsql.test generate-ini-pgsql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/pgsql.ini ./integrations.pgsql.test -test.run $(subst .,/,$*)

.PHONY: test-pgsql-migration
test-pgsql-migration: migrations.pgsql.test migrations.individual.pgsql.test generate-ini-pgsql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/pgsql.ini ./migrations.pgsql.test
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/pgsql.ini ./migrations.individual.pgsql.test

generate-ini-mssql:
	sed -e 's|{{TEST_MSSQL_HOST}}|${TEST_MSSQL_HOST}|g' \
		-e 's|{{TEST_MSSQL_DBNAME}}|${TEST_MSSQL_DBNAME}|g' \
		-e 's|{{TEST_MSSQL_USERNAME}}|${TEST_MSSQL_USERNAME}|g' \
		-e 's|{{TEST_MSSQL_PASSWORD}}|${TEST_MSSQL_PASSWORD}|g' \
		-e 's|{{REPO_TEST_DIR}}|${REPO_TEST_DIR}|g' \
			integrations/mssql.ini.tmpl > integrations/mssql.ini

.PHONY: test-mssql
test-mssql: integrations.mssql.test generate-ini-mssql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mssql.ini ./integrations.mssql.test

.PHONY: test-mssql\#%
test-mssql\#%: integrations.mssql.test generate-ini-mssql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mssql.ini ./integrations.mssql.test -test.run $(subst .,/,$*)

.PHONY: test-mssql-migration
test-mssql-migration: migrations.mssql.test migrations.individual.mssql.test generate-ini-mssql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mssql.ini ./migrations.mssql.test -test.failfast
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mssql.ini ./migrations.individual.mssql.test -test.failfast

.PHONY: bench-sqlite
bench-sqlite: integrations.sqlite.test generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: bench-mysql
bench-mysql: integrations.mysql.test generate-ini-mysql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mysql.ini ./integrations.mysql.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: bench-mssql
bench-mssql: integrations.mssql.test generate-ini-mssql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mssql.ini ./integrations.mssql.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: bench-pgsql
bench-pgsql: integrations.pgsql.test generate-ini-pgsql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/pgsql.ini ./integrations.pgsql.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: integration-test-coverage
integration-test-coverage: integrations.cover.test generate-ini-mysql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=integrations/mysql.ini ./integrations.cover.test -test.coverprofile=integration.coverage.out

integrations.mysql.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -mod=vendor -c code.gitea.io/gitea/integrations -o integrations.mysql.test

integrations.mysql8.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -mod=vendor -c code.gitea.io/gitea/integrations -o integrations.mysql8.test

integrations.pgsql.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -mod=vendor -c code.gitea.io/gitea/integrations -o integrations.pgsql.test

integrations.mssql.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -mod=vendor -c code.gitea.io/gitea/integrations -o integrations.mssql.test

integrations.sqlite.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -mod=vendor -c code.gitea.io/gitea/integrations -o integrations.sqlite.test -tags '$(TEST_TAGS)'

integrations.cover.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -mod=vendor -c code.gitea.io/gitea/integrations -coverpkg $(shell echo $(GO_PACKAGES) | tr ' ' ',') -o integrations.cover.test

.PHONY: migrations.mysql.test
migrations.mysql.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/integrations/migration-test -o migrations.mysql.test

.PHONY: migrations.mysql8.test
migrations.mysql8.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/integrations/migration-test -o migrations.mysql8.test

.PHONY: migrations.pgsql.test
migrations.pgsql.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/integrations/migration-test -o migrations.pgsql.test

.PHONY: migrations.mssql.test
migrations.mssql.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/integrations/migration-test -o migrations.mssql.test

.PHONY: migrations.sqlite.test
migrations.sqlite.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/integrations/migration-test -o migrations.sqlite.test -tags '$(TEST_TAGS)'

.PHONY: migrations.individual.mysql.test
migrations.individual.mysql.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/models/migrations -o migrations.individual.mysql.test

.PHONY: migrations.individual.mysql8.test
migrations.individual.mysql8.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/models/migrations -o migrations.individual.mysql8.test

.PHONY: migrations.individual.pgsql.test
migrations.individual.pgsql.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/models/migrations -o migrations.individual.pgsql.test

.PHONY: migrations.individual.mssql.test
migrations.individual.mssql.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/models/migrations -o migrations.individual.mssql.test

.PHONY: migrations.individual.sqlite.test
migrations.individual.sqlite.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/models/migrations -o migrations.individual.sqlite.test -tags '$(TEST_TAGS)'

.PHONY: check
check: test

.PHONY: install $(TAGS_PREREQ)
install: $(wildcard *.go)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) install -v -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)'

.PHONY: build
build: frontend backend

.PHONY: frontend
frontend: $(WEBPACK_DEST)

.PHONY: backend
backend: go-check generate $(EXECUTABLE)

.PHONY: generate
generate: $(TAGS_PREREQ)
	@echo "Running go generate..."
	@CC= GOOS= GOARCH= $(GO) generate -mod=vendor -tags '$(TAGS)' $(GO_PACKAGES)

$(EXECUTABLE): $(GO_SOURCES) $(TAGS_PREREQ)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) build -mod=vendor $(GOFLAGS) $(EXTRA_GOFLAGS) -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)' -o $@

.PHONY: release
release: frontend generate release-windows release-linux release-darwin release-copy release-compress release-sources release-docs release-check

$(DIST_DIRS):
	mkdir -p $(DIST_DIRS)

.PHONY: release-windows
release-windows: | $(DIST_DIRS)
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) install src.techknowlogick.com/xgo@latest; \
	fi
	CGO_CFLAGS="$(CGO_CFLAGS)" xgo -go $(XGO_VERSION) -buildmode exe -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets 'windows/*' -out gitea-$(VERSION) .
ifeq (,$(findstring gogit,$(TAGS)))
	CGO_CFLAGS="$(CGO_CFLAGS)" xgo -go $(XGO_VERSION) -buildmode exe -dest $(DIST)/binaries -tags 'netgo osusergo gogit $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets 'windows/*' -out gitea-$(VERSION)-gogit .
endif
ifeq ($(CI),drone)
	cp /build/* $(DIST)/binaries
endif

.PHONY: release-linux
release-linux: | $(DIST_DIRS)
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) install src.techknowlogick.com/xgo@latest; \
	fi
	CGO_CFLAGS="$(CGO_CFLAGS)" xgo -go $(XGO_VERSION) -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets '$(LINUX_ARCHS)' -out gitea-$(VERSION) .
ifeq ($(CI),drone)
	cp /build/* $(DIST)/binaries
endif

.PHONY: release-darwin
release-darwin: | $(DIST_DIRS)
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) install src.techknowlogick.com/xgo@latest; \
	fi
	CGO_CFLAGS="$(CGO_CFLAGS)" xgo -go $(XGO_VERSION) -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '$(LDFLAGS)' -targets 'darwin-10.12/amd64,darwin-10.12/arm64' -out gitea-$(VERSION) .
ifeq ($(CI),drone)
	cp /build/* $(DIST)/binaries
endif

.PHONY: release-copy
release-copy: | $(DIST_DIRS)
	cd $(DIST); for file in `find /build -type f -name "*"`; do cp $${file} ./release/; done;

.PHONY: release-check
release-check: | $(DIST_DIRS)
	cd $(DIST)/release/; for file in `find . -type f -name "*"`; do echo "checksumming $${file}" && $(SHASUM) `echo $${file} | sed 's/^..//'` > $${file}.sha256; done;

.PHONY: release-compress
release-compress: | $(DIST_DIRS)
	@hash gxz > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		GO111MODULE=off $(GO) get -u github.com/ulikunitz/xz/cmd/gxz; \
	fi
	cd $(DIST)/release/; for file in `find . -type f -name "*"`; do echo "compressing $${file}" && gxz -k -9 $${file}; done;

.PHONY: release-sources
release-sources: | $(DIST_DIRS)
	echo $(VERSION) > $(STORED_VERSION_FILE)
# bsdtar needs a ^ to prevent matching subdirectories
	$(eval EXCL := --exclude=$(shell tar --help | grep -q bsdtar && echo "^")./)
	tar $(addprefix $(EXCL),$(TAR_EXCLUDES)) -czf $(DIST)/release/gitea-src-$(VERSION).tar.gz .
	rm -f $(STORED_VERSION_FILE)

.PHONY: release-docs
release-docs: | $(DIST_DIRS) docs
	tar -czf $(DIST)/release/gitea-docs-$(VERSION).tar.gz -C ./docs/public .

.PHONY: docs
docs:
	@hash hugo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		curl -sL https://github.com/gohugoio/hugo/releases/download/v0.74.3/hugo_0.74.3_Linux-64bit.tar.gz | tar zxf - -C /tmp && mv /tmp/hugo /usr/bin/hugo && chmod +x /usr/bin/hugo; \
	fi
	cd docs; make trans-copy clean build-offline;

node_modules: package-lock.json
	npm install --no-save
	@touch node_modules

.PHONY: npm-update
npm-update: node-check | node_modules
	npx updates -cu
	rm -rf node_modules package-lock.json
	npm install --package-lock
	@touch node_modules

.PHONY: fomantic
fomantic:
	rm -rf $(FOMANTIC_WORK_DIR)/build
	cd $(FOMANTIC_WORK_DIR) && npm install --no-save
	cp -f $(FOMANTIC_WORK_DIR)/theme.config.less $(FOMANTIC_WORK_DIR)/node_modules/fomantic-ui/src/theme.config
	cp -rf $(FOMANTIC_WORK_DIR)/_site $(FOMANTIC_WORK_DIR)/node_modules/fomantic-ui/src/
	cd $(FOMANTIC_WORK_DIR) && npx gulp -f node_modules/fomantic-ui/gulpfile.js build

.PHONY: webpack
webpack: $(WEBPACK_DEST)

$(WEBPACK_DEST): $(WEBPACK_SOURCES) $(WEBPACK_CONFIGS) package-lock.json
	@$(MAKE) -s node-check node_modules
	rm -rf $(WEBPACK_DEST_ENTRIES)
	npx webpack
	@touch $(WEBPACK_DEST)

.PHONY: svg
svg: node-check | node_modules
	rm -rf $(SVG_DEST_DIR)
	node build/generate-svg.js

.PHONY: svg-check
svg-check: svg
	@git add $(SVG_DEST_DIR)
	@diff=$$(git diff --cached $(SVG_DEST_DIR)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make svg' and 'git add $(SVG_DEST_DIR)' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi

.PHONY: update-translations
update-translations:
	mkdir -p ./translations
	cd ./translations && curl -L https://crowdin.com/download/project/gitea.zip > gitea.zip && unzip gitea.zip
	rm ./translations/gitea.zip
	$(SED_INPLACE) -e 's/="/=/g' -e 's/"$$//g' ./translations/*.ini
	$(SED_INPLACE) -e 's/\\"/"/g' ./translations/*.ini
	mv ./translations/*.ini ./options/locale/
	rmdir ./translations

.PHONY: generate-license
generate-license:
	GO111MODULE=on $(GO) run build/generate-licenses.go

.PHONY: generate-gitignore
generate-gitignore:
	GO111MODULE=on $(GO) run build/generate-gitignores.go

.PHONY: generate-images
generate-images: | node_modules
	npm install --no-save --no-package-lock fabric@4 imagemin-zopfli@7
	node build/generate-images.js $(TAGS)

.PHONY: generate-manpage
generate-manpage:
	@[ -f gitea ] || make backend
	@mkdir -p man/man1/ man/man5
	@./gitea docs --man > man/man1/gitea.1
	@gzip -9 man/man1/gitea.1 && echo man/man1/gitea.1.gz created
	@#TODO A smal script witch format config-cheat-sheet.en-us.md nicely to suit as config man page

.PHONY: pr\#%
pr\#%: clean-all
	$(GO) run contrib/pr/checkout.go $*

.PHONY: golangci-lint
golangci-lint:
	@hash golangci-lint > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		export BINARY="golangci-lint"; \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin v1.37.0; \
	fi
	golangci-lint run --timeout 10m

.PHONY: docker
docker:
	docker build --disable-content-trust=false -t $(DOCKER_REF) .
# support also build args docker build --build-arg GITEA_VERSION=v1.2.3 --build-arg TAGS="bindata sqlite sqlite_unlock_notify"  .

.PHONY: docker-build
docker-build:
	docker run -ti --rm -v "$(CURDIR):/srv/app/src/code.gitea.io/gitea" -w /srv/app/src/code.gitea.io/gitea -e TAGS="bindata $(TAGS)" LDFLAGS="$(LDFLAGS)" CGO_EXTRA_CFLAGS="$(CGO_EXTRA_CFLAGS)" webhippie/golang:edge make clean build

# This endif closes the if at the top of the file
endif
