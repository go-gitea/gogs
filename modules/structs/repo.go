// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"strings"
	"time"
)

// Permission represents a set of permissions
type Permission struct {
	Admin bool `json:"admin"`
	Push  bool `json:"push"`
	Pull  bool `json:"pull"`
}

// InternalTracker represents settings for internal tracker
// swagger:model
type InternalTracker struct {
	// Enable time tracking (Built-in issue tracker)
	EnableTimeTracker bool `json:"enable_time_tracker"`
	// Let only contributors track time (Built-in issue tracker)
	AllowOnlyContributorsToTrackTime bool `json:"allow_only_contributors_to_track_time"`
	// Enable dependencies for issues and pull requests (Built-in issue tracker)
	EnableIssueDependencies bool `json:"enable_issue_dependencies"`
}

// ExternalTracker represents settings for external tracker
// swagger:model
type ExternalTracker struct {
	// URL of external issue tracker.
	ExternalTrackerURL string `json:"external_tracker_url"`
	// External Issue Tracker URL Format. Use the placeholders {user}, {repo} and {index} for the username, repository name and issue index.
	ExternalTrackerFormat string `json:"external_tracker_format"`
	// External Issue Tracker Number Format, either `numeric` or `alphanumeric`
	ExternalTrackerStyle string `json:"external_tracker_style"`
}

// ExternalWiki represents setting for external wiki
// swagger:model
type ExternalWiki struct {
	// URL of external wiki.
	ExternalWikiURL string `json:"external_wiki_url"`
}

