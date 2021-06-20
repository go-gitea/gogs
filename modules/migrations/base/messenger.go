// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

// Messenger is a formatting function similar to i18n.Tr
type Messenger func(key string, args ...interface{})

// NilMessenger represents an empty formatting function
func NilMessenger(string, ...interface{}) {}
