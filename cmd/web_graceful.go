// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package cmd

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/fcgi"
	"strings"

	"code.gitea.io/gitea/modules/graceful"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

func runHTTP(network, listenAddr, name string, m http.Handler, haProxy bool) error {
	return graceful.HTTPListenAndServe(network, listenAddr, name, m, haProxy)
}

func runHTTPS(network, listenAddr, name, certFile, keyFile string, m http.Handler, haProxy, haProxyTLSBridging bool) error {
	return graceful.HTTPListenAndServeTLS(network, listenAddr, name, certFile, keyFile, m, haProxy, haProxyTLSBridging)
}

func runHTTPSWithTLSConfig(network, listenAddr, name string, tlsConfig *tls.Config, m http.Handler, haProxy, haProxyTLSBridging bool) error {
	return graceful.HTTPListenAndServeTLSConfig(network, listenAddr, name, tlsConfig, m, haProxy, haProxyTLSBridging)
}

// NoHTTPRedirector tells our cleanup routine that we will not be using a fallback http redirector
func NoHTTPRedirector() {
	graceful.GetManager().InformCleanup()
}

// NoMainListener tells our cleanup routine that we will not be using a possibly provided listener
// for our main HTTP/HTTPS service
func NoMainListener() {
	graceful.GetManager().InformCleanup()
}

// NoInstallListener tells our cleanup routine that we will not be using a possibly provided listener
// for our install HTTP/HTTPS service
func NoInstallListener() {
	graceful.GetManager().InformCleanup()
}

func runFCGI(network, listenAddr, name string, m http.Handler, haProxy bool) error {
	// This needs to handle stdin as fcgi point
	fcgiServer := graceful.NewServer(network, listenAddr, name)

	err := fcgiServer.ListenAndServe(func(listener net.Listener) error {
		return fcgi.Serve(listener, http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			if setting.AppSubURL != "" {
				req.URL.Path = strings.TrimPrefix(req.URL.Path, setting.AppSubURL)
			}
			m.ServeHTTP(resp, req)
		}))
	}, haProxy)
	if err != nil {
		log.Fatal("Failed to start FCGI main server: %v", err)
	}
	log.Info("FCGI Listener: %s Closed", listenAddr)
	return err
}
