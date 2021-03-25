// +build pam

// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package pam

import (
	"errors"

	"github.com/msteinert/pam"
)

// Supported is true when built with PAM
var Supported = true

// Auth pam auth service
func Auth(serviceName, userName, passwd string) (string, error) {
	t, err := pam.StartFunc(serviceName, userName, func(s pam.Style, msg string) (string, error) {
		switch s {
		case pam.PromptEchoOff:
			return passwd, nil
		case pam.PromptEchoOn, pam.ErrorMsg, pam.TextInfo:
			return "", nil
		}
		return "", errors.New("Unrecognized PAM message style")
	})
	if err != nil {
		return "", err
	}

	if err = t.Authenticate(0); err != nil {
		return "", err
	}

	// PAM login names might suffer transformations in the PAM stack.
	// We should take whatever the PAM stack returns for it.
	return t.GetItem(pam.User)
}
