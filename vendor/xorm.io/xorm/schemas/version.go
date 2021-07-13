// Copyright 2021 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package schemas

// Version represents a database version
type Version struct {
	Number  string // the version number which could be compared
	Level   string
	Edition string
}
