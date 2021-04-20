// Copyright 2018 The Memory Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build amd64 amd64p32 arm64 arm64be mips64 mips64le mips64p32 mips64p32le ppc64 sparc64

package memory // import "modernc.org/memory"

type rawmem [1<<50 - 1]byte
