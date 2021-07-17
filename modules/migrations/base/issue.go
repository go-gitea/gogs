// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2018 Jonas Franz. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import "time"

// Issue is a standard issue information
type Issue struct {
	Number      int64
	PosterID    int64  `yaml:"poster_id"`
	PosterName  string `yaml:"poster_name"`
	PosterEmail string `yaml:"poster_email"`
	Title       string
	Content     string
	Ref         string
	Milestone   string
	State       string // closed, open
	IsLocked    bool   `yaml:"is_locked"`
	Created     time.Time
	Updated     time.Time
	Closed      *time.Time
	Labels      []*Label
	Reactions   []*Reaction
	Assignees   []string
	Context     interface{} `yaml:"-"`
}