// Repository represents a repository
type Repository struct {
	ID            int64       `json:"id"`
	Owner         *User       `json:"owner"`
	Name          string      `json:"name"`
	FullName      string      `json:"full_name"`
	Description   string      `json:"description"`
	Empty         bool        `json:"empty"`
	Private       bool        `json:"private"`
	Fork          bool        `json:"fork"`
	Template      bool        `json:"template"`
	Parent        *Repository `json:"parent"`
	Mirror        bool        `json:"mirror"`
	Size          int         `json:"size"`
	HTMLURL       string      `json:"html_url"`
	SSHURL        string      `json:"ssh_url"`
	CloneURL      string      `json:"clone_url"`
	OriginalURL   string      `json:"original_url"`
	Website       string      `json:"website"`
	Stars         int         `json:"stars_count"`
	Forks         int         `json:"forks_count"`
	Watchers      int         `json:"watchers_count"`
	OpenIssues    int         `json:"open_issues_count"`
	OpenPulls     int         `json:"open_pr_counter"`
	Releases      int         `json:"release_counter"`
	DefaultBranch string      `json:"default_branch"`
	Archived      bool        `json:"archived"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated                   time.Time        `json:"updated_at"`
	Permissions               *Permission      `json:"permissions,omitempty"`
	HasIssues                 bool             `json:"has_issues"`
	InternalTracker           *InternalTracker `json:"internal_tracker,omitempty"`
	ExternalTracker           *ExternalTracker `json:"external_tracker,omitempty"`
	HasWiki                   bool             `json:"has_wiki"`
	ExternalWiki              *ExternalWiki    `json:"external_wiki,omitempty"`
	HasPullRequests           bool             `json:"has_pull_requests"`
	HasProjects               bool             `json:"has_projects"`
	IgnoreWhitespaceConflicts bool             `json:"ignore_whitespace_conflicts"`
	AllowMerge                bool             `json:"allow_merge_commits"`
	AllowRebase               bool             `json:"allow_rebase"`
	AllowRebaseMerge          bool             `json:"allow_rebase_explicit"`
	AllowSquash               bool             `json:"allow_squash_merge"`
	DefaultMergeStyle         string           `json:"default_merge_style"`
	AvatarURL                 string           `json:"avatar_url"`
	Internal                  bool             `json:"internal"`
	MirrorInterval            string           `json:"mirror_interval"`
}

// CreateRepoOption options when creating repository
// swagger:model
type CreateRepoOption struct {
	// Name of the repository to create
	//
	// required: true
	// unique: true
	Name string `json:"name" binding:"Required;AlphaDashDot;MaxSize(100)"`
	// Description of the repository to create
	Description string `json:"description" binding:"MaxSize(255)"`
	// Whether the repository is private
	Private bool `json:"private"`
	// Label-Set to use
	IssueLabels string `json:"issue_labels"`
	// Whether the repository should be auto-intialized?
	AutoInit bool `json:"auto_init"`
	// Whether the repository is template
	Template bool `json:"template"`
	// Gitignores to use
	Gitignores string `json:"gitignores"`
	// License to use
	License string `json:"license"`
	// Readme of the repository to create
	Readme string `json:"readme"`
	// DefaultBranch of the repository (used when initializes and in template)
	DefaultBranch string `json:"default_branch" binding:"GitRefName;MaxSize(100)"`
	// TrustModel of the repository
	// enum: default,collaborator,committer,collaboratorcommitter
	TrustModel string `json:"trust_model"`
}

// EditRepoOption options when editing a repository's properties
// swagger:model
type EditRepoOption struct {
	// name of the repository
	// unique: true
	Name *string `json:"name,omitempty" binding:"OmitEmpty;AlphaDashDot;MaxSize(100);"`
	// a short description of the repository.
	Description *string `json:"description,omitempty" binding:"MaxSize(255)"`
	// a URL with more information about the repository.
	Website *string `json:"website,omitempty" binding:"MaxSize(255)"`
	// either `true` to make the repository private or `false` to make it public.
	// Note: you will get a 422 error if the organization restricts changing repository visibility to organization
	// owners and a non-owner tries to change the value of private.
	Private *bool `json:"private,omitempty"`
	// either `true` to make this repository a template or `false` to make it a normal repository
	Template *bool `json:"template,omitempty"`
	// either `true` to enable issues for this repository or `false` to disable them.
	HasIssues *bool `json:"has_issues,omitempty"`
	// set this structure to configure internal issue tracker (requires has_issues)
	InternalTracker *InternalTracker `json:"internal_tracker,omitempty"`
	// set this structure to use external issue tracker (requires has_issues)
	ExternalTracker *ExternalTracker `json:"external_tracker,omitempty"`
	// either `true` to enable the wiki for this repository or `false` to disable it.
	HasWiki *bool `json:"has_wiki,omitempty"`
	// set this structure to use external wiki instead of internal (requires has_wiki)
	ExternalWiki *ExternalWiki `json:"external_wiki,omitempty"`
	// sets the default branch for this repository.
	DefaultBranch *string `json:"default_branch,omitempty"`
	// either `true` to allow pull requests, or `false` to prevent pull request.
	HasPullRequests *bool `json:"has_pull_requests,omitempty"`
	// either `true` to enable project unit, or `false` to disable them.
	HasProjects *bool `json:"has_projects,omitempty"`
	// either `true` to ignore whitespace for conflicts, or `false` to not ignore whitespace. `has_pull_requests` must be `true`.
	IgnoreWhitespaceConflicts *bool `json:"ignore_whitespace_conflicts,omitempty"`
	// either `true` to allow merging pull requests with a merge commit, or `false` to prevent merging pull requests with merge commits. `has_pull_requests` must be `true`.
	AllowMerge *bool `json:"allow_merge_commits,omitempty"`
	// either `true` to allow rebase-merging pull requests, or `false` to prevent rebase-merging. `has_pull_requests` must be `true`.
	AllowRebase *bool `json:"allow_rebase,omitempty"`
	// either `true` to allow rebase with explicit merge commits (--no-ff), or `false` to prevent rebase with explicit merge commits. `has_pull_requests` must be `true`.
	AllowRebaseMerge *bool `json:"allow_rebase_explicit,omitempty"`
	// either `true` to allow squash-merging pull requests, or `false` to prevent squash-merging. `has_pull_requests` must be `true`.
	AllowSquash *bool `json:"allow_squash_merge,omitempty"`
	// either `true` to allow mark pr as merged manually, or `false` to prevent it. `has_pull_requests` must be `true`.
	AllowManualMerge *bool `json:"allow_manual_merge,omitempty"`
	// either `true` to enable AutodetectManualMerge, or `false` to prevent it. `has_pull_requests` must be `true`, Note: In some special cases, misjudgments can occur.
	AutodetectManualMerge *bool `json:"autodetect_manual_merge,omitempty"`
	// set to `true` to delete pr branch after merge by default
	DefaultDeleteBranchAfterMerge *bool `json:"default_delete_branch_after_merge,omitempty"`
	// set to a merge style to be used by this repository: "merge", "rebase", "rebase-merge", or "squash". `has_pull_requests` must be `true`.
	DefaultMergeStyle *string `json:"default_merge_style,omitempty"`
	// set to `true` to archive this repository.
	Archived *bool `json:"archived,omitempty"`
	// set to a string like `8h30m0s` to set the mirror interval time
	MirrorInterval *string `json:"mirror_interval,omitempty"`
}

// GenerateRepoOption options when creating repository using a template
// swagger:model
type GenerateRepoOption struct {
	// The organization or person who will own the new repository
	//
	// required: true
	Owner string `json:"owner"`
	// Name of the repository to create
	//
	// required: true
	// unique: true
	Name string `json:"name" binding:"Required;AlphaDashDot;MaxSize(100)"`
	// Description of the repository to create
	Description string `json:"description" binding:"MaxSize(255)"`
	// Whether the repository is private
	Private bool `json:"private"`
	// include git content of default branch in template repo
	GitContent bool `json:"git_content"`
	// include topics in template repo
	Topics bool `json:"topics"`
	// include git hooks in template repo
	GitHooks bool `json:"git_hooks"`
	// include webhooks in template repo
	Webhooks bool `json:"webhooks"`
	// include avatar of the template repo
	Avatar bool `json:"avatar"`
	// include labels in template repo
	Labels bool `json:"labels"`
}

// CreateBranchRepoOption options when creating a branch in a repository
// swagger:model
type CreateBranchRepoOption struct {

	// Name of the branch to create
	//
	// required: true
	// unique: true
	BranchName string `json:"new_branch_name" binding:"Required;GitRefName;MaxSize(100)"`

	// Name of the old branch to create from
	//
	// unique: true
	OldBranchName string `json:"old_branch_name" binding:"GitRefName;MaxSize(100)"`
}

// TransferRepoOption options when transfer a repository's ownership
// swagger:model
type TransferRepoOption struct {
	// required: true
	NewOwner string `json:"new_owner"`
	// ID of the team or teams to add to the repository. Teams can only be added to organization-owned repositories.
	TeamIDs *[]int64 `json:"team_ids"`
}

// GitServiceType represents a git service
type GitServiceType int

// enumerate all GitServiceType
const (
	NotMigrated     GitServiceType = iota // 0 not migrated from external sites
	PlainGitService                       // 1 plain git service
	GithubService                         // 2 github.com
	GiteaService                          // 3 gitea service
	GitlabService                         // 4 gitlab service
	GogsService                           // 5 gogs service
)

// Name represents the service type's name
// WARNNING: the name have to be equal to that on goth's library
func (gt GitServiceType) Name() string {
	return strings.ToLower(gt.Title())
}

// Title represents the service type's proper title
func (gt GitServiceType) Title() string {
	switch gt {
	case GithubService:
		return "GitHub"
	case GiteaService:
		return "Gitea"
	case GitlabService:
		return "GitLab"
	case GogsService:
		return "Gogs"
	case PlainGitService:
		return "Git"
	}
	return ""
}

// MigrateRepoOptions options for migrating repository's
// this is used to interact with api v1
type MigrateRepoOptions struct {
	// required: true
	CloneAddr string `json:"clone_addr" binding:"Required"`
	// deprecated (only for backwards compatibility)
	RepoOwnerID int64 `json:"uid"`
	// Name of User or Organisation who will own Repo after migration
	RepoOwner string `json:"repo_owner"`
	// required: true
	RepoName string `json:"repo_name" binding:"Required;AlphaDashDot;MaxSize(100)"`

	// enum: git,github,gitea,gitlab
	Service      string `json:"service"`
	AuthUsername string `json:"auth_username"`
	AuthPassword string `json:"auth_password"`
	AuthToken    string `json:"auth_token"`

	Mirror         bool   `json:"mirror"`
	LFS            bool   `json:"lfs"`
	LFSEndpoint    string `json:"lfs_endpoint"`
	Private        bool   `json:"private"`
	Description    string `json:"description" binding:"MaxSize(255)"`
	Wiki           bool   `json:"wiki"`
	Milestones     bool   `json:"milestones"`
	Labels         bool   `json:"labels"`
	Issues         bool   `json:"issues"`
	PullRequests   bool   `json:"pull_requests"`
	Releases       bool   `json:"releases"`
	MirrorInterval string `json:"mirror_interval"`
}

// TokenAuth represents whether a service type supports token-based auth
func (gt GitServiceType) TokenAuth() bool {
	switch gt {
	case GithubService, GiteaService, GitlabService:
		return true
	}
	return false
}

var (
	// SupportedFullGitService represents all git services supported to migrate issues/labels/prs and etc.
	// TODO: add to this list after new git service added
	SupportedFullGitService = []GitServiceType{
		GithubService,
		GitlabService,
		GiteaService,
		GogsService,
	}
)
