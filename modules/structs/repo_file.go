// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// FileOptions options for all file APIs
type FileOptions struct {
	// message (optional) for the commit of this file. if not supplied, a default message will be used
	Message string `json:"message"`
	// branch (optional) to base this file from. if not given, the default branch is used
	BranchName string `json:"branch" binding:"GitRefName;MaxSize(100)"`
	// new_branch (optional) will make a new branch from `branch` before creating the file
	NewBranchName string `json:"new_branch" binding:"GitRefName;MaxSize(100)"`
	// `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
	Author    Identity          `json:"author"`
	Committer Identity          `json:"committer"`
	Dates     CommitDateOptions `json:"dates"`
}

// CreateFileOptions options for creating files
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type CreateFileOptions struct {
	FileOptions
	// content must be base64 encoded
	// required: true
	Content string `json:"content"`
}

// DeleteFileOptions options for deleting files (used for other File structs below)
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type DeleteFileOptions struct {
	FileOptions
	// sha is the SHA for the file that already exists
	// required: true
	SHA string `json:"sha" binding:"Required"`
}

// UpdateFileOptions options for updating files
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type UpdateFileOptions struct {
	DeleteFileOptions
	// content must be base64 encoded
	// required: true
	Content string `json:"content"`
	// from_path (optional) is the path of the original file which will be moved/renamed to the path in the URL
	FromPath string `json:"from_path" binding:"MaxSize(500)"`
}

// ApplyDiffPatchFileOptions options for applying a diff patch
// Note: `author` and `committer` are optional (if only one is given, it will be used for the other, otherwise the authenticated user will be used)
type ApplyDiffPatchFileOptions struct {
	DeleteFileOptions
	// required: true
	Content string `json:"content"`
}

// FileLinksResponse contains the links for a repo's file
type FileLinksResponse struct {
	Self    *string `json:"self"`
	GitURL  *string `json:"git"`
	HTMLURL *string `json:"html"`
}

// ContentsResponse contains information about a repo's entry's (dir, file, symlink, submodule) metadata and content
type ContentsResponse struct {
	Name string `json:"name"`
	Path string `json:"path"`
	SHA  string `json:"sha"`
	// `type` will be `file`, `dir`, `symlink`, or `submodule`
	Type string `json:"type"`
	Size int64  `json:"size"`
	// `encoding` is populated when `type` is `file`, otherwise null
	Encoding *string `json:"encoding"`
	// `content` is populated when `type` is `file`, otherwise null
	Content *string `json:"content"`
	// `target` is populated when `type` is `symlink`, otherwise null
	Target      *string `json:"target"`
	URL         *string `json:"url"`
	HTMLURL     *string `json:"html_url"`
	GitURL      *string `json:"git_url"`
	DownloadURL *string `json:"download_url"`
	// `submodule_git_url` is populated when `type` is `submodule`, otherwise null
	SubmoduleGitURL *string            `json:"submodule_git_url"`
	Links           *FileLinksResponse `json:"_links"`
}

// FileCommitResponse contains information generated from a Git commit for a repo's file.
type FileCommitResponse struct {
	CommitMeta
	HTMLURL   string        `json:"html_url"`
	Author    *CommitUser   `json:"author"`
	Committer *CommitUser   `json:"committer"`
	Parents   []*CommitMeta `json:"parents"`
	Message   string        `json:"message"`
	Tree      *CommitMeta   `json:"tree"`
}

// FileResponse contains information about a repo's file
type FileResponse struct {
	Content      *ContentsResponse          `json:"content"`
	Commit       *FileCommitResponse        `json:"commit"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// FileDeleteResponse contains information about a repo's file that was deleted
type FileDeleteResponse struct {
	Content      interface{}                `json:"content"` // to be set to nil
	Commit       *FileCommitResponse        `json:"commit"`
	Verification *PayloadCommitVerification `json:"verification"`
}
