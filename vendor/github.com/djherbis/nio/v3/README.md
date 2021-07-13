nio 
==========

[![GoDoc](https://godoc.org/github.com/djherbis/nio?status.svg)](https://godoc.org/github.com/djherbis/nio) 
[![Release](https://img.shields.io/github/release/djherbis/nio.svg)](https://github.com/djherbis/nio/releases/latest)
[![Software License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE.txt)
[![Build Status](https://travis-ci.org/djherbis/nio.svg)](https://travis-ci.org/djherbis/nio)
[![Coverage Status](https://coveralls.io/repos/djherbis/nio/badge.svg?branch=master)](https://coveralls.io/r/djherbis/nio?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/djherbis/nio)](https://goreportcard.com/report/github.com/djherbis/nio)

Usage
-----

The Buffer interface:

```go
type Buffer interface {
	Len() int64
	Cap() int64
	io.ReadWriter
}

```

nio's Copy method concurrently copies from an io.Reader to a supplied nio.Buffer, 
then from the nio.Buffer to an io.Writer. This way, blocking writes don't slow the io.Reader.

```go
import (
  "github.com/djherbis/buffer"
  "github.com/djherbis/nio"
)

buf := buffer.New(32*1024) // 32KB In memory Buffer
nio.Copy(w, r, buf) // Reads and Writes concurrently, buffering using buf.
```

nio's Pipe method is a buffered version of io.Pipe
The writer return once its data has been written to the Buffer.
The reader returns with data off the Buffer.

```go
import (
  "gopkg.in/djherbis/buffer.v1"
  "gopkg.in/djherbis/nio.v2"
)

buf := buffer.New(32*1024) // 32KB In memory Buffer
r, w := nio.Pipe(buf)
```

Installation
------------
```sh
go get gopkg.in/djherbis/nio.v2
```

For some pre-built buffers grab:
```sh
go get gopkg.in/djherbis/buffer.v1
```

Mentions
------------
[GopherCon 2017: Peter Bourgon - Evolutionary Optimization with Go](https://www.youtube.com/watch?v=ha8gdZ27wMo&start=2077&end=2140)
