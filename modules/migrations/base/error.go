// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

// ErrNotSupported represents status if a downloader do not supported something.
type ErrNotSupported struct {
}

// IsErrNotSupported checks if an error is an ErrNotSupported
func IsErrNotSupported(err error) bool {
	_, ok := err.(ErrNotSupported)
	return ok
}

// Error return error message
func (err ErrNotSupported) Error() string {
	return "not supported"
}
