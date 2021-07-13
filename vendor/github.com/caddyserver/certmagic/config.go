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
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"errors"
	"fmt"
	weakrand "math/rand"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/mholt/acmez"
	"github.com/mholt/acmez/acme"
	"go.uber.org/zap"
	"golang.org/x/net/idna"
)

// Config configures a certificate manager instance.
// An empty Config is not valid: use New() to obtain
// a valid Config.
type Config struct {
	// How much of a certificate's lifetime becomes the
	// renewal window, which is the span of time at the
	// end of the certificate's validity period in which
	// it should be renewed; for most certificates, the
	// global default is good, but for extremely short-
	// lived certs, you may want to raise this to ~0.5.
	RenewalWindowRatio float64

	// An optional event callback clients can set
	// to subscribe to certain things happening
	// internally by this config; invocations are
	// synchronous, so make them return quickly!
	OnEvent func(event string, data interface{})

	// DefaultServerName specifies a server name
	// to use when choosing a certificate if the
	// ClientHello's ServerName field is empty.
	DefaultServerName string

	// The state needed to operate on-demand TLS;
	// if non-nil, on-demand TLS is enabled and
	// certificate operations are deferred to
	// TLS handshakes (or as-needed).
	// TODO: Can we call this feature "Reactive/Lazy/Passive TLS" instead?
	OnDemand *OnDemandConfig

	// Adds the must staple TLS extension to the CSR.
	MustStaple bool

	// The source for getting new certificates; the
	// default Issuer is ACMEManager. If multiple
	// issuers are specified, they will be tried in
	// turn until one succeeds.
	Issuers []Issuer

	// The source of new private keys for certificates;
	// the default KeySource is StandardKeyGenerator.
	KeySource KeyGenerator

	// CertSelection chooses one of the certificates
	// with which the ClientHello will be completed;
	// if not set, DefaultCertificateSelector will
	// be used.
	CertSelection CertificateSelector

	// OCSP configures how OCSP is handled. By default,
	// OCSP responses are fetched for every certificate
	// with a responder URL, and cached on disk. Changing
	// these defaults is STRONGLY discouraged unless you
	// have a compelling reason to put clients at greater
	// risk and reduce their privacy.
	OCSP OCSPConfig

	// The storage to access when storing or loading
	// TLS assets. Default is the local file system.
	Storage Storage

	// Set a logger to enable logging.
	Logger *zap.Logger

	// required pointer to the in-memory cert cache
	certCache *Cache
}

// NewDefault makes a valid config based on the package
// Default config. Most users will call this function
// instead of New() since most use cases require only a
// single config for any and all certificates.
//
// If your requirements are more advanced (for example,
// multiple configs depending on the certificate), then use
// New() instead. (You will need to make your own Cache
// first.) If you only need a single Config to manage your
// certs (even if that config changes, as long as it is the
// only one), customize the Default package variable before
// calling NewDefault().
//
// All calls to NewDefault() will return configs that use the
// same, default certificate cache. All configs returned
// by NewDefault() are based on the values of the fields of
// Default at the time it is called.
//
// This is the only way to get a config that uses the
// default certificate cache.
func NewDefault() *Config {
	defaultCacheMu.Lock()
	if defaultCache == nil {
		defaultCache = NewCache(CacheOptions{
			// the cache will likely need to renew certificates,
			// so it will need to know how to do that, which
			// depends on the certificate being managed and which
			// can change during the lifetime of the cache; this
			// callback makes it possible to get the latest and
			// correct config with which to manage the cert,
			// but if the user does not provide one, we can only
			// assume that we are to use the default config
			GetConfigForCert: func(Certificate) (*Config, error) {
				return NewDefault(), nil
			},
		})
	}
	certCache := defaultCache
	defaultCacheMu.Unlock()

	return newWithCache(certCache, Default)
}

// New makes a new, valid config based on cfg and
// uses the provided certificate cache. certCache
// MUST NOT be nil or this function will panic.
//
// Use this method when you have an advanced use case
// that requires a custom certificate cache and config
// that may differ from the Default. For example, if
// not all certificates are managed/renewed the same
// way, you need to make your own Cache value with a
// GetConfigForCert callback that returns the correct
// configuration for each certificate. However, for
// the vast majority of cases, there will be only a
// single Config, thus the default cache (which always
// uses the default Config) and default config will
// suffice, and you should use NewDefault() instead.
func New(certCache *Cache, cfg Config) *Config {
	if certCache == nil {
		panic("a certificate cache is required")
	}
	if certCache.options.GetConfigForCert == nil {
		panic("cache must have GetConfigForCert set in its options")
	}
	return newWithCache(certCache, cfg)
}

