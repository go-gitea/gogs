// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import "strconv"

// EntryMode the type of the object in the git tree
type EntryMode int

// There are only a few file modes in Git. They look like unix file modes, but they can only be
// one of these.
const (
	// EntryModeBlob
	EntryModeBlob EntryMode = 0o100644
	// EntryModeExec
	EntryModeExec EntryMode = 0o100755
	// EntryModeSymlink
	EntryModeSymlink EntryMode = 0o120000
	// EntryModeCommit
	EntryModeCommit EntryMode = 0o160000
	// EntryModeTree
	EntryModeTree EntryMode = 0o040000
)

// String converts an EntryMode to a string
func (e EntryMode) String() string {
	return strconv.FormatInt(int64(e), 8)
}

// ToEntryMode converts a string to an EntryMode
func ToEntryMode(value string) EntryMode {
	v, _ := strconv.ParseInt(value, 8, 32)
	return EntryMode(v)
}
