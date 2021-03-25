// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/unknwon/paginater"
)

// Pagination provides a pagination via Paginater and additional configurations for the link params used in rendering
type Pagination struct {
	Paginater *paginater.Paginater
	urlParams []string
}

// NewPagination creates a new instance of the Pagination struct
func NewPagination(total, page, issueNum, numPages int) *Pagination {
	p := &Pagination{}
	p.Paginater = paginater.New(total, page, issueNum, numPages)
	return p
}

// AddParam adds a value from context identified by ctxKey as link param under a given paramKey
func (p *Pagination) AddParam(ctx *Context, paramKey, ctxKey string) {
	_, exists := ctx.Data[ctxKey]
	if !exists {
		return
	}
	paramData := fmt.Sprintf("%v", ctx.Data[ctxKey]) // cast interface{} to string
	urlParam := fmt.Sprintf("%s=%v", url.QueryEscape(paramKey), url.QueryEscape(paramData))
	p.urlParams = append(p.urlParams, urlParam)
}

// AddParamString adds a string parameter directly
func (p *Pagination) AddParamString(key, value string) {
	urlParam := fmt.Sprintf("%s=%v", url.QueryEscape(key), url.QueryEscape(value))
	p.urlParams = append(p.urlParams, urlParam)
}

// GetParams returns the configured URL params
func (p *Pagination) GetParams() template.URL {
	return template.URL(strings.Join(p.urlParams, "&"))
}

// SetDefaultParams sets common pagination params that are often used
func (p *Pagination) SetDefaultParams(ctx *Context) {
	p.AddParam(ctx, "sort", "SortType")
	p.AddParam(ctx, "q", "Keyword")
	p.AddParam(ctx, "tab", "TabName")
	p.AddParam(ctx, "t", "queryType")
}
