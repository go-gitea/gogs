// Copyright 2015 Matthew Holt
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package certmagic

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/klauspost/cpuid/v2"
	"go.uber.org/zap"
	"golang.org/x/net/idna"
)

// encodePrivateKey marshals a EC or RSA private key into a PEM-encoded array of bytes.
func encodePrivateKey(key crypto.PrivateKey) ([]byte, error) {
	var pemType string
	var keyBytes []byte
	switch key := key.(type) {
	case *ecdsa.PrivateKey:
		var err error
		pemType = "EC"
		keyBytes, err = x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, err
		}
	case *rsa.PrivateKey:
		pemType = "RSA"
		keyBytes = x509.MarshalPKCS1PrivateKey(key)
	case ed25519.PrivateKey:
		var err error
		pemType = "ED25519"
		keyBytes, err = x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported key type: %T", key)
	}
	pemKey := pem.Block{Type: pemType + " PRIVATE KEY", Bytes: keyBytes}
	return pem.EncodeToMemory(&pemKey), nil
}

// decodePrivateKey loads a PEM-encoded ECC/RSA private key from an array of bytes.
// Borrowed from Go standard library, to handle various private key and PEM block types.
// https://github.com/golang/go/blob/693748e9fa385f1e2c3b91ca9acbb6c0ad2d133d/src/crypto/tls/tls.go#L291-L308
// https://github.com/golang/go/blob/693748e9fa385f1e2c3b91ca9acbb6c0ad2d133d/src/crypto/tls/tls.go#L238)
func decodePrivateKey(keyPEMBytes []byte) (crypto.Signer, error) {
	keyBlockDER, _ := pem.Decode(keyPEMBytes)

	if keyBlockDER.Type != "PRIVATE KEY" && !strings.HasSuffix(keyBlockDER.Type, " PRIVATE KEY") {
		return nil, fmt.Errorf("unknown PEM header %q", keyBlockDER.Type)
	}

	if key, err := x509.ParsePKCS1PrivateKey(keyBlockDER.Bytes); err == nil {
		return key, nil
	}

	if key, err := x509.ParsePKCS8PrivateKey(keyBlockDER.Bytes); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey, ed25519.PrivateKey:
			return key.(crypto.Signer), nil
		default:
			return nil, fmt.Errorf("found unknown private key type in PKCS#8 wrapping: %T", key)
		}
	}

	if key, err := x509.ParseECPrivateKey(keyBlockDER.Bytes); err == nil {
		return key, nil
	}

	return nil, fmt.Errorf("unknown private key type")
}

// parseCertsFromPEMBundle parses a certificate bundle from top to bottom and returns
// a slice of x509 certificates. This function will error if no certificates are found.
func parseCertsFromPEMBundle(bundle []byte) ([]*x509.Certificate, error) {
	var certificates []*x509.Certificate
	var certDERBlock *pem.Block
	for {
		certDERBlock, bundle = pem.Decode(bundle)
		if certDERBlock == nil {
			break
		}
		if certDERBlock.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(certDERBlock.Bytes)
			if err != nil {
				return nil, err
			}
			certificates = append(certificates, cert)
		}
	}
	if len(certificates) == 0 {
		return nil, fmt.Errorf("no certificates found in bundle")
	}
	return certificates, nil
}

// fastHash hashes input using a hashing algorithm that
// is fast, and returns the hash as a hex-encoded string.
// Do not use this for cryptographic purposes.
func fastHash(input []byte) string {
	h := fnv.New32a()
	h.Write(input)
	return fmt.Sprintf("%x", h.Sum32())
}

// saveCertResource saves the certificate resource to disk. This
// includes the certificate file itself, the private key, and the
// metadata file.
func (cfg *Config) saveCertResource(issuer Issuer, cert CertificateResource) error {
	metaBytes, err := json.MarshalIndent(cert, "", "\t")
	if err != nil {
		return fmt.Errorf("encoding certificate metadata: %v", err)
	}

	issuerKey := issuer.IssuerKey()
	certKey := cert.NamesKey()

	all := []keyValue{
		{
			key:   StorageKeys.SiteCert(issuerKey, certKey),
			value: cert.CertificatePEM,
		},
		{
			key:   StorageKeys.SitePrivateKey(issuerKey, certKey),
			value: cert.PrivateKeyPEM,
		},
		{
			key:   StorageKeys.SiteMeta(issuerKey, certKey),
			value: metaBytes,
		},
	}

	return storeTx(cfg.Storage, all)
}

