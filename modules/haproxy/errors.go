// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package haproxy

import "fmt"

// ErrBadHeader is an error demonstrating a bad proxy header
type ErrBadHeader struct {
	Header []byte
}

func (e *ErrBadHeader) Error() string {
	return fmt.Sprintf("Unexpected proxy header: %v", e.Header)
}

// ErrBadAddressType is an error demonstrating a bad proxy header with bad Address type
type ErrBadAddressType struct {
	Address string
}

func (e *ErrBadAddressType) Error() string {
	return fmt.Sprintf("Unexpected proxy header address type: %s", e.Address)
}

// ErrBadRemote is an error demonstrating a bad proxy header with bad Remote
type ErrBadRemote struct {
	IP   string
	Port string
}

func (e *ErrBadRemote) Error() string {
	return fmt.Sprintf("Unexpected proxy header remote IP and port: %s %s", e.IP, e.Port)
}

// ErrBadLocal is an error demonstrating a bad proxy header with bad Local
type ErrBadLocal struct {
	IP   string
	Port string
}

func (e *ErrBadLocal) Error() string {
	return fmt.Sprintf("Unexpected proxy header local IP and port: %s %s", e.IP, e.Port)
}
