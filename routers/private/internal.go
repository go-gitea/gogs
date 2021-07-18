// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"net/http"
	"reflect"
	"strings"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/web"

	"gitea.com/go-chi/binding"
)

// CheckInternalToken check internal token is set
func CheckInternalToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		tokens := req.Header.Get("Authorization")
		fields := strings.SplitN(tokens, " ", 2)
		if len(fields) != 2 || fields[0] != "Bearer" || fields[1] != setting.InternalToken {
			log.Debug("Forbidden attempt to access internal url: Authorization header: %s", tokens)
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		} else {
			next.ServeHTTP(w, req)
		}
	})
}

// bind binding an obj to a handler
func bind(obj interface{}) http.HandlerFunc {
	var tp = reflect.TypeOf(obj)
	for tp.Kind() == reflect.Ptr {
		tp = tp.Elem()
	}
	return web.Wrap(func(ctx *context.PrivateContext) {
		var theObj = reflect.New(tp).Interface() // create a new form obj for every request but not use obj directly
		binding.Bind(ctx.Req, theObj)
		web.SetForm(ctx, theObj)
	})
}

// Routes registers all internal APIs routes to web application.
// These APIs will be invoked by internal commands for example `gitea serv` and etc.
func Routes() *web.Route {
	var r = web.NewRoute()
	r.Use(context.PrivateContexter())
	r.Use(CheckInternalToken)

	r.Post("/ssh/authorized_keys", AuthorizedPublicKeyByContent)
	r.Post("/ssh/{id}/update/{repoid}", UpdatePublicKeyInRepo)
	r.Post("/ssh/log", bind(private.SSHLogOption{}), SSHLog)
	r.Post("/hook/pre-receive/{owner}/{repo}", bind(private.HookOptions{}), HookPreReceive)
	r.Post("/hook/post-receive/{owner}/{repo}", bind(private.HookOptions{}), HookPostReceive)
	r.Post("/hook/proc-receive/{owner}/{repo}", bind(private.HookOptions{}), HookProcReceive)
	r.Post("/hook/set-default-branch/{owner}/{repo}/{branch}", SetDefaultBranch)
	r.Get("/serv/none/{keyid}", ServNoCommand)
	r.Get("/serv/command/{keyid}/{owner}/{repo}", ServCommand)
	r.Post("/manager/shutdown", Shutdown)
	r.Post("/manager/restart", Restart)
	r.Post("/manager/flush-queues", bind(private.FlushOptions{}), FlushQueues)
	r.Post("/manager/pause-logging", PauseLogging)
	r.Post("/manager/resume-logging", ResumeLogging)
	r.Post("/manager/release-and-reopen-logging", ReleaseReopenLogging)
	r.Post("/manager/add-logger", bind(private.LoggerOptions{}), AddLogger)
	r.Post("/manager/remove-logger/{group}/{name}", RemoveLogger)
	r.Post("/mail/send", SendEmail)
	r.Post("/restore_repo", RestoreRepo)

	return r
}
