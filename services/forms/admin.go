// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package forms

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/binding"
)

// AdminCreateUserForm form for admin to create user
type AdminCreateUserForm struct {
	LoginType          string `binding:"Required"`
	LoginName          string
	UserName           string `binding:"Required;AlphaDashDot;MaxSize(40)"`
	Email              string `binding:"Required;Email;MaxSize(254)"`
	Password           string `binding:"MaxSize(255)"`
	SendNotify         bool
	MustChangePassword bool
	Visibility         structs.VisibleType
}

// Validate validates form fields
func (f *AdminCreateUserForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// AdminEditUserForm form for admin to create user
type AdminEditUserForm struct {
	LoginType               string `binding:"Required"`
	UserName                string `binding:"AlphaDashDot;MaxSize(40)"`
	LoginName               string
	FullName                string `binding:"MaxSize(100)"`
	Email                   string `binding:"Required;Email;MaxSize(254)"`
	Password                string `binding:"MaxSize(255)"`
	Website                 string `binding:"ValidUrl;MaxSize(255)"`
	Location                string `binding:"MaxSize(50)"`
	MaxRepoCreation         int
	Active                  bool
	Admin                   bool
	Restricted              bool
	AllowGitHook            bool
	AllowImportLocal        bool
	AllowCreateOrganization bool
	ProhibitLogin           bool
	Reset2FA                bool `form:"reset_2fa"`
	Visibility              structs.VisibleType
}

// Validate validates form fields
func (f *AdminEditUserForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}

// AdminDashboardForm form for admin dashboard operations
type AdminDashboardForm struct {
	Op   string `binding:"required"`
	From string
}

// Validate validates form fields
func (f *AdminDashboardForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}
