// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package session

// Store represents a session store
type Store interface {
	Get(interface{}) interface{}
	Set(interface{}, interface{}) error
	Delete(interface{}) error
}

type emptyStore struct{}

// NewEmptyStore returns an emptyStore
func NewEmptyStore() *emptyStore {
	return &emptyStore{}
}

func (emptyStore) Get(interface{}) interface{} {
	return nil
}

func (emptyStore) Set(interface{}, interface{}) error {
	return nil
}

func (emptyStore) Delete(interface{}) error {
	return nil
}
