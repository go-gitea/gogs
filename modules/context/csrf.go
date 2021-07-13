// Copyright 2013 Martini Authors
// Copyright 2014 The Macaron Authors
// Copyright 2021 The Gitea Authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// a middleware that generates and validates CSRF tokens.

package context

import (
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web/middleware"

	"github.com/unknwon/com"
)

// CSRF represents a CSRF service and is used to get the current token and validate a suspect token.
type CSRF interface {
	// Return HTTP header to search for token.
	GetHeaderName() string
	// Return form value to search for token.
	GetFormName() string
	// Return cookie name to search for token.
	GetCookieName() string
	// Return cookie path
	GetCookiePath() string
	// Return the flag value used for the csrf token.
	GetCookieHTTPOnly() bool
	// Return cookie domain
	GetCookieDomain() string
	// Return the token.
	GetToken() string
	// Validate by token.
	ValidToken(t string) bool
	// Error replies to the request with a custom function when ValidToken fails.
	Error(w http.ResponseWriter)
}

type csrf struct {
	// Header name value for setting and getting csrf token.
	Header string
	// Form name value for setting and getting csrf token.
	Form string
	// Cookie name value for setting and getting csrf token.
	Cookie string
	//Cookie domain
	CookieDomain string
	//Cookie path
	CookiePath string
	// Cookie HttpOnly flag value used for the csrf token.
	CookieHTTPOnly bool
	// Token generated to pass via header, cookie, or hidden form value.
	Token string
	// This value must be unique per user.
	ID string
	// Secret used along with the unique id above to generate the Token.
	Secret string
	// ErrorFunc is the custom function that replies to the request when ValidToken fails.
	ErrorFunc func(w http.ResponseWriter)
}

// GetHeaderName returns the name of the HTTP header for csrf token.
func (c *csrf) GetHeaderName() string {
	return c.Header
}

// GetFormName returns the name of the form value for csrf token.
func (c *csrf) GetFormName() string {
	return c.Form
}

// GetCookieName returns the name of the cookie for csrf token.
func (c *csrf) GetCookieName() string {
	return c.Cookie
}

// GetCookiePath returns the path of the cookie for csrf token.
func (c *csrf) GetCookiePath() string {
	return c.CookiePath
}

// GetCookieHTTPOnly returns the flag value used for the csrf token.
func (c *csrf) GetCookieHTTPOnly() bool {
	return c.CookieHTTPOnly
}

// GetCookieDomain returns the flag value used for the csrf token.
func (c *csrf) GetCookieDomain() string {
	return c.CookieDomain
}

// GetToken returns the current token. This is typically used
// to populate a hidden form in an HTML template.
func (c *csrf) GetToken() string {
	return c.Token
}

// ValidToken validates the passed token against the existing Secret and ID.
func (c *csrf) ValidToken(t string) bool {
	return ValidToken(t, c.Secret, c.ID, "POST")
}

// Error replies to the request when ValidToken fails.
func (c *csrf) Error(w http.ResponseWriter) {
	c.ErrorFunc(w)
}

// CsrfOptions maintains options to manage behavior of Generate.
type CsrfOptions struct {
	// The global secret value used to generate Tokens.
	Secret string
	// HTTP header used to set and get token.
	Header string
	// Form value used to set and get token.
	Form string
	// Cookie value used to set and get token.
	Cookie string
	// Cookie domain.
	CookieDomain string
	// Cookie path.
	CookiePath     string
	CookieHTTPOnly bool
	// SameSite set the cookie SameSite type
	SameSite http.SameSite
	// Key used for getting the unique ID per user.
	SessionKey string
	// oldSessionKey saves old value corresponding to SessionKey.
	oldSessionKey string
	// If true, send token via X-CSRFToken header.
	SetHeader bool
	// If true, send token via _csrf cookie.
	SetCookie bool
	// Set the Secure flag to true on the cookie.
	Secure bool
	// Disallow Origin appear in request header.
	Origin bool
	// The function called when Validate fails.
	ErrorFunc func(w http.ResponseWriter)
	// Cookie life time. Default is 0
	CookieLifeTime int
}

func prepareOptions(options []CsrfOptions) CsrfOptions {
	var opt CsrfOptions
	if len(options) > 0 {
		opt = options[0]
	}

	// Defaults.
	if len(opt.Secret) == 0 {
		opt.Secret = string(com.RandomCreateBytes(10))
	}
	if len(opt.Header) == 0 {
		opt.Header = "X-CSRFToken"
	}
	if len(opt.Form) == 0 {
		opt.Form = "_csrf"
	}
	if len(opt.Cookie) == 0 {
		opt.Cookie = "_csrf"
	}
	if len(opt.CookiePath) == 0 {
		opt.CookiePath = "/"
	}
	if len(opt.SessionKey) == 0 {
		opt.SessionKey = "uid"
	}
	opt.oldSessionKey = "_old_" + opt.SessionKey
	if opt.ErrorFunc == nil {
		opt.ErrorFunc = func(w http.ResponseWriter) {
			http.Error(w, "Invalid csrf token.", http.StatusBadRequest)
		}
	}

	return opt
}

