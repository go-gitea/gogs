// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package base

import "time"

// Project is a standard project information
type Project struct {
	Number      int
	Name        string
	Description string `xorm:"TEXT"`
	State       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Columns     []*ProjectColumn
}

type ProjectColumn struct {
	Cards []*ProjectCard
}

type ProjectCard struct {
}
