# Copyright 2017 The Sqlite Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

.PHONY:	all clean cover cpu editor internalError later mem nuke todo edit tcl extraquick full

grep=--include=*.go --include=*.l --include=*.y --include=*.yy
ngrep='TODOOK\|internal\/vfs\|internal\/bin\|internal\/mptest\|.*stringer.*\.go'
host=$(shell go env GOOS)-$(shell go env GOARCH)
testlog=testdata/testlog-$(shell echo $$GOOS)-$(shell echo $$GOARCH)$(shell echo $$SQLITE_TEST_SUFFIX)

all: editor
	date
	go version 2>&1 | tee log
	./unconvert.sh
	gofmt -l -s -w *.go
	go test -i
	go test -v 2>&1 -timeout 24h | tee -a log
	go run speedtest1/main_$(shell go env GOOS)_$(shell go env GOARCH).go
	GOOS=linux GOARCH=386 go build -v ./...
	GOOS=linux GOARCH=386 go build -v ./...
	GOOS=linux GOARCH=amd64 go build -v ./...
	GOOS=linux GOARCH=amd64 go build -v ./...
	GOOS=linux GOARCH=arm go build -v ./...
	GOOS=linux GOARCH=arm64 go build -v ./...
	GOOS=windows GOARCH=386 go build -v ./...
	GOOS=windows GOARCH=amd64 go build -v ./...
	golint 2>&1 | grep -v $(ngrep) || true
	misspell *.go
	staticcheck || true
	maligned || true
	git diff --unified=0 testdata *.golden
	grep -n --color=always 'FAIL\|PASS' log
	go version
	date 2>&1 | tee -a log

build_all_targets:
	GOOS=darwin GOARCH=amd64 go build -v ./...
	GOOS=darwin GOARCH=arm64 go build -v ./...
	GOOS=linux GOARCH=386 go build -v ./...
	GOOS=linux GOARCH=amd64 go build -v ./...
	GOOS=linux GOARCH=arm go build -v ./...
	GOOS=linux GOARCH=arm64 go build -v ./...
	GOOS=windows GOARCH=386 go build -v ./...
	GOOS=windows GOARCH=amd64 go build -v ./...
	echo done

darwin_amd64:
	TARGET_GOOS=darwin TARGET_GOARCH=amd64 go generate 2>&1 | tee /tmp/log-generate-sqlite-darwin-amd64
	GOOS=darwin GOARCH=amd64 go build -v ./...

darwin_arm64:
	TARGET_GOOS=darwin TARGET_GOARCH=arm64 go generate 2>&1 | tee /tmp/log-generate-sqlite-darwin-arm64
	GOOS=darwin GOARCH=arm64 go build -v ./...

linux_amd64:
	TARGET_GOOS=linux TARGET_GOARCH=amd64 go generate 2>&1 | tee /tmp/log-generate-sqlite-linux-amd64
	GOOS=linux GOARCH=amd64 go build -v ./...

linux_386:
	CCGO_CPP=i686-linux-gnu-cpp TARGET_GOARCH=386 TARGET_GOOS=linux go generate 2>&1 | tee /tmp/log-generate-sqlite-linux-386
	GOOS=linux GOARCH=386 go build -v ./...

linux_arm:
	CCGO_CPP=arm-linux-gnueabi-cpp-8 TARGET_GOARCH=arm TARGET_GOOS=linux go generate 2>&1 | tee /tmp/log-generate-sqlite-linux-arm
	GOOS=linux GOARCH=arm go build -v ./...

linux_arm64:
	CCGO_CPP=aarch64-linux-gnu-cpp-8 TARGET_GOARCH=arm64 TARGET_GOOS=linux go generate 2>&1 | tee /tmp/log-generate-sqlite-linux-arm64
	GOOS=linux GOARCH=arm64 go build -v ./...

windows_amd64:
	CCGO_CPP=x86_64-w64-mingw32-cpp TARGET_GOOS=windows TARGET_GOARCH=amd64 go generate 2>&1 | tee /tmp/log-generate-sqlite-windows-amd64
	GOOS=windows GOARCH=amd64 go build -v ./...

windows_386:
	CCGO_CPP=i686-w64-mingw32-cpp TARGET_GOOS=windows TARGET_GOARCH=386 go generate 2>&1 | tee /tmp/log-generate-sqlite-windows-386
	GOOS=windows GOARCH=386 go build -v ./...

