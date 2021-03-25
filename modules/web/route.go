// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package web

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/binding"
	"github.com/go-chi/chi"
)

// Wrap converts all kinds of routes to standard library one
func Wrap(handlers ...interface{}) http.HandlerFunc {
	if len(handlers) == 0 {
		panic("No handlers found")
	}

	for _, handler := range handlers {
		switch t := handler.(type) {
		case http.HandlerFunc, func(http.ResponseWriter, *http.Request),
			func(ctx *context.Context),
			func(*context.APIContext),
			func(*context.PrivateContext),
			func(http.Handler) http.Handler:
		default:
			panic(fmt.Sprintf("Unsupported handler type: %#v", t))
		}
	}
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		for i := 0; i < len(handlers); i++ {
			handler := handlers[i]
			switch t := handler.(type) {
			case http.HandlerFunc:
				t(resp, req)
				if r, ok := resp.(context.ResponseWriter); ok && r.Status() > 0 {
					return
				}
			case func(http.ResponseWriter, *http.Request):
				t(resp, req)
				if r, ok := resp.(context.ResponseWriter); ok && r.Status() > 0 {
					return
				}
			case func(ctx *context.Context):
				ctx := context.GetContext(req)
				t(ctx)
				if ctx.Written() {
					return
				}
			case func(*context.APIContext):
				ctx := context.GetAPIContext(req)
				t(ctx)
				if ctx.Written() {
					return
				}
			case func(*context.PrivateContext):
				ctx := context.GetPrivateContext(req)
				t(ctx)
				if ctx.Written() {
					return
				}
			case func(http.Handler) http.Handler:
				next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
				t(next).ServeHTTP(resp, req)
				if r, ok := resp.(context.ResponseWriter); ok && r.Status() > 0 {
					return
				}
			default:
				panic(fmt.Sprintf("Unsupported handler type: %#v", t))
			}
		}
	})
}

// Middle wrap a context function as a chi middleware
func Middle(f func(ctx *context.Context)) func(netx http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx := context.GetContext(req)
			f(ctx)
			if ctx.Written() {
				return
			}
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

// MiddleAPI wrap a context function as a chi middleware
func MiddleAPI(f func(ctx *context.APIContext)) func(netx http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			ctx := context.GetAPIContext(req)
			f(ctx)
			if ctx.Written() {
				return
			}
			next.ServeHTTP(ctx.Resp, ctx.Req)
		})
	}
}

// Bind binding an obj to a handler
func Bind(obj interface{}) http.HandlerFunc {
	tp := reflect.TypeOf(obj)
	if tp.Kind() == reflect.Ptr {
		tp = tp.Elem()
	}
	if tp.Kind() != reflect.Struct {
		panic("Only structs are allowed to bind")
	}
	return Wrap(func(ctx *context.Context) {
		theObj := reflect.New(tp).Interface() // create a new form obj for every request but not use obj directly
		binding.Bind(ctx.Req, theObj)
		SetForm(ctx, theObj)
		middleware.AssignForm(theObj, ctx.Data)
	})
}

// SetForm set the form object
func SetForm(data middleware.DataStore, obj interface{}) {
	data.GetData()["__form"] = obj
}

// GetForm returns the validate form information
func GetForm(data middleware.DataStore) interface{} {
	return data.GetData()["__form"]
}

// Route defines a route based on chi's router
type Route struct {
	R              chi.Router
	curGroupPrefix string
	curMiddlewares []interface{}
}

// NewRoute creates a new route
func NewRoute() *Route {
	r := chi.NewRouter()
	return &Route{
		R:              r,
		curGroupPrefix: "",
		curMiddlewares: []interface{}{},
	}
}

// Use supports two middlewares
func (r *Route) Use(middlewares ...interface{}) {
	if r.curGroupPrefix != "" {
		r.curMiddlewares = append(r.curMiddlewares, middlewares...)
	} else {
		for _, middle := range middlewares {
			switch t := middle.(type) {
			case func(http.Handler) http.Handler:
				r.R.Use(t)
			case func(*context.Context):
				r.R.Use(Middle(t))
			case func(*context.APIContext):
				r.R.Use(MiddleAPI(t))
			default:
				panic(fmt.Sprintf("Unsupported middleware type: %#v", t))
			}
		}
	}
}

// Group mounts a sub-Router along a `pattern` string.
func (r *Route) Group(pattern string, fn func(), middlewares ...interface{}) {
	previousGroupPrefix := r.curGroupPrefix
	previousMiddlewares := r.curMiddlewares
	r.curGroupPrefix += pattern
	r.curMiddlewares = append(r.curMiddlewares, middlewares...)

	fn()

	r.curGroupPrefix = previousGroupPrefix
	r.curMiddlewares = previousMiddlewares
}

