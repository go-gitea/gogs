// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"crypto"
	"crypto/tls"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
)

// Docker docker plugin config
var Docker struct {
	Enabled     bool
	ServiceName string
	IssuerName  string
	Expiration  int64

	PrivateKey crypto.PrivateKey
}

func newPackages() {
	cfg := Cfg.Section("package.container_registry")

	if Docker.Enabled = cfg.Key("ENABLED").MustBool(false); Docker.Enabled {

		keyFile := cfg.Key("KEY_FILE").String()
		if len(keyFile) == 0 {
			log.Fatal("newContainerRegistry: `KeyFile` is requested")
		} else if !filepath.IsAbs(keyFile) {
			keyFile = filepath.Join(CustomPath, keyFile)
		}

		certFile := cfg.Key("CERT_FILE").String()
		if len(certFile) == 0 {
			log.Fatal("newContainerRegistry: `certFile` is requested")
		} else if !filepath.IsAbs(certFile) {
			certFile = filepath.Join(CustomPath, certFile)
		}

		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			log.Fatal("newContainerRegistry: loadCertAndKey: %v", err)
		}
		Docker.PrivateKey = cert.PrivateKey

		Docker.Expiration = cfg.Key("EXPIRATION").RangeInt64(60, 60, 3600)
		Docker.IssuerName = cfg.Key("ISSUER_NAME").MustString("gitea")
		Docker.ServiceName = cfg.Key("SERVICE_NAME").MustString("gitea-token-service")

		log.Info("Container Registry Enabled")
	}
}
