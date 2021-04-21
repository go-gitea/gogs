// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import "time"

// Package represents a package
type Package struct {
	Name    string      `json:"name"`
	Type    string      `json:"package_type"`
	Owner   *User       `json:"owner"`
	Repo    *Repository `json:"repository"`
	Private bool        `json:"private"`

	// swagger:strfmt date-time
	Created *time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated *time.Time `json:"updated_at"`
}

// PackageVersion represents a package version
type PackageVersion struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
}
