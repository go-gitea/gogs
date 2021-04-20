# Copyright 2019 The Libc Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

.PHONY:	all bench clean cover cpu editor internalError later mem nuke todo edit devbench \
	darwin_amd64 \
	darwin_arm64 \
	linux_386 \
	linux_amd64 \
	linux_arm \
	linux_arm64 \


grep=--include=*.go --include=*.l --include=*.y --include=*.yy --include=*.qbe --include=*.ssa
ngrep='internalError\|TODOOK'
log=log-$(shell go env GOOS)-$(shell go env GOARCH)

all:
	date
	go version 2>&1 | tee $(log)
	go generate
	gofmt -l -s -w *.go
	go install -v ./...
	go test -i
	go test 2>&1 -timeout 1h | tee -a $(log)
	GOOS=darwin GOARCH=amd64 go build
	GOOS=darwin GOARCH=arm64 go build
	GOOS=linux GOARCH=386 go build
	GOOS=linux GOARCH=amd64 go build
	GOOS=linux GOARCH=arm go build
	GOOS=linux GOARCH=arm64 go build
	GOOS=windows GOARCH=386 go build
	GOOS=windows GOARCH=amd64 go build
	go vet -unsafeptr=false 2>&1 | grep -v $(ngrep) || true
	golint 2>&1 | grep -v $(ngrep) || true
	make todo
	misspell *.go
	staticcheck || true
	maligned || true
	grep -n 'FAIL\|PASS' $(log)
	go version
	date 2>&1 | tee -a $(log)

darwin_amd64:
	TARGET_GOOS=darwin TARGET_GOARCH=amd64 go generate
	GOOS=darwin GOARCH=amd64 go build -v ./...

darwin_arm64:
	TARGET_GOOS=darwin TARGET_GOARCH=arm64 go generate
	GOOS=darwin GOARCH=arm64 go build -v ./...

linux_amd64:
	TARGET_GOOS=linux TARGET_GOARCH=amd64 go generate
	GOOS=linux GOARCH=amd64 go build -v ./...

linux_386:
	CCGO_CPP=i686-linux-gnu-cpp TARGET_GOOS=linux TARGET_GOARCH=386 go generate
	GOOS=linux GOARCH=386 go build -v ./...

linux_arm:
	CCGO_CPP=arm-linux-gnueabi-cpp-8 TARGET_GOOS=linux TARGET_GOARCH=arm go generate
	GOOS=linux GOARCH=arm go build -v ./...

linux_arm64:
	CCGO_CPP=aarch64-linux-gnu-cpp-8 TARGET_GOOS=linux TARGET_GOARCH=arm64 go generate
	GOOS=linux GOARCH=arm64 go build -v ./...

windows_amd64:
	CCGO_CPP=x86_64-w64-mingw32-cpp TARGET_GOOS=windows TARGET_GOARCH=amd64 go generate
	GOOS=windows GOARCH=amd64 go build -v ./...

windows_386:
	CCGO_CPP=i686-w64-mingw32-cpp TARGET_GOOS=windows TARGET_GOARCH=386 go generate
	GOOS=windows GOARCH=386 go build -v ./...

all_targets: linux_amd64 linux_386 linux_arm linux_arm64 windows_amd64 windows_386
	echo done

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

devbench:
	date 2>&1 | tee log-devbench
	go test -timeout 24h -dev -run @ -bench . 2>&1 | tee -a log-devbench
	grep -n 'FAIL\|SKIP' log-devbench || true

bench:
	date 2>&1 | tee log-bench
	go test -timeout 24h -v -run '^[^E]' -bench . 2>&1 | tee -a log-bench
	grep -n 'FAIL\|SKIP' log-bench || true

clean:
	go clean
	rm -f *~ *.test *.out

cover:
	t=$(shell mktemp) ; go test -coverprofile $$t && go tool cover -html $$t && unlink $$t

cpu: clean
	go test -run @ -bench . -cpuprofile cpu.out
	go tool pprof -lines *.test cpu.out

edit:
	@touch log
	@if [ -f "Session.vim" ]; then gvim -S & else gvim -p Makefile *.go & fi

editor:
	GO111MODULE=off go generate 2>&1 | tee log
	gofmt -l -s -w *.go
	GO111MODULE=off go test -i
	GO111MODULE=off go test -short 2>&1 | tee -a log
	GO111MODULE=off go install -v ./...

later:
	@grep -n $(grep) LATER * || true
	@grep -n $(grep) MAYBE * || true

mem: clean
	go test -v -run ParserCS -memprofile mem.out -timeout 24h
	go tool pprof -lines -web -alloc_space *.test mem.out

nuke: clean
	go clean -i

todo:
	@grep -nr $(grep) ^[[:space:]]*_[[:space:]]*=[[:space:]][[:alpha:]][[:alnum:]]* * | grep -v $(ngrep) || true
	@grep -nr $(grep) 'TODO\|panic' * | grep -v $(ngrep) || true
	@grep -nr $(grep) BUG * | grep -v $(ngrep) || true
	@grep -nr $(grep) [^[:alpha:]]println * | grep -v $(ngrep) || true
	@grep -nir $(grep) 'work.*progress' || true