// loadCertResourceAnyIssuer loads and returns the certificate resource from any
// of the configured issuers. If multiple are found (e.g. if there are 3 issuers
// configured, and all 3 have a resource matching certNamesKey), then the newest
// (latest NotBefore date) resource will be chosen.
func (cfg *Config) loadCertResourceAnyIssuer(certNamesKey string) (CertificateResource, error) {
	// we can save some extra decoding steps if there's only one issuer, since
	// we don't need to compare potentially multiple available resources to
	// select the best one, when there's only one choice anyway
	if len(cfg.Issuers) == 1 {
		return cfg.loadCertResource(cfg.Issuers[0], certNamesKey)
	}

	type decodedCertResource struct {
		CertificateResource
		issuer  Issuer
		decoded *x509.Certificate
	}
	var certResources []decodedCertResource
	var lastErr error

	// load and decode all certificate resources found with the
	// configured issuers so we can sort by newest
	for _, issuer := range cfg.Issuers {
		certRes, err := cfg.loadCertResource(issuer, certNamesKey)
		if err != nil {
			if _, ok := err.(ErrNotExist); ok {
				// not a problem, but we need to remember the error
				// in case we end up not finding any cert resources
				// since we'll need an error to return in that case
				lastErr = err
				continue
			}
			return CertificateResource{}, err
		}
		certs, err := parseCertsFromPEMBundle(certRes.CertificatePEM)
		if err != nil {
			return CertificateResource{}, err
		}
		certResources = append(certResources, decodedCertResource{
			CertificateResource: certRes,
			issuer:              issuer,
			decoded:             certs[0],
		})
	}
	if len(certResources) == 0 {
		if lastErr == nil {
			lastErr = fmt.Errorf("no certificate resources found") // just in case; e.g. no Issuers configured
		}
		return CertificateResource{}, lastErr
	}

	// sort by date so the most recently issued comes first
	sort.Slice(certResources, func(i, j int) bool {
		return certResources[j].decoded.NotBefore.Before(certResources[i].decoded.NotBefore)
	})

	if cfg.Logger != nil {
		cfg.Logger.Debug("loading managed certificate",
			zap.String("domain", certNamesKey),
			zap.Time("expiration", certResources[0].decoded.NotAfter),
			zap.String("issuer_key", certResources[0].issuer.IssuerKey()),
			zap.Any("storage", cfg.Storage),
		)
	}

	return certResources[0].CertificateResource, nil
}

// loadCertResource loads a certificate resource from the given issuer's storage location.
func (cfg *Config) loadCertResource(issuer Issuer, certNamesKey string) (CertificateResource, error) {
	certRes := CertificateResource{issuerKey: issuer.IssuerKey()}

	normalizedName, err := idna.ToASCII(certNamesKey)
	if err != nil {
		return CertificateResource{}, fmt.Errorf("converting '%s' to ASCII: %v", certNamesKey, err)
	}

	certBytes, err := cfg.Storage.Load(StorageKeys.SiteCert(certRes.issuerKey, normalizedName))
	if err != nil {
		return CertificateResource{}, err
	}
	certRes.CertificatePEM = certBytes
	keyBytes, err := cfg.Storage.Load(StorageKeys.SitePrivateKey(certRes.issuerKey, normalizedName))
	if err != nil {
		return CertificateResource{}, err
	}
	certRes.PrivateKeyPEM = keyBytes
	metaBytes, err := cfg.Storage.Load(StorageKeys.SiteMeta(certRes.issuerKey, normalizedName))
	if err != nil {
		return CertificateResource{}, err
	}
	err = json.Unmarshal(metaBytes, &certRes)
	if err != nil {
		return CertificateResource{}, fmt.Errorf("decoding certificate metadata: %v", err)
	}

	return certRes, nil
}

// hashCertificateChain computes the unique hash of certChain,
// which is the chain of DER-encoded bytes. It returns the
// hex encoding of the hash.
func hashCertificateChain(certChain [][]byte) string {
	h := sha256.New()
	for _, certInChain := range certChain {
		h.Write(certInChain)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func namesFromCSR(csr *x509.CertificateRequest) []string {
	var nameSet []string
	nameSet = append(nameSet, csr.DNSNames...)
	nameSet = append(nameSet, csr.EmailAddresses...)
	for _, v := range csr.IPAddresses {
		nameSet = append(nameSet, v.String())
	}
	for _, v := range csr.URIs {
		nameSet = append(nameSet, v.String())
	}
	return nameSet
}

// preferredDefaultCipherSuites returns an appropriate
// cipher suite to use depending on hardware support
// for AES-NI.
//
// See https://github.com/mholt/caddy/issues/1674
func preferredDefaultCipherSuites() []uint16 {
	if cpuid.CPU.Supports(cpuid.AESNI) {
		return defaultCiphersPreferAES
	}
	return defaultCiphersPreferChaCha
}

var (
	defaultCiphersPreferAES = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	}
	defaultCiphersPreferChaCha = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}
)

// StandardKeyGenerator is the standard, in-memory key source
// that uses crypto/rand.
type StandardKeyGenerator struct {
	// The type of keys to generate.
	KeyType KeyType
}

// GenerateKey generates a new private key according to kg.KeyType.
func (kg StandardKeyGenerator) GenerateKey() (crypto.PrivateKey, error) {
	switch kg.KeyType {
	case ED25519:
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		return priv, err
	case "", P256:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case P384:
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case RSA2048:
		return rsa.GenerateKey(rand.Reader, 2048)
	case RSA4096:
		return rsa.GenerateKey(rand.Reader, 4096)
	case RSA8192:
		return rsa.GenerateKey(rand.Reader, 8192)
	}
	return nil, fmt.Errorf("unrecognized or unsupported key type: %s", kg.KeyType)
}

// DefaultKeyGenerator is the default key source.
var DefaultKeyGenerator = StandardKeyGenerator{KeyType: P256}

// KeyType enumerates the known/supported key types.
type KeyType string

// Constants for all key types we support.
const (
	ED25519 = KeyType("ed25519")
	P256    = KeyType("p256")
	P384    = KeyType("p384")
	RSA2048 = KeyType("rsa2048")
	RSA4096 = KeyType("rsa4096")
	RSA8192 = KeyType("rsa8192")
)
