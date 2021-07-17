// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "github.com/lafriks/xormstore"

// CreateStore creates a xormstore for the provided table and key
func CreateStore(table, key string) (*xormstore.Store, error) {
	store, err := xormstore.NewOptions(x, xormstore.Options{
		TableName: table,
	}, []byte(key))

	return store, err
}
