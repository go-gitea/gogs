// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"html"
	"net/http"
	"strings"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/session"
)

// APIUIContexter returns apicontext as middleware
func APIUIContexter() func(http.Handler) http.Handler {
	var csrfOpts = getCsrfOpts()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			var locale = middleware.Locale(w, req)
			var ctx = APIContext{
				Context: &Context{
					Resp:    NewResponse(w),
					Data:    map[string]interface{}{},
					Locale:  locale,
					Session: session.GetSession(req),
					Repo: &Repository{
						PullRequest: &PullRequest{},
					},
					Org: &Organization{},
				},
				Org: &APIOrganization{},
			}

			ctx.Req = WithAPIContext(WithContext(req, ctx.Context), &ctx)
			ctx.csrf = Csrfer(csrfOpts, ctx.Context)

			// If request sends files, parse them here otherwise the Query() can't be parsed and the CsrfToken will be invalid.
			if ctx.Req.Method == "POST" && strings.Contains(ctx.Req.Header.Get("Content-Type"), "multipart/form-data") {
				if err := ctx.Req.ParseMultipartForm(setting.Attachment.MaxSize << 20); err != nil && !strings.Contains(err.Error(), "EOF") { // 32MB max size
					ctx.InternalServerError(err)
					return
				}
			}

			ctx.Resp.Header().Set(`X-Frame-Options`, `SAMEORIGIN`)

			ctx.Data["CsrfToken"] = html.EscapeString(ctx.csrf.GetToken())

			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}
