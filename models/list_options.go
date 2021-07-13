// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

// ListOptions options to paginate results
type ListOptions struct {
	PageSize int
	Page     int // start from 1
}

func (opts *ListOptions) getPaginatedSession() *xorm.Session {
	opts.setDefaultValues()

	return x.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
}

func (opts *ListOptions) setSessionPagination(sess *xorm.Session) *xorm.Session {
	opts.setDefaultValues()

	if opts.PageSize <= 0 {
		return sess
	}
	return sess.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
}

func (opts *ListOptions) setEnginePagination(e Engine) Engine {
	opts.setDefaultValues()

	return e.Limit(opts.PageSize, (opts.Page-1)*opts.PageSize)
}

// GetStartEnd returns the start and end of the ListOptions
func (opts *ListOptions) GetStartEnd() (start, end int) {
	opts.setDefaultValues()
	start = (opts.Page - 1) * opts.PageSize
	end = start + opts.PageSize
	return
}

func (opts *ListOptions) setDefaultValues() {
	if opts.PageSize <= 0 {
		opts.PageSize = setting.API.DefaultPagingNum
	}
	if opts.PageSize > setting.API.MaxResponseItems {
		opts.PageSize = setting.API.MaxResponseItems
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
}
