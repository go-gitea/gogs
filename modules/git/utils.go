// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

// ObjectCache provides thread-safe cache operations.
type ObjectCache struct {
	lock  sync.RWMutex
	cache map[string]interface{}
}

func newObjectCache() *ObjectCache {
	return &ObjectCache{
		cache: make(map[string]interface{}, 10),
	}
}

// Set add obj to cache
func (oc *ObjectCache) Set(id string, obj interface{}) {
	oc.lock.Lock()
	defer oc.lock.Unlock()

	oc.cache[id] = obj
}

// Get get cached obj by id
func (oc *ObjectCache) Get(id string) (interface{}, bool) {
	oc.lock.RLock()
	defer oc.lock.RUnlock()

	obj, has := oc.cache[id]
	return obj, has
}

// isDir returns true if given path is a directory,
// or returns false when it's a file or does not exist.
func isDir(dir string) bool {
	f, e := os.Stat(dir)
	if e != nil {
		return false
	}
	return f.IsDir()
}

// isFile returns true if given path is a file,
// or returns false when it's a directory or does not exist.
func isFile(filePath string) bool {
	f, e := os.Stat(filePath)
	if e != nil {
		return false
	}
	return !f.IsDir()
}

// isExist checks whether a file or directory exists.
// It returns false when the file or directory does not exist.
func isExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

// ConcatenateError concatenats an error with stderr string
func ConcatenateError(err error, stderr string) error {
	if len(stderr) == 0 {
		return err
	}
	return fmt.Errorf("%w - %s", err, stderr)
}

// RefEndName return the end name of a ref name
func RefEndName(refStr string) string {
	if strings.HasPrefix(refStr, BranchPrefix) {
		return refStr[len(BranchPrefix):]
	}

	if strings.HasPrefix(refStr, TagPrefix) {
		return refStr[len(TagPrefix):]
	}

	return refStr
}

// RefURL returns the absolute URL for a ref in a repository
func RefURL(repoURL, ref string) string {
	refName := RefEndName(ref)
	switch {
	case strings.HasPrefix(ref, BranchPrefix):
		return repoURL + "/src/branch/" + refName
	case strings.HasPrefix(ref, TagPrefix):
		return repoURL + "/src/tag/" + refName
	default:
		return repoURL + "/src/commit/" + refName
	}
}

// SplitRefName splits a full refname to reftype and simple refname
func SplitRefName(refStr string) (string, string) {
	if strings.HasPrefix(refStr, BranchPrefix) {
		return BranchPrefix, refStr[len(BranchPrefix):]
	}

	if strings.HasPrefix(refStr, TagPrefix) {
		return TagPrefix, refStr[len(TagPrefix):]
	}

	return "", refStr
}

// ParseBool returns the boolean value represented by the string as per git's git_config_bool
// true will be returned for the result if the string is empty, but valid will be false.
// "true", "yes", "on" are all true, true
// "false", "no", "off" are all false, true
// 0 is false, true
// Any other integer is true, true
// Anything else will return false, false
func ParseBool(value string) (result bool, valid bool) {
	// Empty strings are true but invalid
	if len(value) == 0 {
		return true, false
	}
	// These are the git expected true and false values
	if strings.EqualFold(value, "true") || strings.EqualFold(value, "yes") || strings.EqualFold(value, "on") {
		return true, true
	}
	if strings.EqualFold(value, "false") || strings.EqualFold(value, "no") || strings.EqualFold(value, "off") {
		return false, true
	}
	// Try a number
	intValue, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return false, false
	}
	return intValue != 0, true
}

// LimitedReaderCloser is a limited reader closer
type LimitedReaderCloser struct {
	R io.Reader
	C io.Closer
	N int64
}

// Read implements io.Reader
func (l *LimitedReaderCloser) Read(p []byte) (n int, err error) {
	if l.N <= 0 {
		_ = l.C.Close()
		return 0, io.EOF
	}
	if int64(len(p)) > l.N {
		p = p[0:l.N]
	}
	n, err = l.R.Read(p)
	l.N -= int64(n)
	return
}

// Close implements io.Closer
func (l *LimitedReaderCloser) Close() error {
	return l.C.Close()
}