// newWithCache ensures that cfg is a valid config by populating
// zero-value fields from the Default Config. If certCache is
// nil, this function panics.
func newWithCache(certCache *Cache, cfg Config) *Config {
	if certCache == nil {
		panic("cannot make a valid config without a pointer to a certificate cache")
	}

	if cfg.OnDemand == nil {
		cfg.OnDemand = Default.OnDemand
	}
	if cfg.RenewalWindowRatio == 0 {
		cfg.RenewalWindowRatio = Default.RenewalWindowRatio
	}
	if cfg.OnEvent == nil {
		cfg.OnEvent = Default.OnEvent
	}
	if cfg.KeySource == nil {
		cfg.KeySource = Default.KeySource
	}
	if cfg.DefaultServerName == "" {
		cfg.DefaultServerName = Default.DefaultServerName
	}
	if cfg.OnDemand == nil {
		cfg.OnDemand = Default.OnDemand
	}
	if !cfg.MustStaple {
		cfg.MustStaple = Default.MustStaple
	}
	if cfg.Storage == nil {
		cfg.Storage = Default.Storage
	}
	if len(cfg.Issuers) == 0 {
		cfg.Issuers = Default.Issuers
		if len(cfg.Issuers) == 0 {
			// at least one issuer is absolutely required
			cfg.Issuers = []Issuer{NewACMEManager(&cfg, DefaultACME)}
		}
	}

	// absolutely don't allow a nil storage,
	// because that would make almost anything
	// a config can do pointless
	if cfg.Storage == nil {
		cfg.Storage = defaultFileStorage
	}

	cfg.certCache = certCache

	return &cfg
}

// ManageSync causes the certificates for domainNames to be managed
// according to cfg. If cfg.OnDemand is not nil, then this simply
// whitelists the domain names and defers the certificate operations
// to when they are needed. Otherwise, the certificates for each
// name are loaded from storage or obtained from the CA. If loaded
// from storage, they are renewed if they are expiring or expired.
// It then caches the certificate in memory and is prepared to serve
// them up during TLS handshakes.
//
// Note that name whitelisting for on-demand management only takes
// effect if cfg.OnDemand.DecisionFunc is not set (is nil); it will
// not overwrite an existing DecisionFunc, nor will it overwrite
// its decision; i.e. the implicit whitelist is only used if no
// DecisionFunc is set.
//
// This method is synchronous, meaning that certificates for all
// domainNames must be successfully obtained (or renewed) before
// it returns. It returns immediately on the first error for any
// of the given domainNames. This behavior is recommended for
// interactive use (i.e. when an administrator is present) so
// that errors can be reported and fixed immediately.
func (cfg *Config) ManageSync(domainNames []string) error {
	return cfg.manageAll(context.Background(), domainNames, false)
}

// ClientCredentials returns a list of TLS client certificate chains for the given identifiers.
// The return value can be used in a tls.Config to enable client authentication using managed certificates.
// Any certificates that need to be obtained or renewed for these identifiers will be managed accordingly.
func (cfg *Config) ClientCredentials(ctx context.Context, identifiers []string) ([]tls.Certificate, error) {
	err := cfg.manageAll(ctx, identifiers, false)
	if err != nil {
		return nil, err
	}
	var chains []tls.Certificate
	for _, id := range identifiers {
		certRes, err := cfg.loadCertResourceAnyIssuer(id)
		if err != nil {
			return chains, err
		}
		chain, err := tls.X509KeyPair(certRes.CertificatePEM, certRes.PrivateKeyPEM)
		if err != nil {
			return chains, err
		}
		chains = append(chains, chain)
	}
	return chains, nil
}

// ManageAsync is the same as ManageSync, except that ACME
// operations are performed asynchronously (in the background).
// This method returns before certificates are ready. It is
// crucial that the administrator monitors the logs and is
// notified of any errors so that corrective action can be
// taken as soon as possible. Any errors returned from this
// method occurred before ACME transactions started.
//
// As long as logs are monitored, this method is typically
// recommended for non-interactive environments.
//
// If there are failures loading, obtaining, or renewing a
// certificate, it will be retried with exponential backoff
// for up to about 30 days, with a maximum interval of about
// 24 hours. Cancelling ctx will cancel retries and shut down
// any goroutines spawned by ManageAsync.
func (cfg *Config) ManageAsync(ctx context.Context, domainNames []string) error {
	return cfg.manageAll(ctx, domainNames, true)
}

