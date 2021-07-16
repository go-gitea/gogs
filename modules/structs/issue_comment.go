// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// Comment represents a comment on a commit or issue
type Comment struct {
	ID               int64         `json:"id"`
	HTMLURL          string        `json:"html_url"`
	PRURL            string        `json:"pull_request_url"`
	IssueURL         string        `json:"issue_url"`
	Poster           *User         `json:"user"`
	OriginalAuthor   string        `json:"original_author"`
	OriginalAuthorID int64         `json:"original_author_id"`
	Body             string        `json:"body"`
	Attachments      []*Attachment `json:"assets"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
}

// CreateIssueCommentOption options for creating a comment on an issue
type CreateIssueCommentOption struct {
	// required:true
	Body string `json:"body" binding:"Required"`
}

// EditIssueCommentOption options for editing a comment
type EditIssueCommentOption struct {
	// required: true
	Body string `json:"body" binding:"Required"`
}