// Csrfer maps CSRF to each request. If this request is a Get request, it will generate a new token.
// Additionally, depending on options set, generated tokens will be sent via Header and/or Cookie.
func Csrfer(opt CsrfOptions, ctx *Context) CSRF {
	opt = prepareOptions([]CsrfOptions{opt})
	x := &csrf{
		Secret:         opt.Secret,
		Header:         opt.Header,
		Form:           opt.Form,
		Cookie:         opt.Cookie,
		CookieDomain:   opt.CookieDomain,
		CookiePath:     opt.CookiePath,
		CookieHTTPOnly: opt.CookieHTTPOnly,
		ErrorFunc:      opt.ErrorFunc,
	}

	if opt.Origin && len(ctx.Req.Header.Get("Origin")) > 0 {
		return x
	}

	x.ID = "0"
	uid := ctx.Session.Get(opt.SessionKey)
	if uid != nil {
		x.ID = com.ToStr(uid)
	}

	needsNew := false
	oldUID := ctx.Session.Get(opt.oldSessionKey)
	if oldUID == nil || oldUID.(string) != x.ID {
		needsNew = true
		_ = ctx.Session.Set(opt.oldSessionKey, x.ID)
	} else {
		// If cookie present, map existing token, else generate a new one.
		if val := ctx.GetCookie(opt.Cookie); len(val) > 0 {
			// FIXME: test coverage.
			x.Token = val
		} else {
			needsNew = true
		}
	}

	if needsNew {
		// FIXME: actionId.
		x.Token = GenerateToken(x.Secret, x.ID, "POST")
		if opt.SetCookie {
			var expires interface{}
			if opt.CookieLifeTime == 0 {
				expires = time.Now().AddDate(0, 0, 1)
			}
			middleware.SetCookie(ctx.Resp, opt.Cookie, x.Token,
				opt.CookieLifeTime,
				opt.CookiePath,
				opt.CookieDomain,
				opt.Secure,
				opt.CookieHTTPOnly,
				expires,
				middleware.SameSite(opt.SameSite),
			)
		}
	}

	if opt.SetHeader {
		ctx.Resp.Header().Add(opt.Header, x.Token)
	}
	return x
}

// Validate should be used as a per route middleware. It attempts to get a token from a "X-CSRFToken"
// HTTP header and then a "_csrf" form value. If one of these is found, the token will be validated
// using ValidToken. If this validation fails, custom Error is sent in the reply.
// If neither a header or form value is found, http.StatusBadRequest is sent.
func Validate(ctx *Context, x CSRF) {
	if token := ctx.Req.Header.Get(x.GetHeaderName()); len(token) > 0 {
		if !x.ValidToken(token) {
			// Delete the cookie
			middleware.SetCookie(ctx.Resp, x.GetCookieName(), "",
				-1,
				x.GetCookiePath(),
				x.GetCookieDomain()) // FIXME: Do we need to set the Secure, httpOnly and SameSite values too?
			if middleware.IsAPIPath(ctx.Req) {
				x.Error(ctx.Resp)
				return
			}
			ctx.Flash.Error(ctx.Tr("error.invalid_csrf"))
			ctx.Redirect(setting.AppSubURL + "/")
		}
		return
	}
	if token := ctx.Req.FormValue(x.GetFormName()); len(token) > 0 {
		if !x.ValidToken(token) {
			// Delete the cookie
			middleware.SetCookie(ctx.Resp, x.GetCookieName(), "",
				-1,
				x.GetCookiePath(),
				x.GetCookieDomain()) // FIXME: Do we need to set the Secure, httpOnly and SameSite values too?
			if middleware.IsAPIPath(ctx.Req) {
				x.Error(ctx.Resp)
				return
			}
			ctx.Flash.Error(ctx.Tr("error.invalid_csrf"))
			ctx.Redirect(setting.AppSubURL + "/")
		}
		return
	}
	if middleware.IsAPIPath(ctx.Req) {
		http.Error(ctx.Resp, "Bad Request: no CSRF token present", http.StatusBadRequest)
		return
	}
	ctx.Flash.Error(ctx.Tr("error.missing_csrf"))
	ctx.Redirect(setting.AppSubURL + "/")
}