func (cfg *Config) manageAll(ctx context.Context, domainNames []string, async bool) error {
	if ctx == nil {
		ctx = context.Background()
	}

	for _, domainName := range domainNames {
		// if on-demand is configured, defer obtain and renew operations
		if cfg.OnDemand != nil {
			if !cfg.OnDemand.whitelistContains(domainName) {
				cfg.OnDemand.hostWhitelist = append(cfg.OnDemand.hostWhitelist, domainName)
			}
			continue
		}

		// otherwise, begin management immediately
		err := cfg.manageOne(ctx, domainName, async)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cfg *Config) manageOne(ctx context.Context, domainName string, async bool) error {
	// first try loading existing certificate from storage
	cert, err := cfg.CacheManagedCertificate(domainName)
	if err != nil {
		if _, ok := err.(ErrNotExist); !ok {
			return fmt.Errorf("%s: caching certificate: %v", domainName, err)
		}
		// if we don't have one in storage, obtain one
		obtain := func() error {
			var err error
			if async {
				err = cfg.ObtainCertAsync(ctx, domainName)
			} else {
				err = cfg.ObtainCertSync(ctx, domainName)
			}
			if err != nil {
				return fmt.Errorf("%s: obtaining certificate: %w", domainName, err)
			}
			cert, err = cfg.CacheManagedCertificate(domainName)
			if err != nil {
				return fmt.Errorf("%s: caching certificate after obtaining it: %v", domainName, err)
			}
			return nil
		}
		if async {
			// Leave the job name empty so as to allow duplicate 'obtain'
			// jobs; this is because Caddy calls ManageAsync() before the
			// previous config is stopped (and before its context is
			// canceled), which means that if an obtain job is still
			// running for the same domain, Submit() would not queue the
			// new one because it is still running, even though it is
			// (probably) about to be canceled (it might not if the new
			// config fails to finish loading, however). In any case, we
			// presume it is safe to enqueue a duplicate obtain job because
			// either the old one (or sometimes the new one) is about to be
			// canceled. This seems like reasonable logic for any consumer
			// of this lib. See https://github.com/caddyserver/caddy/issues/3202
			jm.Submit(cfg.Logger, "", obtain)
			return nil
		}
		return obtain()
	}

	// for an existing certificate, make sure it is renewed
	renew := func() error {
		var err error
		if async {
			err = cfg.RenewCertAsync(ctx, domainName, false)
		} else {
			err = cfg.RenewCertSync(ctx, domainName, false)
		}
		if err != nil {
			return fmt.Errorf("%s: renewing certificate: %w", domainName, err)
		}
		// successful renewal, so update in-memory cache
		err = cfg.reloadManagedCertificate(cert)
		if err != nil {
			return fmt.Errorf("%s: reloading renewed certificate into memory: %v", domainName, err)
		}
		return nil
	}
	if cert.NeedsRenewal(cfg) {
		if async {
			jm.Submit(cfg.Logger, "renew_"+domainName, renew)
			return nil
		}
		return renew()
	}

	return nil
}

// Unmanage causes the certificates for domainNames to stop being managed.
// If there are certificates for the supplied domain names in the cache, they
// are evicted from the cache.
func (cfg *Config) Unmanage(domainNames []string) {
	var deleteQueue []Certificate
	for _, domainName := range domainNames {
		certs := cfg.certCache.AllMatchingCertificates(domainName)
		for _, cert := range certs {
			if !cert.managed {
				continue
			}
			deleteQueue = append(deleteQueue, cert)
		}
	}

	cfg.certCache.mu.Lock()
	for _, cert := range deleteQueue {
		cfg.certCache.removeCertificate(cert)
	}
	cfg.certCache.mu.Unlock()
}

// ObtainCertSync generates a new private key and obtains a certificate for
// name using cfg in the foreground; i.e. interactively and without retries.
// It stows the renewed certificate and its assets in storage if successful.
// It DOES NOT load the certificate into the in-memory cache. This method
// is a no-op if storage already has a certificate for name.
func (cfg *Config) ObtainCertSync(ctx context.Context, name string) error {
	return cfg.obtainCert(ctx, name, true)
}

// ObtainCertAsync is the same as ObtainCertSync(), except it runs in the
// background; i.e. non-interactively, and with retries if it fails.
func (cfg *Config) ObtainCertAsync(ctx context.Context, name string) error {
	return cfg.obtainCert(ctx, name, false)
}

func (cfg *Config) obtainCert(ctx context.Context, name string, interactive bool) error {
	if len(cfg.Issuers) == 0 {
		return fmt.Errorf("no issuers configured; impossible to obtain or check for existing certificate in storage")
	}

	// if storage has all resources for this certificate, obtain is a no-op
	if cfg.storageHasCertResourcesAnyIssuer(name) {
		return nil
	}

	// ensure storage is writeable and readable
	// TODO: this is not necessary every time; should only perform check once every so often for each storage, which may require some global state...
	err := cfg.checkStorage()
	if err != nil {
		return fmt.Errorf("failed storage check: %v - storage is probably misconfigured", err)
	}

	log := loggerNamed(cfg.Logger, "obtain")

	if log != nil {
		log.Info("acquiring lock", zap.String("identifier", name))
	}

	// ensure idempotency of the obtain operation for this name
	lockKey := cfg.lockKey(certIssueLockOp, name)
	err = acquireLock(ctx, cfg.Storage, lockKey)
	if err != nil {
		return fmt.Errorf("unable to acquire lock '%s': %v", lockKey, err)
	}
	defer func() {
		if log != nil {
			log.Info("releasing lock", zap.String("identifier", name))
		}
		if err := releaseLock(cfg.Storage, lockKey); err != nil {
			if log != nil {
				log.Error("unable to unlock",
					zap.String("identifier", name),
					zap.String("lock_key", lockKey),
					zap.Error(err))
			}
		}
	}()
	if log != nil {
		log.Info("lock acquired", zap.String("identifier", name))
	}

	f := func(ctx context.Context) error {
		// check if obtain is still needed -- might have been obtained during lock
		if cfg.storageHasCertResourcesAnyIssuer(name) {
			if log != nil {
				log.Info("certificate already exists in storage", zap.String("identifier", name))
			}
			return nil
		}

		// if storage has a private key already, use it; otherwise,
		// we'll generate our own
		privKey, privKeyPEM, issuers, err := cfg.reusePrivateKey(name)
		if err != nil {
			return err
		}
		if privKey == nil {
			privKey, err = cfg.KeySource.GenerateKey()
			if err != nil {
				return err
			}
			privKeyPEM, err = encodePrivateKey(privKey)
			if err != nil {
				return err
			}
		}

		csr, err := cfg.generateCSR(privKey, []string{name})
		if err != nil {
			return err
		}

		// try to obtain from each issuer until we succeed
		var issuedCert *IssuedCertificate
		var issuerUsed Issuer
		for i, issuer := range issuers {
			log.Debug(fmt.Sprintf("trying issuer %d/%d", i+1, len(cfg.Issuers)),
				zap.String("issuer", issuer.IssuerKey()))

			if prechecker, ok := issuer.(PreChecker); ok {
				err = prechecker.PreCheck(ctx, []string{name}, interactive)
				if err != nil {
					continue
				}
			}

			issuedCert, err = issuer.Issue(ctx, csr)
			if err == nil {
				issuerUsed = issuer
				break
			}

			// err is usually wrapped, which is nice for simply printing it, but
			// with our structured error logs we only need the problem string
			errToLog := err
			var problem acme.Problem
			if errors.As(err, &problem) {
				errToLog = problem
			}
			log.Error("could not get certificate from issuer",
				zap.String("identifier", name),
				zap.String("issuer", issuer.IssuerKey()),
				zap.Error(errToLog))
		}
		if err != nil {
			// only the error from the last issuer will be returned, but we logged the others
			return fmt.Errorf("[%s] Obtain: %w", name, err)
		}

		// success - immediately save the certificate resource
		certRes := CertificateResource{
			SANs:           namesFromCSR(csr),
			CertificatePEM: issuedCert.Certificate,
			PrivateKeyPEM:  privKeyPEM,
			IssuerData:     issuedCert.Metadata,
		}
		err = cfg.saveCertResource(issuerUsed, certRes)
		if err != nil {
			return fmt.Errorf("[%s] Obtain: saving assets: %v", name, err)
		}

		cfg.emit("cert_obtained", name)

		if log != nil {
			log.Info("certificate obtained successfully", zap.String("identifier", name))
		}

		return nil
	}

	if interactive {
		err = f(ctx)
	} else {
		err = doWithRetry(ctx, log, f)
	}

	return err
}

// reusePrivateKey looks for a private key for domain in storage in the configured issuers
// paths. For the first private key it finds, it returns that key both decoded and PEM-encoded,
// as well as the reordered list of issuers to use instead of cfg.Issuers (because if a key
// is found, that issuer should be tried first, so it is moved to the front in a copy of
// cfg.Issuers).
func (cfg *Config) reusePrivateKey(domain string) (privKey crypto.PrivateKey, privKeyPEM []byte, issuers []Issuer, err error) {
	// make a copy of cfg.Issuers so that if we have to reorder elements, we don't
	// inadvertently mutate the configured issuers (see append calls below)
	issuers = make([]Issuer, len(cfg.Issuers))
	copy(issuers, cfg.Issuers)

	for i, issuer := range issuers {
		// see if this issuer location in storage has a private key for the domain
		privateKeyStorageKey := StorageKeys.SitePrivateKey(issuer.IssuerKey(), domain)
		privKeyPEM, err = cfg.Storage.Load(privateKeyStorageKey)
		if _, ok := err.(ErrNotExist); ok {
			err = nil // obviously, it's OK to not have a private key; so don't prevent obtaining a cert
			continue
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("loading existing private key for reuse with issuer %s: %v", issuer.IssuerKey(), err)
		}

		// we loaded a private key; try decoding it so we can use it
		privKey, err = decodePrivateKey(privKeyPEM)
		if err != nil {
			return nil, nil, nil, err
		}

		// since the private key was found in storage for this issuer, move it
		// to the front of the list so we prefer this issuer first
		issuers = append([]Issuer{issuer}, append(issuers[:i], issuers[i+1:]...)...)
		break
	}

	return
}

// storageHasCertResourcesAnyIssuer returns true if storage has all the
// certificate resources in storage from any configured issuer. It checks
// all configured issuers in order.
func (cfg *Config) storageHasCertResourcesAnyIssuer(name string) bool {
	for _, iss := range cfg.Issuers {
		if cfg.storageHasCertResources(iss, name) {
			return true
		}
	}
	return false
}

// RenewCertSync renews the certificate for name using cfg in the foreground;
// i.e. interactively and without retries. It stows the renewed certificate
// and its assets in storage if successful. It DOES NOT update the in-memory
// cache with the new certificate. The certificate will not be renewed if it
// is not close to expiring unless force is true.
//
// Renewing a certificate is the same as obtaining a certificate, except that
// the existing private key already in storage is reused.
func (cfg *Config) RenewCertSync(ctx context.Context, name string, force bool) error {
	return cfg.renewCert(ctx, name, force, true)
}

// RenewCertAsync is the same as RenewCertSync(), except it runs in the
// background; i.e. non-interactively, and with retries if it fails.
func (cfg *Config) RenewCertAsync(ctx context.Context, name string, force bool) error {
	return cfg.renewCert(ctx, name, force, false)
}

func (cfg *Config) renewCert(ctx context.Context, name string, force, interactive bool) error {
	if len(cfg.Issuers) == 0 {
		return fmt.Errorf("no issuers configured; impossible to renew or check existing certificate in storage")
	}

	// ensure storage is writeable and readable
	// TODO: this is not necessary every time; should only perform check once every so often for each storage, which may require some global state...
	err := cfg.checkStorage()
	if err != nil {
		return fmt.Errorf("failed storage check: %v - storage is probably misconfigured", err)
	}

	log := loggerNamed(cfg.Logger, "renew")

	if log != nil {
		log.Info("acquiring lock", zap.String("identifier", name))
	}

	// ensure idempotency of the renew operation for this name
	lockKey := cfg.lockKey(certIssueLockOp, name)
	err = acquireLock(ctx, cfg.Storage, lockKey)
	if err != nil {
		return fmt.Errorf("unable to acquire lock '%s': %v", lockKey, err)
	}
	defer func() {
		if log != nil {
			log.Info("releasing lock", zap.String("identifier", name))
		}
		if err := releaseLock(cfg.Storage, lockKey); err != nil {
			if log != nil {
				log.Error("unable to unlock",
					zap.String("identifier", name),
					zap.String("lock_key", lockKey),
					zap.Error(err))
			}
		}
	}()
	if log != nil {
		log.Info("lock acquired", zap.String("identifier", name))
	}

	f := func(ctx context.Context) error {
		// prepare for renewal (load PEM cert, key, and meta)
		certRes, err := cfg.loadCertResourceAnyIssuer(name)
		if err != nil {
			return err
		}

		// check if renew is still needed - might have been renewed while waiting for lock
		timeLeft, needsRenew := cfg.managedCertNeedsRenewal(certRes)
		if !needsRenew {
			if force {
				if log != nil {
					log.Info("certificate does not need to be renewed, but renewal is being forced",
						zap.String("identifier", name),
						zap.Duration("remaining", timeLeft))
				}
			} else {
				if log != nil {
					log.Info("certificate appears to have been renewed already",
						zap.String("identifier", name),
						zap.Duration("remaining", timeLeft))
				}
				return nil
			}
		}

		if log != nil {
			log.Info("renewing certificate",
				zap.String("identifier", name),
				zap.Duration("remaining", timeLeft))
		}

		privateKey, err := decodePrivateKey(certRes.PrivateKeyPEM)
		if err != nil {
			return err
		}
		csr, err := cfg.generateCSR(privateKey, []string{name})
		if err != nil {
			return err
		}

		// try to obtain from each issuer until we succeed
		var issuedCert *IssuedCertificate
		var issuerUsed Issuer
		for _, issuer := range cfg.Issuers {
			if prechecker, ok := issuer.(PreChecker); ok {
				err = prechecker.PreCheck(ctx, []string{name}, interactive)
				if err != nil {
					continue
				}
			}

			issuedCert, err = issuer.Issue(ctx, csr)
			if err == nil {
				issuerUsed = issuer
				break
			}

			// err is usually wrapped, which is nice for simply printing it, but
			// with our structured error logs we only need the problem string
			errToLog := err
			var problem acme.Problem
			if errors.As(err, &problem) {
				errToLog = problem
			}
			log.Error("could not get certificate from issuer",
				zap.String("identifier", name),
				zap.String("issuer", issuer.IssuerKey()),
				zap.Error(errToLog))
		}
		if err != nil {
			// only the error from the last issuer will be returned, but we logged the others
			return fmt.Errorf("[%s] Renew: %w", name, err)
		}

		// success - immediately save the renewed certificate resource
		newCertRes := CertificateResource{
			SANs:           namesFromCSR(csr),
			CertificatePEM: issuedCert.Certificate,
			PrivateKeyPEM:  certRes.PrivateKeyPEM,
			IssuerData:     issuedCert.Metadata,
		}
		err = cfg.saveCertResource(issuerUsed, newCertRes)
		if err != nil {
			return fmt.Errorf("[%s] Renew: saving assets: %v", name, err)
		}

		cfg.emit("cert_renewed", name)

		if log != nil {
			log.Info("certificate renewed successfully", zap.String("identifier", name))
		}

		return nil
	}

	if interactive {
		err = f(ctx)
	} else {
		err = doWithRetry(ctx, log, f)
	}

	return err
}

func (cfg *Config) generateCSR(privateKey crypto.PrivateKey, sans []string) (*x509.CertificateRequest, error) {
	csrTemplate := new(x509.CertificateRequest)

	for _, name := range sans {
		if ip := net.ParseIP(name); ip != nil {
			csrTemplate.IPAddresses = append(csrTemplate.IPAddresses, ip)
		} else if strings.Contains(name, "@") {
			csrTemplate.EmailAddresses = append(csrTemplate.EmailAddresses, name)
		} else if u, err := url.Parse(name); err == nil && strings.Contains(name, "/") {
			csrTemplate.URIs = append(csrTemplate.URIs, u)
		} else {
			// convert IDNs to ASCII according to RFC 5280 section 7
			normalizedName, err := idna.ToASCII(name)
			if err != nil {
				return nil, fmt.Errorf("converting identifier '%s' to ASCII: %v", name, err)
			}
			csrTemplate.DNSNames = append(csrTemplate.DNSNames, normalizedName)
		}
	}

	if cfg.MustStaple {
		csrTemplate.ExtraExtensions = append(csrTemplate.ExtraExtensions, mustStapleExtension)
	}

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, csrTemplate, privateKey)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificateRequest(csrDER)
}

