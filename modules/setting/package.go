// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"crypto/tls"
	"crypto/x509"
	"strings"

	"code.gitea.io/gitea/modules/log"

	"github.com/docker/libtrust"
)

// settings
type dockerPluginConfig struct {
	ServiceName string
	IssuerName  string
	Expiration  int64
	NotifyToken string

	PublicKey  libtrust.PublicKey
	PrivateKey libtrust.PrivateKey

	APIBasePath string
}

var (
	// Docker docker plugin config
	Docker *dockerPluginConfig
)

func newPackage() {
	cfg := Cfg.Section("package.docker_registry_plugin")
	if cfg == nil || !cfg.Key("ENABLED").MustBool(false) {
		Repository.DisabledRepoUnits = append(Repository.DisabledRepoUnits, "repo.packages")
		return
	}

	Docker = new(dockerPluginConfig)
	Docker.Expiration = cfg.Key("EXPIRATION").RangeInt64(60, 60, 3600)

	certFile := cfg.Key("CERT_FILE").String()
	if len(certFile) == 0 {
		log.Fatal("newPackage.docker_registry_plugin: `CERT_FILE` is requested")
	}
	keyFile := cfg.Key("KEY_FILE").String()
	if len(keyFile) == 0 {
		log.Fatal("newPackage.docker_registry_plugin: `KeyFile` is requested")
	}
	var err error
	Docker.PublicKey, Docker.PrivateKey, err = loadCertAndKey(certFile, keyFile)
	if err != nil {
		log.Fatal("docker_registry_plugin: loadCertAndKey: %V", err)
	}

	Docker.IssuerName = cfg.Key("ISSUER_NAME").MustString("gitea")
	Docker.ServiceName = cfg.Key("SERVICE_NAME").MustString("gitea-token-service")
	Docker.NotifyToken = cfg.Key("NOTIFY_TOKEN").String()
	if len(Docker.NotifyToken) == 0 {
		log.Fatal("docker_registry_plugin: `NOTIFY_TOKEN` is requested")
	}
	Docker.APIBasePath = cfg.Key("API_BASE_PATH").String()
	if len(Docker.APIBasePath) == 0 {
		log.Fatal("docker_registry_plugin: `API_BASE_PATH` is requested")
	}
	Docker.APIBasePath = strings.TrimRight(Docker.APIBasePath, "/")
}

// HasDockerPlugin has docker plugin
func HasDockerPlugin() bool {
	return Docker != nil
}

func loadCertAndKey(certFile, keyFile string) (pk libtrust.PublicKey, prk libtrust.PrivateKey, err error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return
	}
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return
	}
	pk, err = libtrust.FromCryptoPublicKey(x509Cert.PublicKey)
	if err != nil {
		return
	}
	prk, err = libtrust.FromCryptoPrivateKey(cert.PrivateKey)
	return
}
