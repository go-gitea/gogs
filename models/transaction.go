// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"github.com/go-xorm/xorm"
)

// AutoTransaction Execute sql wrapped in a transaction(abbr as tx), tx will automatic commit if no errors occurred
func AutoTransaction(f func(*xorm.Session) (interface{}, error), engine *xorm.Engine) (interface{}, error) {
	session := engine.NewSession()
	defer session.Close()

	if err := session.Begin(); err != nil {
		return nil, err
	}

	result, err := f(session)
	if err != nil {
		return nil, err
	}

	if err := session.Commit(); err != nil {
		return nil, err
	}

	return result, nil
}