// RevokeCert revokes the certificate for domain via ACME protocol. It requires
// that cfg.Issuers is properly configured with the same issuer that issued the
// certificate being revoked. See RFC 5280 §5.3.1 for reason codes.
//
// The certificate assets are deleted from storage after successful revocation
// to prevent reuse.
func (cfg *Config) RevokeCert(ctx context.Context, domain string, reason int, interactive bool) error {
	for i, issuer := range cfg.Issuers {
		issuerKey := issuer.IssuerKey()

		rev, ok := issuer.(Revoker)
		if !ok {
			return fmt.Errorf("issuer %d (%s) is not a Revoker", i, issuerKey)
		}

		certRes, err := cfg.loadCertResource(issuer, domain)
		if err != nil {
			return err
		}

		if !cfg.Storage.Exists(StorageKeys.SitePrivateKey(issuerKey, domain)) {
			return fmt.Errorf("private key not found for %s", certRes.SANs)
		}

		err = rev.Revoke(ctx, certRes, reason)
		if err != nil {
			return fmt.Errorf("issuer %d (%s): %v", i, issuerKey, err)
		}

		cfg.emit("cert_revoked", domain)

		err = cfg.deleteSiteAssets(issuerKey, domain)
		if err != nil {
			return fmt.Errorf("certificate revoked, but unable to fully clean up assets from issuer %s: %v", issuerKey, err)
		}
	}

	return nil
}

