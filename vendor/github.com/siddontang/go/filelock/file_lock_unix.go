// Copyright 2014 The LevelDB-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux netbsd openbsd

package filelock

import (
	"io"
	"os"
	"syscall"
)

// lockCloser hides all of an os.File's methods, except for Close.
type lockCloser struct {
	f *os.File
}

func (l lockCloser) Close() error {
	return l.f.Close()
}

func Lock(name string) (io.Closer, error) {
	f, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	/*
		Some people tell me FcntlFlock does not exist, so use flock here
	*/
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}

	// spec := syscall.Flock_t{
	// 	Type:   syscall.F_WRLCK,
	// 	Whence: int16(os.SEEK_SET),
	// 	Start:  0,
	// 	Len:    0, // 0 means to lock the entire file.
	// 	Pid:    int32(os.Getpid()),
	// }
	// if err := syscall.FcntlFlock(f.Fd(), syscall.F_SETLK, &spec); err != nil {
	// 	f.Close()
	// 	return nil, err
	// }

	return lockCloser{f}, nil
}
