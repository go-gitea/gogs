// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"time"
)

const (
	// MediaType contains the media type for LFS server requests
	MediaType = "application/vnd.git-lfs+json"
)

// BatchRequest contains multiple requests processed in one batch operation.
// https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md#requests
type BatchRequest struct {
	Operation string     `json:"operation"`
	Transfers []string   `json:"transfers,omitempty"`
	Ref       *Reference `json:"ref,omitempty"`
	Objects   []Pointer  `json:"objects"`
}

// Reference contains a git reference.
// https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md#ref-property
type Reference struct {
	Name string `json:"name"`
}

// Pointer contains LFS pointer data
type Pointer struct {
	Oid  string `json:"oid" xorm:"UNIQUE(s) INDEX NOT NULL"`
	Size int64  `json:"size" xorm:"NOT NULL"`
}

// BatchResponse contains multiple object metadata Representation structures
// for use with the batch API.
// https://github.com/git-lfs/git-lfs/blob/main/docs/api/batch.md#successful-responses
type BatchResponse struct {
	Transfer string            `json:"transfer,omitempty"`
	Objects  []*ObjectResponse `json:"objects"`
}

// ObjectResponse is object metadata as seen by clients of the LFS server.
type ObjectResponse struct {
	Pointer
	Actions map[string]*Link `json:"actions,omitempty"`
	Error   *ObjectError     `json:"error,omitempty"`
}

// Link provides a structure with information about how to access a object.
type Link struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
}

// ObjectError defines the JSON structure returned to the client in case of an error.
type ObjectError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// PointerBlob associates a Git blob with a Pointer.
type PointerBlob struct {
	Hash string
	Pointer
}

// ErrorResponse describes the error to the client.
type ErrorResponse struct {
	Message          string
	DocumentationURL string `json:"documentation_url,omitempty"`
	RequestID        string `json:"request_id,omitempty"`
}