// TLSConfig is an opinionated method that returns a
// recommended, modern TLS configuration that can be
// used to configure TLS listeners, which also supports
// the TLS-ALPN challenge and serves up certificates
// managed by cfg.
//
// Unlike the package TLS() function, this method does
// not, by itself, enable certificate management for
// any domain names.
//
// Feel free to further customize the returned tls.Config,
// but do not mess with the GetCertificate or NextProtos
// fields unless you know what you're doing, as they're
// necessary to solve the TLS-ALPN challenge.
func (cfg *Config) TLSConfig() *tls.Config {
	return &tls.Config{
		// these two fields necessary for TLS-ALPN challenge
		GetCertificate: cfg.GetCertificate,
		NextProtos:     []string{acmez.ACMETLS1Protocol},

		// the rest recommended for modern TLS servers
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
		CipherSuites:             preferredDefaultCipherSuites(),
		PreferServerCipherSuites: true,
	}
}

// getChallengeInfo loads the challenge info from either the internal challenge memory
// or the external storage (implying distributed solving). The second return value
// indicates whether challenge info was loaded from external storage. If true, the
// challenge is being solved in a distributed fashion; if false, from internal memory.
// If no matching challenge information can be found, an error is returned.
func (cfg *Config) getChallengeInfo(identifier string) (Challenge, bool, error) {
	// first, check if our process initiated this challenge; if so, just return it
	chalData, ok := GetACMEChallenge(identifier)
	if ok {
		return chalData, false, nil
	}

	// otherwise, perhaps another instance in the cluster initiated it; check
	// the configured storage to retrieve challenge data

	var chalInfo acme.Challenge
	var chalInfoBytes []byte
	var tokenKey string
	for _, issuer := range cfg.Issuers {
		ds := distributedSolver{
			storage:                cfg.Storage,
			storageKeyIssuerPrefix: storageKeyACMECAPrefix(issuer.IssuerKey()),
		}
		tokenKey = ds.challengeTokensKey(identifier)
		var err error
		chalInfoBytes, err = cfg.Storage.Load(tokenKey)
		if err == nil {
			break
		}
		if _, ok := err.(ErrNotExist); ok {
			continue
		}
		return Challenge{}, false, fmt.Errorf("opening distributed challenge token file %s: %v", tokenKey, err)
	}
	if len(chalInfoBytes) == 0 {
		return Challenge{}, false, fmt.Errorf("no information found to solve challenge for identifier: %s", identifier)
	}

	err := json.Unmarshal(chalInfoBytes, &chalInfo)
	if err != nil {
		return Challenge{}, false, fmt.Errorf("decoding challenge token file %s (corrupted?): %v", tokenKey, err)
	}

	return Challenge{Challenge: chalInfo}, true, nil
}

