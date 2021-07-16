// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"context"
	"net/http"
)

// PrivateContext represents a context for private routes
type PrivateContext = BaseContext

var (
	privateContextKey interface{} = "default_private_context"
)

// WithPrivateContext set up private context in request
func WithPrivateContext(req *http.Request, ctx *PrivateContext) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), privateContextKey, ctx))
}

// GetPrivateContext returns a context for Private routes
func GetPrivateContext(req *http.Request) *PrivateContext {
	return req.Context().Value(privateContextKey).(*PrivateContext)
}

// PrivateContexter returns apicontext as middleware
func PrivateContexter() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := NewBaseContext(w, req, map[string]interface{}{})
			ctx.Req = WithPrivateContext(req, ctx)
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}
