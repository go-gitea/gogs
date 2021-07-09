// Copyright 2012 The LevelDB-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd,!solaris,!windows

package filelock

import (
	"fmt"
	"io"
	"runtime"
)

func Lock(name string) (io.Closer, error) {
	return nil, fmt.Errorf("leveldb/db: file locking is not implemented on %s/%s", runtime.GOOS, runtime.GOARCH)
}