// checkStorage tests the storage by writing random bytes
// to a random key, and then loading those bytes and
// comparing the loaded value. If this fails, the provided
// cfg.Storage mechanism should not be used.
func (cfg *Config) checkStorage() error {
	key := fmt.Sprintf("rw_test_%d", weakrand.Int())
	contents := make([]byte, 1024*10) // size sufficient for one or two ACME resources
	_, err := weakrand.Read(contents)
	if err != nil {
		return err
	}
	err = cfg.Storage.Store(key, contents)
	if err != nil {
		return err
	}
	defer func() {
		deleteErr := cfg.Storage.Delete(key)
		if deleteErr != nil {
			if cfg.Logger != nil {
				cfg.Logger.Error("deleting test key from storage",
					zap.String("key", key), zap.Error(err))
			}
		}
		// if there was no other error, make sure
		// to return any error returned from Delete
		if err == nil {
			err = deleteErr
		}
	}()
	loaded, err := cfg.Storage.Load(key)
	if err != nil {
		return err
	}
	if !bytes.Equal(contents, loaded) {
		return fmt.Errorf("load yielded different value than was stored; expected %d bytes, got %d bytes of differing elements", len(contents), len(loaded))
	}
	return nil
}

// storageHasCertResources returns true if the storage
// associated with cfg's certificate cache has all the
// resources related to the certificate for domain: the
// certificate, the private key, and the metadata.
func (cfg *Config) storageHasCertResources(issuer Issuer, domain string) bool {
	issuerKey := issuer.IssuerKey()
	certKey := StorageKeys.SiteCert(issuerKey, domain)
	keyKey := StorageKeys.SitePrivateKey(issuerKey, domain)
	metaKey := StorageKeys.SiteMeta(issuerKey, domain)
	return cfg.Storage.Exists(certKey) &&
		cfg.Storage.Exists(keyKey) &&
		cfg.Storage.Exists(metaKey)
}

