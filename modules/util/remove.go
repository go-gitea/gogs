// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"os"
	"syscall"
	"time"
)

// Remove removes the named file or (empty) directory with at most 5 attempts.
func Remove(name string) error {
	var err error
	for i := 0; i < 5; i++ {
		err = os.Remove(name)
		if err == nil {
			break
		}
		unwrapped := err.(*os.PathError).Err
		if unwrapped == syscall.EBUSY || unwrapped == syscall.ENOTEMPTY || unwrapped == syscall.EPERM || unwrapped == syscall.EMFILE || unwrapped == syscall.ENFILE {
			// try again
			<-time.After(100 * time.Millisecond)
			continue
		}

		if unwrapped == syscall.ENOENT {
			// it's already gone
			return nil
		}
	}
	return err
}

// RemoveAll removes the named file or (empty) directory with at most 5 attempts.
func RemoveAll(name string) error {
	var err error
	for i := 0; i < 5; i++ {
		err = os.RemoveAll(name)
		if err == nil {
			break
		}
		unwrapped := err.(*os.PathError).Err
		if unwrapped == syscall.EBUSY || unwrapped == syscall.ENOTEMPTY || unwrapped == syscall.EPERM || unwrapped == syscall.EMFILE || unwrapped == syscall.ENFILE {
			// try again
			<-time.After(100 * time.Millisecond)
			continue
		}

		if unwrapped == syscall.ENOENT {
			// it's already gone
			return nil
		}
	}
	return err
}

// Rename renames (moves) oldpath to newpath with at most 5 attempts.
func Rename(oldpath, newpath string) error {
	var err error
	for i := 0; i < 5; i++ {
		err = os.Rename(oldpath, newpath)
		if err == nil {
			break
		}
		unwrapped := err.(*os.LinkError).Err
		if unwrapped == syscall.EBUSY || unwrapped == syscall.ENOTEMPTY || unwrapped == syscall.EPERM || unwrapped == syscall.EMFILE || unwrapped == syscall.ENFILE {
			// try again
			<-time.After(100 * time.Millisecond)
			continue
		}

		if i == 0 && os.IsNotExist(err) {
			return err
		}

		if unwrapped == syscall.ENOENT {
			// it's already gone
			return nil
		}
	}
	return err
}
