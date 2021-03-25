// Copyright 2020 The Macaron Authors
// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package middleware

import (
	"net/http"
	"net/url"
	"time"

	"code.gitea.io/gitea/modules/setting"
)

// MaxAge sets the maximum age for a provided cookie
func MaxAge(maxAge int) func(*http.Cookie) {
	return func(c *http.Cookie) {
		c.MaxAge = maxAge
	}
}

// Path sets the path for a provided cookie
func Path(path string) func(*http.Cookie) {
	return func(c *http.Cookie) {
		c.Path = path
	}
}

// Domain sets the domain for a provided cookie
func Domain(domain string) func(*http.Cookie) {
	return func(c *http.Cookie) {
		c.Domain = domain
	}
}

// Secure sets the secure setting for a provided cookie
func Secure(secure bool) func(*http.Cookie) {
	return func(c *http.Cookie) {
		c.Secure = secure
	}
}

// HTTPOnly sets the HttpOnly setting for a provided cookie
func HTTPOnly(httpOnly bool) func(*http.Cookie) {
	return func(c *http.Cookie) {
		c.HttpOnly = httpOnly
	}
}

// Expires sets the expires and rawexpires for a provided cookie
func Expires(expires time.Time) func(*http.Cookie) {
	return func(c *http.Cookie) {
		c.Expires = expires
		c.RawExpires = expires.Format(time.UnixDate)
	}
}

// SameSite sets the SameSite for a provided cookie
func SameSite(sameSite http.SameSite) func(*http.Cookie) {
	return func(c *http.Cookie) {
		c.SameSite = sameSite
	}
}

// NewCookie creates a cookie
func NewCookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		HttpOnly: true,
		Path:     setting.SessionConfig.CookiePath,
		Domain:   setting.SessionConfig.Domain,
		MaxAge:   maxAge,
		Secure:   setting.SessionConfig.Secure,
	}
}

// SetRedirectToCookie convenience function to set the RedirectTo cookie consistently
func SetRedirectToCookie(resp http.ResponseWriter, value string) {
	SetCookie(resp, "redirect_to", value,
		0,
		setting.AppSubURL,
		"",
		setting.SessionConfig.Secure,
		true,
		SameSite(setting.SessionConfig.SameSite))
}

// DeleteRedirectToCookie convenience function to delete most cookies consistently
func DeleteRedirectToCookie(resp http.ResponseWriter) {
	SetCookie(resp, "redirect_to", "",
		-1,
		setting.AppSubURL,
		"",
		setting.SessionConfig.Secure,
		true,
		SameSite(setting.SessionConfig.SameSite))
}

// DeleteSesionConfigPathCookie convenience function to delete SessionConfigPath cookies consistently
func DeleteSesionConfigPathCookie(resp http.ResponseWriter, name string) {
	SetCookie(resp, name, "",
		-1,
		setting.SessionConfig.CookiePath,
		setting.SessionConfig.Domain,
		setting.SessionConfig.Secure,
		true,
		SameSite(setting.SessionConfig.SameSite))
}

// DeleteCSRFCookie convenience function to delete SessionConfigPath cookies consistently
func DeleteCSRFCookie(resp http.ResponseWriter) {
	SetCookie(resp, setting.CSRFCookieName, "",
		-1,
		setting.SessionConfig.CookiePath,
		setting.SessionConfig.Domain) // FIXME: Do we need to set the Secure, httpOnly and SameSite values too?
}

// SetCookie set the cookies
// TODO: Copied from gitea.com/macaron/macaron and should be improved after macaron removed.
func SetCookie(resp http.ResponseWriter, name, value string, others ...interface{}) {
	cookie := http.Cookie{}
	cookie.Name = name
	cookie.Value = url.QueryEscape(value)

	if len(others) > 0 {
		switch v := others[0].(type) {
		case int:
			cookie.MaxAge = v
		case int64:
			cookie.MaxAge = int(v)
		case int32:
			cookie.MaxAge = int(v)
		case func(*http.Cookie):
			v(&cookie)
		}
	}

	cookie.Path = "/"
	if len(others) > 1 {
		if v, ok := others[1].(string); ok && len(v) > 0 {
			cookie.Path = v
		} else if v, ok := others[1].(func(*http.Cookie)); ok {
			v(&cookie)
		}
	}

	if len(others) > 2 {
		if v, ok := others[2].(string); ok && len(v) > 0 {
			cookie.Domain = v
		} else if v, ok := others[1].(func(*http.Cookie)); ok {
			v(&cookie)
		}
	}

	if len(others) > 3 {
		switch v := others[3].(type) {
		case bool:
			cookie.Secure = v
		case func(*http.Cookie):
			v(&cookie)
		default:
			if others[3] != nil {
				cookie.Secure = true
			}
		}
	}

	if len(others) > 4 {
		if v, ok := others[4].(bool); ok && v {
			cookie.HttpOnly = true
		} else if v, ok := others[1].(func(*http.Cookie)); ok {
			v(&cookie)
		}
	}

	if len(others) > 5 {
		if v, ok := others[5].(time.Time); ok {
			cookie.Expires = v
			cookie.RawExpires = v.Format(time.UnixDate)
		} else if v, ok := others[1].(func(*http.Cookie)); ok {
			v(&cookie)
		}
	}

	if len(others) > 6 {
		for _, other := range others[6:] {
			if v, ok := other.(func(*http.Cookie)); ok {
				v(&cookie)
			}
		}
	}

	resp.Header().Add("Set-Cookie", cookie.String())
}

// GetCookie returns given cookie value from request header.
func GetCookie(req *http.Request, name string) string {
	cookie, err := req.Cookie(name)
	if err != nil {
		return ""
	}
	val, _ := url.QueryUnescape(cookie.Value)
	return val
}
