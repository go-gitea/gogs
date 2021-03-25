// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routes

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/auth/sso"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/storage"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/session"
)

// LoggerHandler is a handler that will log the routing to the default gitea log
func LoggerHandler(level log.Level) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()

			_ = log.GetLogger("router").Log(0, level, "Started %s %s for %s", log.ColoredMethod(req.Method), req.URL.RequestURI(), req.RemoteAddr)

			next.ServeHTTP(w, req)

			var status int
			if v, ok := w.(context.ResponseWriter); ok {
				status = v.Status()
			}

			_ = log.GetLogger("router").Log(0, level, "Completed %s %s %v %s in %v", log.ColoredMethod(req.Method), req.URL.RequestURI(), log.ColoredStatus(status), log.ColoredStatus(status, http.StatusText(status)), log.ColoredTime(time.Since(start)))
		})
	}
}

func storageHandler(storageSetting setting.Storage, prefix string, objStore storage.ObjectStorage) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if storageSetting.ServeDirect {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != "GET" && req.Method != "HEAD" {
					next.ServeHTTP(w, req)
					return
				}

				if !strings.HasPrefix(req.URL.RequestURI(), "/"+prefix) {
					next.ServeHTTP(w, req)
					return
				}

				rPath := strings.TrimPrefix(req.URL.RequestURI(), "/"+prefix)
				u, err := objStore.URL(rPath, path.Base(rPath))
				if err != nil {
					if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
						log.Warn("Unable to find %s %s", prefix, rPath)
						http.Error(w, "file not found", 404)
						return
					}
					log.Error("Error whilst getting URL for %s %s. Error: %v", prefix, rPath, err)
					http.Error(w, fmt.Sprintf("Error whilst getting URL for %s %s", prefix, rPath), 500)
					return
				}
				http.Redirect(
					w,
					req,
					u.String(),
					301,
				)
			})
		}

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method != "GET" && req.Method != "HEAD" {
				next.ServeHTTP(w, req)
				return
			}

			prefix := strings.Trim(prefix, "/")

			if !strings.HasPrefix(req.URL.EscapedPath(), "/"+prefix+"/") {
				next.ServeHTTP(w, req)
				return
			}

			rPath := strings.TrimPrefix(req.URL.EscapedPath(), "/"+prefix+"/")
			rPath = strings.TrimPrefix(rPath, "/")
			if rPath == "" {
				http.Error(w, "file not found", 404)
				return
			}
			rPath = path.Clean("/" + filepath.ToSlash(rPath))
			rPath = rPath[1:]

			fi, err := objStore.Stat(rPath)
			if err == nil && httpcache.HandleTimeCache(req, w, fi) {
				return
			}

			// If we have matched and access to release or issue
			fr, err := objStore.Open(rPath)
			if err != nil {
				if os.IsNotExist(err) || errors.Is(err, os.ErrNotExist) {
					log.Warn("Unable to find %s %s", prefix, rPath)
					http.Error(w, "file not found", 404)
					return
				}
				log.Error("Error whilst opening %s %s. Error: %v", prefix, rPath, err)
				http.Error(w, fmt.Sprintf("Error whilst opening %s %s", prefix, rPath), 500)
				return
			}
			defer fr.Close()

			_, err = io.Copy(w, fr)
			if err != nil {
				log.Error("Error whilst rendering %s %s. Error: %v", prefix, rPath, err)
				http.Error(w, fmt.Sprintf("Error whilst rendering %s %s", prefix, rPath), 500)
				return
			}
		})
	}
}

type dataStore struct {
	Data map[string]interface{}
}

func (d *dataStore) GetData() map[string]interface{} {
	return d.Data
}

// Recovery returns a middleware that recovers from any panics and writes a 500 and a log if so.
// This error will be created with the gitea 500 page.
func Recovery() func(next http.Handler) http.Handler {
	rnd := templates.HTMLRenderer()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					combinedErr := fmt.Sprintf("PANIC: %v\n%s", err, string(log.Stack(2)))
					log.Error("%v", combinedErr)

					sessionStore := session.GetSession(req)
					if sessionStore == nil {
						if setting.IsProd() {
							http.Error(w, http.StatusText(500), 500)
						} else {
							http.Error(w, combinedErr, 500)
						}
						return
					}

					lc := middleware.Locale(w, req)
					store := dataStore{
						Data: templates.Vars{
							"Language":   lc.Language(),
							"CurrentURL": setting.AppSubURL + req.URL.RequestURI(),
							"i18n":       lc,
						},
					}

					// Get user from session if logged in.
					user, _ := sso.SignedInUser(req, w, &store, sessionStore)
					if user != nil {
						store.Data["IsSigned"] = true
						store.Data["SignedUser"] = user
						store.Data["SignedUserID"] = user.ID
						store.Data["SignedUserName"] = user.Name
						store.Data["IsAdmin"] = user.IsAdmin
					} else {
						store.Data["SignedUserID"] = int64(0)
						store.Data["SignedUserName"] = ""
					}

					w.Header().Set(`X-Frame-Options`, `SAMEORIGIN`)

					if !setting.IsProd() {
						store.Data["ErrorMsg"] = combinedErr
					}
					err = rnd.HTML(w, 500, "status/500", templates.BaseVars().Merge(store.Data))
					if err != nil {
						log.Error("%v", err)
					}
				}
			}()

			next.ServeHTTP(w, req)
		})
	}
}
