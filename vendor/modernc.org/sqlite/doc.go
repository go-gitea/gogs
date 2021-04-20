// Copyright 2017 The Sqlite Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sqlite is a CGo-free port of SQLite.
//
// SQLite is an in-process implementation of a self-contained, serverless,
// zero-configuration, transactional SQL database engine.
//
// Builders
//
// Builder results available at
//
//	https://modern-c.appspot.com/-/builder/?importpath=modernc.org%2fsqlite
//
// Changelog
//
// 2021-03-14 v1.10.0:
//
// Update to use sqlite 3.35.0, release notes at https://www.sqlite.org/releaselog/3_35_0.html.
//
// 2021-03-11 v1.9.0:
//
// Support darwin/arm64.
//
// 2021-01-08 v1.8.0:
//
// Support darwin/amd64.
//
// 2020-09-13 v1.7.0:
//
// Support linux/arm and linux/arm64.
//
// 2020-09-08 v1.6.0:
//
// Support linux/386.
//
// 2020-09-03 v1.5.0:
//
// This project is now completely CGo-free, including the Tcl tests.
//
// 2020-08-26 v1.4.0:
//
// First stable release for linux/amd64.  The database/sql driver and its tests
// are CGo free.  Tests of the translated sqlite3.c library still require CGo.
//
//	$ make full
//
//	...
//
//	SQLite 2020-08-14 13:23:32 fca8dc8b578f215a969cd899336378966156154710873e68b3d9ac5881b0ff3f
//	0 errors out of 928271 tests on 3900x Linux 64-bit little-endian
//	WARNING: Multi-threaded tests skipped: Linked against a non-threadsafe Tcl build
//	All memory allocations freed - no leaks
//	Maximum memory usage: 9156360 bytes
//	Current memory usage: 0 bytes
//	Number of malloc()  : -1 calls
//	--- PASS: TestTclTest (1785.04s)
//	PASS
//	ok  	modernc.org/sqlite	1785.041s
//	$
//
// 2020-07-26 v1.4.0-beta1:
//
// The project has reached beta status while supporting linux/amd64 only at the
// moment. The 'extraquick' Tcl testsuite reports
//
//	630 errors out of 200177 tests on  Linux 64-bit little-endian
//
// and some memory leaks
//
//	Unfreed memory: 698816 bytes in 322 allocations
//
// 2019-12-28 v1.2.0-alpha.3: Third alpha fixes issue #19.
//
// It also bumps the minor version as the repository was wrongly already tagged
// with v1.1.0 before.  Even though the tag was deleted there are proxies that
// cached that tag. Thanks /u/garaktailor for detecting the problem and
// suggesting this solution.
//
// 2019-12-26 v1.1.0-alpha.2: Second alpha release adds support for accessing a
// database concurrently by multiple goroutines and/or processes. v1.1.0 is now
// considered feature-complete. Next planed release should be a beta with a
// proper test suite.
//
// 2019-12-18 v1.1.0-alpha.1: First alpha release using the new cc/v3, gocc,
// qbe toolchain. Some primitive tests pass on linux_{amd64,386}. Not yet safe
// for concurrent access by multiple goroutines. Next alpha release is planed
// to arrive before the end of this year.
//
// 2017-06-10 Windows/Intel no more uses the VM (thanks Steffen Butzer).
//
// 2017-06-05 Linux/Intel no more uses the VM (cznic/virtual).
//
// Connecting to a database
//
// To access a Sqlite database do something like
//
//	import (
//		"database/sql"
//
//		_ "modernc.org/sqlite"
//	)
//
//	...
//
//
//	db, err := sql.Open("sqlite", dsnURI)
//
//	...
//
// Supported platforms and architectures
//
// These combinations of GOOS and GOARCH are currently supported
//
// 	darwin	amd64
// 	darwin	arm64
// 	linux	386
// 	linux	amd64
// 	linux	arm
// 	linux	arm64
//
// Planned platforms and architectures
//
// 	windows	386
// 	windows	amd64
//
// Debug and development versions
//
// A comma separated list of options can be passed to `go generate` via the
// environment variable GO_GENERATE. Some useful options include for example:
//
//	-DSQLITE_DEBUG
//	-DSQLITE_MEM_DEBUG
//	-ccgo-verify-structs
//
// To create a debug/development version, issue for example:
//
//	$ GO_GENERATE=-DSQLITE_DEBUG,-DSQLITE_MEM_DEBUG go generate
//
// Note: To run `go generate` you need to have modernc.org/ccgo/v3 installed.
//
// Sqlite documentation
//
// See https://sqlite.org/docs.html
package sqlite // import "modernc.org/sqlite"
