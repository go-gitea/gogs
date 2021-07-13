// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build !gogit

package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// ParseTreeEntries parses the output of a `git ls-tree -l` command.
func ParseTreeEntries(data []byte) ([]*TreeEntry, error) {
	return parseTreeEntries(data, nil)
}

func parseTreeEntries(data []byte, ptree *Tree) ([]*TreeEntry, error) {
	entries := make([]*TreeEntry, 0, 10)
	for pos := 0; pos < len(data); {
		// expect line to be of the form "<mode> <type> <sha> <space-padded-size>\t<filename>"
		entry := new(TreeEntry)
		entry.ptree = ptree
		if pos+6 > len(data) {
			return nil, fmt.Errorf("Invalid ls-tree output: %s", string(data))
		}
		switch string(data[pos : pos+6]) {
		case "100644":
			entry.entryMode = EntryModeBlob
			pos += 12 // skip over "100644 blob "
		case "100755":
			entry.entryMode = EntryModeExec
			pos += 12 // skip over "100755 blob "
		case "120000":
			entry.entryMode = EntryModeSymlink
			pos += 12 // skip over "120000 blob "
		case "160000":
			entry.entryMode = EntryModeCommit
			pos += 14 // skip over "160000 object "
		case "040000":
			entry.entryMode = EntryModeTree
			pos += 12 // skip over "040000 tree "
		default:
			return nil, fmt.Errorf("unknown type: %v", string(data[pos:pos+6]))
		}

		if pos+40 > len(data) {
			return nil, fmt.Errorf("Invalid ls-tree output: %s", string(data))
		}
		id, err := NewIDFromString(string(data[pos : pos+40]))
		if err != nil {
			return nil, fmt.Errorf("Invalid ls-tree output: %v", err)
		}
		entry.ID = id
		pos += 41 // skip over sha and trailing space

		end := pos + bytes.IndexByte(data[pos:], '\t')
		if end < pos {
			return nil, fmt.Errorf("Invalid ls-tree -l output: %s", string(data))
		}
		entry.size, _ = strconv.ParseInt(strings.TrimSpace(string(data[pos:end])), 10, 64)
		entry.sized = true

		pos = end + 1

		end = pos + bytes.IndexByte(data[pos:], '\n')
		if end < pos {
			return nil, fmt.Errorf("Invalid ls-tree output: %s", string(data))
		}

		// In case entry name is surrounded by double quotes(it happens only in git-shell).
		if data[pos] == '"' {
			entry.name, err = strconv.Unquote(string(data[pos:end]))
			if err != nil {
				return nil, fmt.Errorf("Invalid ls-tree output: %v", err)
			}
		} else {
			entry.name = string(data[pos:end])
		}

		pos = end + 1
		entries = append(entries, entry)
	}
	return entries, nil
}

func catBatchParseTreeEntries(ptree *Tree, rd *bufio.Reader, sz int64) ([]*TreeEntry, error) {
	fnameBuf := make([]byte, 4096)
	modeBuf := make([]byte, 40)
	shaBuf := make([]byte, 40)
	entries := make([]*TreeEntry, 0, 10)

loop:
	for sz > 0 {
		mode, fname, sha, count, err := ParseTreeLine(rd, modeBuf, fnameBuf, shaBuf)
		if err != nil {
			if err == io.EOF {
				break loop
			}
			return nil, err
		}
		sz -= int64(count)
		entry := new(TreeEntry)
		entry.ptree = ptree

		switch string(mode) {
		case "100644":
			entry.entryMode = EntryModeBlob
		case "100755":
			entry.entryMode = EntryModeExec
		case "120000":
			entry.entryMode = EntryModeSymlink
		case "160000":
			entry.entryMode = EntryModeCommit
		case "40000":
			entry.entryMode = EntryModeTree
		default:
			log.Debug("Unknown mode: %v", string(mode))
			return nil, fmt.Errorf("unknown mode: %v", string(mode))
		}

		entry.ID = MustID(sha)
		entry.name = string(fname)
		entries = append(entries, entry)
	}
	if _, err := rd.Discard(1); err != nil {
		return entries, err
	}

	return entries, nil
}
