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

// EmptyStore represents an empty store
type EmptyStore struct{}

// NewEmptyStore returns an EmptyStore
func NewEmptyStore() *EmptyStore {
	return &EmptyStore{}
}

func (EmptyStore) Get(interface{}) interface{} {
	return nil
}

func (EmptyStore) Set(interface{}, interface{}) error {
	return nil
}

func (EmptyStore) Delete(interface{}) error {
	return nil
}
