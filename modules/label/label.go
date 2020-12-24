// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package label

import (
	"sort"
)

// Priority represents label priority
type Priority string

// Label represents label information loaded from template
type Label struct {
	Name        string   `yaml:"name"`
	Color       string   `yaml:"color"`
	Description string   `yaml:"description,omitempty"`
	Priority    Priority `yaml:"priority,omitempty"`
}

var priorityValues map[Priority]int = map[Priority]int{
	"critical": 1000,
	"high": 100,
	"medium": 0,
	"low": -100,
}

var priorities []Priority

// Value returns numeric value for priority
func (p Priority) Value() int {
	v, ok := priorityValues[p]
	if !ok {
		return 0
	}
	return v
}

// IsValid checks if priority is valid
func (p Priority) IsValid() bool {
	if len(p) == 0 {
		return true
	}
	_, ok := priorityValues[p]
	return ok
}

// GetPriorities returns list of priorities
func GetPriorities() []Priority {
	return priorities
}

func init() {
	type kv struct {
		Key   Priority
		Value int
	}
	var ss []kv
	for k, v := range priorityValues {
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})
	priorities = make([]Priority, len(priorityValues))
	for i, kv := range ss {
		priorities[i] = kv.Key
	}
}
