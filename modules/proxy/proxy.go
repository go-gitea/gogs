// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package proxy

import (
	"net/http"
	"net/url"
	"os"
	"sync"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/gobwas/glob"
)

var (
	once         sync.Once
	hostMatchers []glob.Glob
)

// GetProxy returns proxy
func GetProxy() string {
	if !setting.Proxy.Enabled {
		return ""
	}

	if setting.Proxy.ProxyURL == "" {
		if os.Getenv("http_proxy") != "" {
			return os.Getenv("http_proxy")
		}
		return os.Getenv("https_proxy")
	}
	return setting.Proxy.ProxyURL
}

// Match return true if url needs to be proxied
func Match(u string) bool {
	if !setting.Proxy.Enabled {
		return false
	}

	// enforce do once
	SystemProxy()

	for _, v := range hostMatchers {
		if v.Match(u) {
			return true
		}
	}
	return false
}

// SystemProxy returns the system proxy
func SystemProxy() func(req *http.Request) (*url.URL, error) {
	if !setting.Proxy.Enabled {
		return nil
	}
	if setting.Proxy.ProxyURL == "" {
		return http.ProxyFromEnvironment
	}

	once.Do(func() {
		for _, h := range setting.Proxy.ProxyHosts {
			if g, err := glob.Compile(h); err == nil {
				hostMatchers = append(hostMatchers, g)
			} else {
				log.Error("glob.Compile %s failed: %v", h, err)
			}
		}
	})

	return func(req *http.Request) (*url.URL, error) {
		for _, v := range hostMatchers {
			if v.Match(req.URL.Host) {
				return http.ProxyURL(setting.Proxy.ProxyURLFixed)(req)
			}
		}
		return http.ProxyFromEnvironment(req)
	}
}
