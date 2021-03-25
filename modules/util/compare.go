// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"sort"
	"strings"
)

// Int64Slice attaches the methods of Interface to []int64, sorting in increasing order.
type Int64Slice []int64

func (p Int64Slice) Len() int           { return len(p) }
func (p Int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p Int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// IsSliceInt64Eq returns if the two slice has the same elements but different sequences.
func IsSliceInt64Eq(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Sort(Int64Slice(a))
	sort.Sort(Int64Slice(b))
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ExistsInSlice returns true if string exists in slice.
func ExistsInSlice(target string, slice []string) bool {
	i := sort.Search(len(slice),
		func(i int) bool { return slice[i] == target })
	return i < len(slice)
}

// IsStringInSlice sequential searches if string exists in slice.
func IsStringInSlice(target string, slice []string, insensitive ...bool) bool {
	caseInsensitive := false
	if len(insensitive) != 0 && insensitive[0] {
		caseInsensitive = true
		target = strings.ToLower(target)
	}

	for i := 0; i < len(slice); i++ {
		if caseInsensitive {
			if strings.ToLower(slice[i]) == target {
				return true
			}
		} else {
			if slice[i] == target {
				return true
			}
		}
	}
	return false
}

// IsInt64InSlice sequential searches if int64 exists in slice.
func IsInt64InSlice(target int64, slice []int64) bool {
	for i := 0; i < len(slice); i++ {
		if slice[i] == target {
			return true
		}
	}
	return false
}

// IsEqualSlice returns true if slices are equal.
func IsEqualSlice(target, source []string) bool {
	if len(target) != len(source) {
		return false
	}

	if (target == nil) != (source == nil) {
		return false
	}

	sort.Strings(target)
	sort.Strings(source)

	for i, v := range target {
		if v != source[i] {
			return false
		}
	}

	return true
}
