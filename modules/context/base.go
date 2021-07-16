// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"
	"github.com/go-chi/chi"
	jsoniter "github.com/json-iterator/go"
)

type BaseContext struct {
	Resp ResponseWriter
	Req  *http.Request
	Data map[string]interface{}
}

func NewBaseContext(resp http.ResponseWriter, req *http.Request, data map[string]interface{}) *BaseContext {
	return &BaseContext{
		Resp: NewResponse(resp),
		Req:  req,
		Data: data,
	}
}

// GetData returns the data
func (ctx *BaseContext) GetData() map[string]interface{} {
	return ctx.Data
}

// HasValue returns true if value of given name exists.
func (ctx *BaseContext) HasValue(name string) bool {
	_, ok := ctx.Data[name]
	return ok
}

// Header returns a header
func (ctx *BaseContext) Header() http.Header {
	return ctx.Resp.Header()
}

// RemoteAddr returns the client machie ip address
func (ctx *BaseContext) RemoteAddr() string {
	return ctx.Req.RemoteAddr
}

// Params returns the param on route
func (ctx *BaseContext) Params(p string) string {
	s, _ := url.PathUnescape(chi.URLParam(ctx.Req, strings.TrimPrefix(p, ":")))
	return s
}

// ParamsInt64 returns the param on route as int64
func (ctx *BaseContext) ParamsInt64(p string) int64 {
	v, _ := strconv.ParseInt(ctx.Params(p), 10, 64)
	return v
}

// SetParams set params into routes
func (ctx *BaseContext) SetParams(k, v string) {
	chiCtx := chi.RouteContext(ctx)
	chiCtx.URLParams.Add(strings.TrimPrefix(k, ":"), url.PathEscape(v))
}

// Write writes data to webbrowser
func (ctx *BaseContext) Write(bs []byte) (int, error) {
	return ctx.Resp.Write(bs)
}

// Written returns true if there are something sent to web browser
func (ctx *BaseContext) Written() bool {
	return ctx.Resp.Status() > 0
}

// Status writes status code
func (ctx *BaseContext) Status(status int) {
	ctx.Resp.WriteHeader(status)
}

// Deadline is part of the interface for context.Context and we pass this to the request context
func (ctx *BaseContext) Deadline() (deadline time.Time, ok bool) {
	return ctx.Req.Context().Deadline()
}

// Done is part of the interface for context.Context and we pass this to the request context
func (ctx *BaseContext) Done() <-chan struct{} {
	return ctx.Req.Context().Done()
}

// Err is part of the interface for context.Context and we pass this to the request context
func (ctx *BaseContext) Err() error {
	return ctx.Req.Context().Err()
}

// Value is part of the interface for context.Context and we pass this to the request context
func (ctx *BaseContext) Value(key interface{}) interface{} {
	return ctx.Req.Context().Value(key)
}

// FIXME: We should differ Query and Form, currently we just use form as query
// Currently to be compatible with macaron, we keep it.

// Query returns request form as string with default
func (ctx *BaseContext) Query(key string, defaults ...string) string {
	return (*Forms)(ctx.Req).MustString(key, defaults...)
}

// QueryTrim returns request form as string with default and trimmed spaces
func (ctx *BaseContext) QueryTrim(key string, defaults ...string) string {
	return (*Forms)(ctx.Req).MustTrimmed(key, defaults...)
}

// QueryStrings returns request form as strings with default
func (ctx *BaseContext) QueryStrings(key string, defaults ...[]string) []string {
	return (*Forms)(ctx.Req).MustStrings(key, defaults...)
}

// QueryInt returns request form as int with default
func (ctx *BaseContext) QueryInt(key string, defaults ...int) int {
	return (*Forms)(ctx.Req).MustInt(key, defaults...)
}

// QueryInt64 returns request form as int64 with default
func (ctx *BaseContext) QueryInt64(key string, defaults ...int64) int64 {
	return (*Forms)(ctx.Req).MustInt64(key, defaults...)
}

// QueryBool returns request form as bool with default
func (ctx *BaseContext) QueryBool(key string, defaults ...bool) bool {
	return (*Forms)(ctx.Req).MustBool(key, defaults...)
}

// QueryOptionalBool returns request form as OptionalBool with default
func (ctx *BaseContext) QueryOptionalBool(key string, defaults ...util.OptionalBool) util.OptionalBool {
	return (*Forms)(ctx.Req).MustOptionalBool(key, defaults...)
}

// JSON render content as JSON
func (ctx *BaseContext) JSON(status int, content interface{}) {
	ctx.Resp.Header().Set("Content-Type", "application/json;charset=utf-8")
	ctx.Resp.WriteHeader(status)
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.NewEncoder(ctx.Resp).Encode(content); err != nil {
		log.Error("Render JSON failed: %v", err)
		ctx.Status(500)
	}
}

// PlainText render content as plain text
func (ctx *BaseContext) PlainText(status int, bs []byte) {
	ctx.Resp.WriteHeader(status)
	ctx.Resp.Header().Set("Content-Type", "text/plain;charset=utf-8")
	if _, err := ctx.Resp.Write(bs); err != nil {
		log.Error("Render PlainText failed: %v", err)
		ctx.Status(500)
	}
}