// deleteSiteAssets deletes the folder in storage containing the
// certificate, private key, and metadata file for domain from the
// issuer with the given issuer key.
func (cfg *Config) deleteSiteAssets(issuerKey, domain string) error {
	err := cfg.Storage.Delete(StorageKeys.SiteCert(issuerKey, domain))
	if err != nil {
		return fmt.Errorf("deleting certificate file: %v", err)
	}
	err = cfg.Storage.Delete(StorageKeys.SitePrivateKey(issuerKey, domain))
	if err != nil {
		return fmt.Errorf("deleting private key: %v", err)
	}
	err = cfg.Storage.Delete(StorageKeys.SiteMeta(issuerKey, domain))
	if err != nil {
		return fmt.Errorf("deleting metadata file: %v", err)
	}
	err = cfg.Storage.Delete(StorageKeys.CertsSitePrefix(issuerKey, domain))
	if err != nil {
		return fmt.Errorf("deleting site asset folder: %v", err)
	}
	return nil
}

// lockKey returns a key for a lock that is specific to the operation
// named op being performed related to domainName and this config's CA.
func (cfg *Config) lockKey(op, domainName string) string {
	return fmt.Sprintf("%s_%s", op, domainName)
}

// managedCertNeedsRenewal returns true if certRes is expiring soon or already expired,
// or if the process of decoding the cert and checking its expiration returned an error.
func (cfg *Config) managedCertNeedsRenewal(certRes CertificateResource) (time.Duration, bool) {
	certChain, err := parseCertsFromPEMBundle(certRes.CertificatePEM)
	if err != nil {
		return 0, true
	}
	remaining := time.Until(certChain[0].NotAfter)
	needsRenew := currentlyInRenewalWindow(certChain[0].NotBefore, certChain[0].NotAfter, cfg.RenewalWindowRatio)
	return remaining, needsRenew
}

