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

// Get implements Store
func (EmptyStore) Get(interface{}) interface{} {
	return nil
}

// Set implements Store
func (EmptyStore) Set(interface{}, interface{}) error {
	return nil
}

// Delete implements Store
func (EmptyStore) Delete(interface{}) error {
	return nil
}
