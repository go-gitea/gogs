// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Label a label to an issue or a pr
// swagger:model
type Label struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// example: 00aabb
	Color       string `json:"color"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// CreateLabelOption options for creating a label
type CreateLabelOption struct {
	// required:true
	Name string `json:"name" binding:"Required"`
	// required:true
	// example: #00aabb
	Color       string `json:"color" binding:"Required"`
	Description string `json:"description"`
}

// EditLabelOption options for editing a label
type EditLabelOption struct {
	Name        *string `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
}

// IssueLabelsOption a collection of labels
type IssueLabelsOption struct {
	// list of label IDs or names
	Labels []string `json:"labels"`
	Mode   string   `json:"mode"` // id_only or name_only
}
