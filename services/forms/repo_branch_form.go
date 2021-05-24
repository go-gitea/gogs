// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forms

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/binding"
)

// NewBranchForm form for creating a new branch
type NewBranchForm struct {
	NewBranchName string `binding:"Required;MaxSize(100);GitRefName"`
	CreateTag     bool
}

// Validate validates the fields
func (f *NewBranchForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// RenameBranchForm form for rename a branch
type RenameBranchForm struct {
	From string `binding:"Required;MaxSize(100);GitRefName"`
	To   string `binding:"Required;MaxSize(100);GitRefName"`
}

// Validate validates the fields
func (f *RenameBranchForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}