all_targets: linux_amd64 linux_386 linux_arm linux_arm64 windows_amd64 windows_386
	gofmt -l -s -w .
	echo done

test:
	go version | tee $(testlog)
	uname -a | tee -a $(testlog)
	go test -v -timeout 24h | tee -a $(testlog)
	grep -ni fail $(testlog) | tee -a $(testlog) || true
	LC_ALL=C date | tee -a $(testlog)
	grep -ni --color=always fail $(testlog) || true

test_darwin_amd64:
	GOOS=darwin GOARCH=amd64 make test

test_darwin_arm64:
	GOOS=darwin GOARCH=arm64 make test

test_linux_amd64:
	GOOS=linux GOARCH=amd64 make test

test_linux_386:
	GOOS=linux GOARCH=386 make test

test_linux_386_hosted:
	GOOS=linux GOARCH=386 SQLITE_TEST_SUFFIX=-hosted-$(host) make test

test_linux_arm:
	GOOS=linux GOARCH=arm make test

test_linux_arm64:
	GOOS=linux GOARCH=arm64 make test

extraquick:
	go test -timeout 24h -v -run Tcl -suite extraquick -maxerror 1 2>&1 | tee log-extraquick
	date

full:
	go test -timeout 24h -v -run Tcl -suite full 2>&1 | tee log-full
	date

clean:
	go clean
	rm -f *~ *.test *.out test.db* tt4-test*.db* test_sv.* testdb-*

cover:
	t=$(shell tempfile) ; go test -coverprofile $$t && go tool cover -html $$t && unlink $$t

cpu: clean
	go test -run @ -bench . -cpuprofile cpu.out
	go tool pprof -lines *.test cpu.out

edit:
	@touch log
	@if [ -f "Session.vim" ]; then gvim -S & else gvim -p Makefile *.go & fi

editor:
	gofmt -l -s -w *.go
	GO111MODULE=off go install -v ./...

internalError:
	egrep -ho '"internal error.*"' *.go | sort | cat -n

later:
	@grep -n $(grep) LATER * || true
	@grep -n $(grep) MAYBE * || true

mem: clean
	go test -run @ -bench . -memprofile mem.out -memprofilerate 1 -timeout 24h
	go tool pprof -lines -web -alloc_space *.test mem.out

memgrind:
	go test -v -timeout 24h -tags libc.memgrind,cgobench -bench . -suite extraquick -xtags=libc.memgrind

regression_base_release:
	GO111MODULE=on go test -v -timeout 24h -tags=cgobench -run @ -bench '(Reading1|InsertComparative)/sqlite[^3]' -recs_per_sec_as_mbps 2>&1 | tee log-regression-base

regression_base_master:
	GO111MODULE=off go test -v -timeout 24h -tags=cgobench -run @ -bench '(Reading1|InsertComparative)/sqlite[^3]' -recs_per_sec_as_mbps 2>&1 | tee log-regression-base

regression_check:
	GO111MODULE=off go test -v -timeout 24h -tags=cgobench -run @ -bench '(Reading1|InsertComparative)/sqlite[^3]' -recs_per_sec_as_mbps 2>&1 | tee log-regression
	benchcmp -changed -mag log-regression-base log-regression

nuke: clean
	go clean -i

todo:
	@grep -nr $(grep) ^[[:space:]]*_[[:space:]]*=[[:space:]][[:alpha:]][[:alnum:]]* * | grep -v $(ngrep) || true
	@grep -nr $(grep) TODO * | grep -v $(ngrep) || true
	@grep -nr $(grep) BUG * | grep -v $(ngrep) || true
	@grep -nr $(grep) [^[:alpha:]]println * | grep -v $(ngrep) || true

tcl:
	cp log log-0
	go test -run Tcl$$ 2>&1 -timeout 24h -trc | tee log
	grep -c '\.\.\. \?Ok' log || true
	grep -c '^!' log || true
	# grep -c 'Error:' log || true

tclshort:
	cp log log-0
	go test -run Tcl$$ -short 2>&1 -timeout 24h -trc | tee log
	grep -c '\.\.\. \?Ok' log || true
	grep -c '^!' log || true
	# grep -c 'Error:' log || true