func (cfg *Config) emit(eventName string, data interface{}) {
	if cfg.OnEvent == nil {
		return
	}
	cfg.OnEvent(eventName, data)
}

func loggerNamed(l *zap.Logger, name string) *zap.Logger {
	if l == nil {
		return nil
	}
	return l.Named(name)
}

// CertificateSelector is a type which can select a certificate to use given multiple choices.
type CertificateSelector interface {
	SelectCertificate(*tls.ClientHelloInfo, []Certificate) (Certificate, error)
}

// OCSPConfig configures how OCSP is handled.
type OCSPConfig struct {
	// Disable automatic OCSP stapling; strongly
	// discouraged unless you have a good reason.
	// Disabling this puts clients at greater risk
	// and reduces their privacy.
	DisableStapling bool

	// A map of OCSP responder domains to replacement
	// domains for querying OCSP servers. Used for
	// overriding the OCSP responder URL that is
	// embedded in certificates. Mapping to an empty
	// URL will disable OCSP from that responder.
	ResponderOverrides map[string]string
}

// certIssueLockOp is the name of the operation used
// when naming a lock to make it mutually exclusive
// with other certificate issuance operations for a
// certain name.
const certIssueLockOp = "issue_cert"

// Constants for PKIX MustStaple extension.
var (
	tlsFeatureExtensionOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 24}
	ocspMustStapleFeature  = []byte{0x30, 0x03, 0x02, 0x01, 0x05}
	mustStapleExtension    = pkix.Extension{
		Id:    tlsFeatureExtensionOID,
		Value: ocspMustStapleFeature,
	}
)