func (r *Route) getPattern(pattern string) string {
	newPattern := r.curGroupPrefix + pattern
	if !strings.HasPrefix(newPattern, "/") {
		newPattern = "/" + newPattern
	}
	if newPattern == "/" {
		return newPattern
	}
	return strings.TrimSuffix(newPattern, "/")
}

// Mount attaches another Route along ./pattern/*
func (r *Route) Mount(pattern string, subR *Route) {
	middlewares := make([]interface{}, len(r.curMiddlewares))
	copy(middlewares, r.curMiddlewares)
	subR.Use(middlewares...)
	r.R.Mount(r.getPattern(pattern), subR.R)
}

// Any delegate requests for all methods
func (r *Route) Any(pattern string, h ...interface{}) {
	middlewares := r.getMiddlewares(h)
	r.R.HandleFunc(r.getPattern(pattern), Wrap(middlewares...))
}

// Route delegate special methods
func (r *Route) Route(pattern, methods string, h ...interface{}) {
	p := r.getPattern(pattern)
	ms := strings.Split(methods, ",")
	middlewares := r.getMiddlewares(h)
	for _, method := range ms {
		r.R.MethodFunc(strings.TrimSpace(method), p, Wrap(middlewares...))
	}
}

// Delete delegate delete method
func (r *Route) Delete(pattern string, h ...interface{}) {
	middlewares := r.getMiddlewares(h)
	r.R.Delete(r.getPattern(pattern), Wrap(middlewares...))
}

func (r *Route) getMiddlewares(h []interface{}) []interface{} {
	middlewares := make([]interface{}, len(r.curMiddlewares), len(r.curMiddlewares)+len(h))
	copy(middlewares, r.curMiddlewares)
	middlewares = append(middlewares, h...)
	return middlewares
}

// Get delegate get method
func (r *Route) Get(pattern string, h ...interface{}) {
	middlewares := r.getMiddlewares(h)
	r.R.Get(r.getPattern(pattern), Wrap(middlewares...))
}

// Head delegate head method
func (r *Route) Head(pattern string, h ...interface{}) {
	middlewares := r.getMiddlewares(h)
	r.R.Head(r.getPattern(pattern), Wrap(middlewares...))
}

// Post delegate post method
func (r *Route) Post(pattern string, h ...interface{}) {
	middlewares := r.getMiddlewares(h)
	r.R.Post(r.getPattern(pattern), Wrap(middlewares...))
}

// Put delegate put method
func (r *Route) Put(pattern string, h ...interface{}) {
	middlewares := r.getMiddlewares(h)
	r.R.Put(r.getPattern(pattern), Wrap(middlewares...))
}

// Patch delegate patch method
func (r *Route) Patch(pattern string, h ...interface{}) {
	middlewares := r.getMiddlewares(h)
	r.R.Patch(r.getPattern(pattern), Wrap(middlewares...))
}

// ServeHTTP implements http.Handler
func (r *Route) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.R.ServeHTTP(w, req)
}

// NotFound defines a handler to respond whenever a route could
// not be found.
func (r *Route) NotFound(h http.HandlerFunc) {
	r.R.NotFound(h)
}

// MethodNotAllowed defines a handler to respond whenever a method is
// not allowed.
func (r *Route) MethodNotAllowed(h http.HandlerFunc) {
	r.R.MethodNotAllowed(h)
}

// Combo deletegate requests to Combo
func (r *Route) Combo(pattern string, h ...interface{}) *Combo {
	return &Combo{r, pattern, h}
}

// Combo represents a tiny group routes with same pattern
type Combo struct {
	r       *Route
	pattern string
	h       []interface{}
}

// Get deletegate Get method
func (c *Combo) Get(h ...interface{}) *Combo {
	c.r.Get(c.pattern, append(c.h, h...)...)
	return c
}

// Post deletegate Post method
func (c *Combo) Post(h ...interface{}) *Combo {
	c.r.Post(c.pattern, append(c.h, h...)...)
	return c
}

// Delete deletegate Delete method
func (c *Combo) Delete(h ...interface{}) *Combo {
	c.r.Delete(c.pattern, append(c.h, h...)...)
	return c
}

// Put deletegate Put method
func (c *Combo) Put(h ...interface{}) *Combo {
	c.r.Put(c.pattern, append(c.h, h...)...)
	return c
}

// Patch deletegate Patch method
func (c *Combo) Patch(h ...interface{}) *Combo {
	c.r.Patch(c.pattern, append(c.h, h...)...)
	return c
}
