// Copyright 2020 The Libc Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build amd64 arm64

package libc // import "modernc.org/libc"

type (
	// RawMem represents the biggest byte array the runtime can handle
	RawMem [1<<50 - 1]byte

	// 48-5*8 = 8 bytes left to pad
	stackHeaderPadding struct {
		a uintptr
	}
)

type bits []int

func newBits(n int) (r bits)  { return make(bits, (n+63)>>6) }
func (b bits) has(n int) bool { return b != nil && b[n>>6]&(1<<uint(n&63)) != 0 }
func (b bits) set(n int)      { b[n>>6] |= 1 << uint(n&63) }
