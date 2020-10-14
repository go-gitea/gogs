// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"strings"
	"time"
)

// StateType issue state type
type StateType string

const (
	// StateOpen pr is opend
	StateOpen StateType = "open"
	// StateClosed pr is closed
	StateClosed StateType = "closed"
	// StateAll is all
	StateAll StateType = "all"
)

// PullRequestMeta PR info if an issue is a PR
type PullRequestMeta struct {
	HasMerged bool       `json:"merged"`
	Merged    *time.Time `json:"merged_at"`
}

// RepositoryMeta basic repository information
type RepositoryMeta struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	FullName string `json:"full_name"`
}

// Issue represents an issue in a repository
// swagger:model
type Issue struct {
	ID               int64           `json:"id"`
	URL              string          `json:"url"`
	HTMLURL          string          `json:"html_url"`
	Index            int64           `json:"number"`
	Poster           *User           `json:"user"`
	OriginalAuthor   string          `json:"original_author"`
	OriginalAuthorID int64           `json:"original_author_id"`
	Title            string          `json:"title"`
	Body             string          `json:"body"`
	Labels           []*Label        `json:"labels"`
	Milestone        *Milestone      `json:"milestone"`
	Assignee         *User           `json:"assignee"`
	Assignees        []*User         `json:"assignees"`
	Reactions        ReactionSummary `json:"reaction_summary"`
	// Whether the issue is open or closed
	//
	// type: string
	// enum: open,closed
	State    StateType `json:"state"`
	IsLocked bool      `json:"is_locked"`
	Comments int       `json:"comments"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated time.Time `json:"updated_at"`
	// swagger:strfmt date-time
	Closed *time.Time `json:"closed_at"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`

	PullRequest *PullRequestMeta `json:"pull_request"`
	Repo        *RepositoryMeta  `json:"repository"`
}

// ListIssueOption list issue options
type ListIssueOption struct {
	Page  int
	State string
}

// CreateIssueOption options to create one issue
type CreateIssueOption struct {
	// required:true
	Title string `json:"title" binding:"Required"`
	Body  string `json:"body"`
	// username of assignee
	Assignee  string   `json:"assignee"`
	Assignees []string `json:"assignees"`
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
	// milestone id
	Milestone int64 `json:"milestone"`
	// list of label ids
	Labels []int64 `json:"labels"`
	Closed bool    `json:"closed"`
}

// EditIssueOption options for editing an issue
type EditIssueOption struct {
	Title     string   `json:"title"`
	Body      *string  `json:"body"`
	Assignee  *string  `json:"assignee"`
	Assignees []string `json:"assignees"`
	Milestone *int64   `json:"milestone"`
	State     *string  `json:"state"`
	// swagger:strfmt date-time
	Deadline       *time.Time `json:"due_date"`
	RemoveDeadline *bool      `json:"unset_due_date"`
}

// EditDeadlineOption options for creating a deadline
type EditDeadlineOption struct {
	// required:true
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
}

// IssueDeadline represents an issue deadline
// swagger:model
type IssueDeadline struct {
	// swagger:strfmt date-time
	Deadline *time.Time `json:"due_date"`
}

// IssueTemplate represents an issue template for a repository
// swagger:model
type IssueTemplate struct {
	Name     string   `json:"name" yaml:"name"`
	Title    string   `json:"title" yaml:"title"`
	About    string   `json:"about" yaml:"about"`
	Labels   []string `json:"labels" yaml:"labels"`
	Content  string   `json:"content" yaml:"-"`
	FileName string   `json:"file_name" yaml:"-"`
}

// Valid checks whether an IssueTemplate is considered valid, e.g. at least name and about
func (it IssueTemplate) Valid() bool {
	return strings.TrimSpace(it.Name) != "" && strings.TrimSpace(it.About) != ""
}
