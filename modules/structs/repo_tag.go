// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// Tag represents a repository tag
type Tag struct {
	Name       string      `json:"name"`
	Message    string      `json:"message"`
	ID         string      `json:"id"`
	Commit     *CommitMeta `json:"commit"`
	ZipballURL string      `json:"zipball_url"`
	TarballURL string      `json:"tarball_url"`
}

// AnnotatedTag represents an annotated tag
type AnnotatedTag struct {
	Tag          string                     `json:"tag"`
	SHA          string                     `json:"sha"`
	URL          string                     `json:"url"`
	Message      string                     `json:"message"`
	Tagger       *CommitUser                `json:"tagger"`
	Object       *AnnotatedTagObject        `json:"object"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// AnnotatedTagObject contains meta information of the tag object
type AnnotatedTagObject struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	SHA  string `json:"sha"`
}

// CreateTagOption options when creating a tag
type CreateTagOption struct {
	// required: true
	TagName string `json:"tag_name" binding:"Required"`
	Message string `json:"message"`
	Target  string `json:"target"`
}
