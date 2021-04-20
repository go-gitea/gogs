// Copyright 2011 Evan Shaw. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE-MMAP-GO file.

// +build darwin dragonfly freebsd linux openbsd solaris netbsd

// Modifications (c) 2017 The Memory Authors.

package memory // import "modernc.org/memory"

import (
	"os"
	"syscall"
	"unsafe"
)

const pageSizeLog = 20

var (
	osPageMask = osPageSize - 1
	osPageSize = os.Getpagesize()
)

func unmap(addr uintptr, size int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_MUNMAP, addr, uintptr(size), 0)
	if errno != 0 {
		return errno
	}

	return nil
}

// pageSize aligned.
func mmap(size int) (uintptr, int, error) {
	size = roundup(size, osPageSize)
	b, err := syscall.Mmap(-1, 0, size+pageSize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED|syscall.MAP_ANON)
	if err != nil {
		return 0, 0, err
	}

	n := len(b)
	p := uintptr(unsafe.Pointer(&b[0]))
	if p&uintptr(osPageMask) != 0 {
		panic("internal error")
	}

	mod := int(p) & pageMask
	if mod != 0 {
		m := pageSize - mod
		if err := unmap(p, m); err != nil {
			return 0, 0, err
		}

		n -= m
		p += uintptr(m)
	}

	if p&uintptr(pageMask) != 0 {
		panic("internal error")
	}

	if n-size != 0 {
		if err := unmap(p+uintptr(size), n-size); err != nil {
			return 0, 0, err
		}
	}

	return p, size, nil
}
