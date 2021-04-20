// Copyright 2018 The Memory Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build 386 arm armbe mips mipsle ppc ppc64le s390 s390x sparc

package memory // import "modernc.org/memory"

type rawmem [1<<31 - 1]byte
