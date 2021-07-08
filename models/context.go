// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/builder"
)

// DBContext represents a db context
type DBContext struct {
	e Engine
}

// DefaultDBContext represents a DBContext with default Engine
func DefaultDBContext() DBContext {
	return DBContext{x}
}

// Committer represents an interface to Commit or Close the dbcontext
type Committer interface {
	Commit() error
	Close() error
}

// TxDBContext represents a transaction DBContext
func TxDBContext() (DBContext, Committer, error) {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		sess.Close()
		return DBContext{}, nil, err
	}

	return DBContext{sess}, sess, nil
}

// WithContext represents executing database operations
func WithContext(f func(ctx DBContext) error) error {
	return f(DBContext{x})
}

// WithTx represents executing database operations on a transaction
func WithTx(f func(ctx DBContext) error) error {
	sess := x.NewSession()
	if err := sess.Begin(); err != nil {
		sess.Close()
		return err
	}

	if err := f(DBContext{sess}); err != nil {
		sess.Close()
		return err
	}

	err := sess.Commit()
	sess.Close()
	return err
}

// Iterate iterates the databases and doing something
func Iterate(ctx DBContext, tableBean interface{}, cond builder.Cond, fun func(idx int, bean interface{}) error) error {
	return ctx.e.Where(cond).
		BufferSize(setting.Database.IterateBufferSize).
		Iterate(tableBean, fun)
}
