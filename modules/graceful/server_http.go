// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package graceful

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
)

func newHTTPServer(network, address, name string, handler http.Handler) (*Server, ServeFunction) {
	server := NewServer(network, address, name)
	httpServer := http.Server{
		ReadTimeout:    DefaultReadTimeOut,
		WriteTimeout:   DefaultWriteTimeOut,
		MaxHeaderBytes: DefaultMaxHeaderBytes,
		Handler:        handler,
		BaseContext:    func(net.Listener) context.Context { return GetManager().HammerContext() },
	}
	server.OnShutdown = func() {
		httpServer.SetKeepAlivesEnabled(false)
	}
	return server, httpServer.Serve
}

// HTTPListenAndServe listens on the provided network address and then calls Serve
// to handle requests on incoming connections.
func HTTPListenAndServe(network, address, name string, handler http.Handler, haProxy bool) error {
	server, lHandler := newHTTPServer(network, address, name, handler)
	return server.ListenAndServe(lHandler, haProxy)
}

// HTTPListenAndServeTLS listens on the provided network address and then calls Serve
// to handle requests on incoming connections.
func HTTPListenAndServeTLS(network, address, name, certFile, keyFile string, handler http.Handler, haProxy, haProxyTLSBridging bool) error {
	server, lHandler := newHTTPServer(network, address, name, handler)
	return server.ListenAndServeTLS(certFile, keyFile, lHandler, haProxy, haProxyTLSBridging)
}

// HTTPListenAndServeTLSConfig listens on the provided network address and then calls Serve
// to handle requests on incoming connections.
func HTTPListenAndServeTLSConfig(network, address, name string, tlsConfig *tls.Config, handler http.Handler, haProxy, haProxyTLSBridging bool) error {
	server, lHandler := newHTTPServer(network, address, name, handler)
	return server.ListenAndServeTLSConfig(tlsConfig, lHandler, haProxy, haProxyTLSBridging)
}
