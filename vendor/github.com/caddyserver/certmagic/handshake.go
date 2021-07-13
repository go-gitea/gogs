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
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/mholt/acmez"
	"go.uber.org/zap"
)

// GetCertificate gets a certificate to satisfy clientHello. In getting
// the certificate, it abides the rules and settings defined in the
// Config that matches clientHello.ServerName. It first checks the in-
// memory cache, then, if the config enables "OnDemand", it accesses
// disk, then accesses the network if it must obtain a new certificate
// via ACME.
//
// This method is safe for use as a tls.Config.GetCertificate callback.
func (cfg *Config) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cfg.emit("tls_handshake_started", clientHello)

	// special case: serve up the certificate for a TLS-ALPN ACME challenge
	// (https://tools.ietf.org/html/draft-ietf-acme-tls-alpn-05)
	for _, proto := range clientHello.SupportedProtos {
		if proto == acmez.ACMETLS1Protocol {
			challengeCert, distributed, err := cfg.getTLSALPNChallengeCert(clientHello)
			if err != nil {
				if cfg.Logger != nil {
					cfg.Logger.Error("tls-alpn challenge",
						zap.String("server_name", clientHello.ServerName),
						zap.Error(err))
				}
				return nil, err
			}
			if cfg.Logger != nil {
				cfg.Logger.Info("served key authentication certificate",
					zap.String("server_name", clientHello.ServerName),
					zap.String("challenge", "tls-alpn-01"),
					zap.String("remote", clientHello.Conn.RemoteAddr().String()),
					zap.Bool("distributed", distributed))
			}
			return challengeCert, nil
		}
	}

	// get the certificate and serve it up
	cert, err := cfg.getCertDuringHandshake(clientHello, true, true)
	if err == nil {
		cfg.emit("tls_handshake_completed", clientHello)
	}
	return &cert.Certificate, err
}

// getCertificate gets a certificate that matches name from the in-memory
// cache, according to the lookup table associated with cfg. The lookup then
// points to a certificate in the Instance certificate cache.
//
// The name is expected to already be normalized (e.g. lowercased).
//
// If there is no exact match for name, it will be checked against names of
// the form '*.example.com' (wildcard certificates) according to RFC 6125.
// If a match is found, matched will be true. If no matches are found, matched
// will be false and a "default" certificate will be returned with defaulted
// set to true. If defaulted is false, then no certificates were available.
//
// The logic in this function is adapted from the Go standard library,
// which is by the Go Authors.
//
// This function is safe for concurrent use.
func (cfg *Config) getCertificate(hello *tls.ClientHelloInfo) (cert Certificate, matched, defaulted bool) {
	name := normalizedName(hello.ServerName)

	if name == "" {
		// if SNI is empty, prefer matching IP address
		if hello.Conn != nil {
			addr := localIPFromConn(hello.Conn)
			cert, matched = cfg.selectCert(hello, addr)
			if matched {
				return
			}
		}

		// fall back to a "default" certificate, if specified
		if cfg.DefaultServerName != "" {
			normDefault := normalizedName(cfg.DefaultServerName)
			cert, defaulted = cfg.selectCert(hello, normDefault)
			if defaulted {
				return
			}
		}
	} else {
		// if SNI is specified, try an exact match first
		cert, matched = cfg.selectCert(hello, name)
		if matched {
			return
		}

		// try replacing labels in the name with
		// wildcards until we get a match
		labels := strings.Split(name, ".")
		for i := range labels {
			labels[i] = "*"
			candidate := strings.Join(labels, ".")
			cert, matched = cfg.selectCert(hello, candidate)
			if matched {
				return
			}
		}

		// check the certCache directly to see if the SNI name is
		// already the key of the certificate it wants; this implies
		// that the SNI can contain the hash of a specific cert
		// (chain) it wants and we will still be able to serve it up
		// (this behavior, by the way, could be controversial as to
		// whether it complies with RFC 6066 about SNI, but I think
		// it does, soooo...)
		// (this is how we solved the former ACME TLS-SNI challenge)
		cfg.certCache.mu.RLock()
		directCert, ok := cfg.certCache.cache[name]
		cfg.certCache.mu.RUnlock()
		if ok {
			cert = directCert
			matched = true
			return
		}
	}

	// otherwise, we're bingo on ammo; see issues
	// caddyserver/caddy#2035 and caddyserver/caddy#1303 (any
	// change to certificate matching behavior must
	// account for hosts defined where the hostname
	// is empty or a catch-all, like ":443" or
	// "0.0.0.0:443")

	return
}

// selectCert uses hello to select a certificate from the
// cache for name. If cfg.CertSelection is set, it will be
// used to make the decision. Otherwise, the first matching
// unexpired cert is returned. As a special case, if no
// certificates match name and cfg.CertSelection is set,
// then all certificates in the cache will be passed in
// for the cfg.CertSelection to make the final decision.
func (cfg *Config) selectCert(hello *tls.ClientHelloInfo, name string) (Certificate, bool) {
	choices := cfg.certCache.getAllMatchingCerts(name)
	if len(choices) == 0 {
		if cfg.CertSelection == nil {
			return Certificate{}, false
		}
		choices = cfg.certCache.getAllCerts()
	}
	if cfg.CertSelection == nil {
		cert, err := DefaultCertificateSelector(hello, choices)
		return cert, err == nil
	}
	cert, err := cfg.CertSelection.SelectCertificate(hello, choices)
	return cert, err == nil
}

// DefaultCertificateSelector is the default certificate selection logic
// given a choice of certificates. If there is at least one certificate in
// choices, it always returns a certificate without error. It chooses the
// first non-expired certificate that the client supports if possible,
// otherwise it returns an expired certificate that the client supports,
// otherwise it just returns the first certificate in the list of choices.
func DefaultCertificateSelector(hello *tls.ClientHelloInfo, choices []Certificate) (Certificate, error) {
	if len(choices) == 0 {
		return Certificate{}, fmt.Errorf("no certificates available")
	}
	now := time.Now()
	best := choices[0]
	for _, choice := range choices {
		if err := hello.SupportsCertificate(&choice.Certificate); err != nil {
			continue
		}
		best = choice // at least the client supports it...
		if now.After(choice.Leaf.NotBefore) && now.Before(choice.Leaf.NotAfter) {
			return choice, nil // ...and unexpired, great! "Certificate, I choose you!"
		}
	}
	return best, nil // all matching certs are expired or incompatible, oh well
}

// getCertDuringHandshake will get a certificate for hello. It first tries
// the in-memory cache. If no certificate for hello is in the cache, the
// config most closely corresponding to hello will be loaded. If that config
// allows it (OnDemand==true) and if loadIfNecessary == true, it goes to disk
// to load it into the cache and serve it. If it's not on disk and if
// obtainIfNecessary == true, the certificate will be obtained from the CA,
// cached, and served. If obtainIfNecessary is true, then loadIfNecessary
// must also be set to true. An error will be returned if and only if no
// certificate is available.
//
// This function is safe for concurrent use.
func (cfg *Config) getCertDuringHandshake(hello *tls.ClientHelloInfo, loadIfNecessary, obtainIfNecessary bool) (Certificate, error) {
	log := loggerNamed(cfg.Logger, "on_demand")

	// First check our in-memory cache to see if we've already loaded it
	cert, matched, defaulted := cfg.getCertificate(hello)
	if matched {
		if cert.managed && cfg.OnDemand != nil && obtainIfNecessary {
			// It's been reported before that if the machine goes to sleep (or
			// suspends the process) that certs which are already loaded into
			// memory won't get renewed in the background, so we need to check
			// expiry on each handshake too, sigh:
			// https://caddy.community/t/local-certificates-not-renewing-on-demand/9482
			return cfg.optionalMaintenance(log, cert, hello)
		}
		return cert, nil
	}

	name := cfg.getNameFromClientHello(hello)

	// If OnDemand is enabled, then we might be able to load or
	// obtain a needed certificate
	if cfg.OnDemand != nil && loadIfNecessary {
		// Then check to see if we have one on disk
		loadedCert, err := cfg.CacheManagedCertificate(name)
		if _, ok := err.(ErrNotExist); ok {
			// If no exact match, try a wildcard variant, which is something we can still use
			labels := strings.Split(name, ".")
			labels[0] = "*"
			loadedCert, err = cfg.CacheManagedCertificate(strings.Join(labels, "."))
		}
		if err == nil {
			loadedCert, err = cfg.handshakeMaintenance(hello, loadedCert)
			if err != nil {
				if log != nil {
					log.Error("maintining newly-loaded certificate",
						zap.String("server_name", name),
						zap.Error(err))
				}
			}
			return loadedCert, nil
		}
		if obtainIfNecessary {
			// By this point, we need to ask the CA for a certificate
			return cfg.obtainOnDemandCertificate(hello)
		}
	}

	// Fall back to the default certificate if there is one
	if defaulted {
		return cert, nil
	}

	return Certificate{}, fmt.Errorf("no certificate available for '%s'", name)
}

// optionalMaintenance will perform maintenance on the certificate (if necessary) and
// will return the resulting certificate. This should only be done if the certificate
// is managed, OnDemand is enabled, and the scope is allowed to obtain certificates.
func (cfg *Config) optionalMaintenance(log *zap.Logger, cert Certificate, hello *tls.ClientHelloInfo) (Certificate, error) {
	newCert, err := cfg.handshakeMaintenance(hello, cert)
	if err == nil {
		return newCert, nil
	}

	if log != nil {
		log.Error("renewing certificate on-demand failed",
			zap.Strings("subjects", cert.Names),
			zap.Time("not_after", cert.Leaf.NotAfter),
			zap.Error(err))
	}

	if cert.Expired() {
		return cert, err
	}

	// still has time remaining, so serve it anyway
	return cert, nil
}

// checkIfCertShouldBeObtained checks to see if an on-demand TLS certificate
// should be obtained for a given domain based upon the config settings. If
// a non-nil error is returned, do not issue a new certificate for name.
func (cfg *Config) checkIfCertShouldBeObtained(name string) error {
	if cfg.OnDemand == nil {
		return fmt.Errorf("not configured for on-demand certificate issuance")
	}
	if !SubjectQualifiesForCert(name) {
		return fmt.Errorf("subject name does not qualify for certificate: %s", name)
	}
	if cfg.OnDemand.DecisionFunc != nil {
		return cfg.OnDemand.DecisionFunc(name)
	}
	if len(cfg.OnDemand.hostWhitelist) > 0 &&
		!cfg.OnDemand.whitelistContains(name) {
		return fmt.Errorf("certificate for '%s' is not managed", name)
	}
	return nil
}

// obtainOnDemandCertificate obtains a certificate for hello.
// If another goroutine has already started obtaining a cert for
// hello, it will wait and use what the other goroutine obtained.
//
// This function is safe for use by multiple concurrent goroutines.
func (cfg *Config) obtainOnDemandCertificate(hello *tls.ClientHelloInfo) (Certificate, error) {
	log := loggerNamed(cfg.Logger, "on_demand")

	name := cfg.getNameFromClientHello(hello)

	getCertWithoutReobtaining := func() (Certificate, error) {
		// very important to set the obtainIfNecessary argument to false, so we don't repeat this infinitely
		return cfg.getCertDuringHandshake(hello, true, false)
	}

	// We must protect this process from happening concurrently, so synchronize.
	obtainCertWaitChansMu.Lock()
	wait, ok := obtainCertWaitChans[name]
	if ok {
		// lucky us -- another goroutine is already obtaining the certificate.
		// wait for it to finish obtaining the cert and then we'll use it.
		obtainCertWaitChansMu.Unlock()

		// TODO: see if we can get a proper context in here, for true cancellation
		timeout := time.NewTimer(2 * time.Minute)
		select {
		case <-timeout.C:
			return Certificate{}, fmt.Errorf("timed out waiting to obtain certificate for %s", name)
		case <-wait:
			timeout.Stop()
		}

		return getCertWithoutReobtaining()
	}

	// looks like it's up to us to do all the work and obtain the cert.
	// make a chan others can wait on if needed
	wait = make(chan struct{})
	obtainCertWaitChans[name] = wait
	obtainCertWaitChansMu.Unlock()

	unblockWaiters := func() {
		obtainCertWaitChansMu.Lock()
		close(wait)
		delete(obtainCertWaitChans, name)
		obtainCertWaitChansMu.Unlock()
	}

	// Make sure the certificate should be obtained based on config
	err := cfg.checkIfCertShouldBeObtained(name)
	if err != nil {
		unblockWaiters()
		return Certificate{}, err
	}

	if log != nil {
		log.Info("obtaining new certificate", zap.String("server_name", name))
	}

	// TODO: use a proper context; we use one with timeout because retries are enabled because interactive is false
	ctx, cancel := context.WithTimeout(context.TODO(), 90*time.Second)
	defer cancel()

	// Obtain the certificate
	err = cfg.ObtainCertAsync(ctx, name)

	// immediately unblock anyone waiting for it; doing this in
	// a defer would risk deadlock because of the recursive call
	// to getCertDuringHandshake below when we return!
	unblockWaiters()

	if err != nil {
		// shucks; failed to solve challenge on-demand
		return Certificate{}, err
	}

	// success; certificate was just placed on disk, so
	// we need only restart serving the certificate
	return getCertWithoutReobtaining()
}

// handshakeMaintenance performs a check on cert for expiration and OCSP validity.
// If necessary, it will renew the certificate and/or refresh the OCSP staple.
// OCSP stapling errors are not returned, only logged.
//
// This function is safe for use by multiple concurrent goroutines.
func (cfg *Config) handshakeMaintenance(hello *tls.ClientHelloInfo, cert Certificate) (Certificate, error) {
	log := loggerNamed(cfg.Logger, "on_demand")

	// Check cert expiration
	if currentlyInRenewalWindow(cert.Leaf.NotBefore, cert.Leaf.NotAfter, cfg.RenewalWindowRatio) {
		return cfg.renewDynamicCertificate(hello, cert)
	}

	// Check OCSP staple validity
	if cert.ocsp != nil {
		refreshTime := cert.ocsp.ThisUpdate.Add(cert.ocsp.NextUpdate.Sub(cert.ocsp.ThisUpdate) / 2)
		if time.Now().After(refreshTime) {
			_, err := stapleOCSP(cfg.OCSP, cfg.Storage, &cert, nil)
			if err != nil {
				// An error with OCSP stapling is not the end of the world, and in fact, is
				// quite common considering not all certs have issuer URLs that support it.
				if log != nil {
					log.Warn("stapling OCSP",
						zap.String("server_name", hello.ServerName),
						zap.Error(err))
				}
			}
			cfg.certCache.mu.Lock()
			cfg.certCache.cache[cert.hash] = cert
			cfg.certCache.mu.Unlock()
		}
	}

	return cert, nil
}

// renewDynamicCertificate renews the certificate for name using cfg. It returns the
// certificate to use and an error, if any. name should already be lower-cased before
// calling this function. name is the name obtained directly from the handshake's
// ClientHello. If the certificate hasn't yet expired, currentCert will be returned
// and the renewal will happen in the background; otherwise this blocks until the
// certificate has been renewed, and returns the renewed certificate.
//
// This function is safe for use by multiple concurrent goroutines.
func (cfg *Config) renewDynamicCertificate(hello *tls.ClientHelloInfo, currentCert Certificate) (Certificate, error) {
	log := loggerNamed(cfg.Logger, "on_demand")

	name := cfg.getNameFromClientHello(hello)
	timeLeft := time.Until(currentCert.Leaf.NotAfter)

	getCertWithoutReobtaining := func() (Certificate, error) {
		// very important to set the obtainIfNecessary argument to false, so we don't repeat this infinitely
		return cfg.getCertDuringHandshake(hello, true, false)
	}

	// see if another goroutine is already working on this certificate
	obtainCertWaitChansMu.Lock()
	wait, ok := obtainCertWaitChans[name]
	if ok {
		// lucky us -- another goroutine is already renewing the certificate
		obtainCertWaitChansMu.Unlock()

		if timeLeft > 0 {
			// the current certificate hasn't expired, and another goroutine is already
			// renewing it, so we might as well serve what we have without blocking
			if log != nil {
				log.Debug("certificate expires soon but is already being renewed; serving current certificate",
					zap.Strings("identifiers", currentCert.Names),
					zap.Duration("remaining", timeLeft))
			}
			return currentCert, nil
		}

		// otherwise, we'll have to wait for the renewal to finish so we don't serve
		// an expired certificate

		if log != nil {
			log.Debug("certificate has expired, but is already being renewed; waiting for renewal to complete",
				zap.Strings("identifiers", currentCert.Names),
				zap.Time("expired", currentCert.Leaf.NotAfter))
		}

		// TODO: see if we can get a proper context in here, for true cancellation
		timeout := time.NewTimer(2 * time.Minute)
		select {
		case <-timeout.C:
			return Certificate{}, fmt.Errorf("timed out waiting for certificate renewal of %s", name)
		case <-wait:
			timeout.Stop()
		}

		return getCertWithoutReobtaining()
	}

	// looks like it's up to us to do all the work and renew the cert
	wait = make(chan struct{})
	obtainCertWaitChans[name] = wait
	obtainCertWaitChansMu.Unlock()

	unblockWaiters := func() {
		obtainCertWaitChansMu.Lock()
		close(wait)
		delete(obtainCertWaitChans, name)
		obtainCertWaitChansMu.Unlock()
	}

	if log != nil {
		log.Info("attempting certificate renewal",
			zap.String("server_name", name),
			zap.Strings("identifiers", currentCert.Names),
			zap.Time("expiration", currentCert.Leaf.NotAfter),
			zap.Duration("remaining", timeLeft))
	}

	// Make sure a certificate for this name should be obtained on-demand
	err := cfg.checkIfCertShouldBeObtained(name)
	if err != nil {
		// if not, remove from cache (it will be deleted from storage later)
		cfg.certCache.mu.Lock()
		cfg.certCache.removeCertificate(currentCert)
		cfg.certCache.mu.Unlock()
		unblockWaiters()
		return Certificate{}, err
	}

	// Renew and reload the certificate
	renewAndReload := func(ctx context.Context, cancel context.CancelFunc) (Certificate, error) {
		defer cancel()
		err = cfg.RenewCertAsync(ctx, name, false)
		if err == nil {
			// even though the recursive nature of the dynamic cert loading
			// would just call this function anyway, we do it here to
			// make the replacement as atomic as possible.
			newCert, err := cfg.CacheManagedCertificate(name)
			if err != nil {
				if log != nil {
					log.Error("loading renewed certificate", zap.String("server_name", name), zap.Error(err))
				}
			} else {
				// replace the old certificate with the new one
				cfg.certCache.replaceCertificate(currentCert, newCert)
			}
		}

		// immediately unblock anyone waiting for it; doing this in
		// a defer would risk deadlock because of the recursive call
		// to getCertDuringHandshake below when we return!
		unblockWaiters()

		if err != nil {
			return Certificate{}, err
		}

		return getCertWithoutReobtaining()
	}

	// if the certificate hasn't expired, we can serve what we have and renew in the background
	if timeLeft > 0 {
		// TODO: get a proper context; we use one with timeout because retries are enabled because interactive is false
		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Minute)
		go renewAndReload(ctx, cancel)
		return currentCert, nil
	}

	// otherwise, we have to block while we renew an expired certificate
	ctx, cancel := context.WithTimeout(context.TODO(), 90*time.Second)
	return renewAndReload(ctx, cancel)
}

// getTLSALPNChallengeCert is to be called when the clientHello pertains to
// a TLS-ALPN challenge and a certificate is required to solve it. This method gets
// the relevant challenge info and then returns the associated certificate (if any)
// or generates it anew if it's not available (as is the case when distributed
// solving). True is returned if the challenge is being solved distributed (there
// is no semantic difference with distributed solving; it is mainly for logging).
func (cfg *Config) getTLSALPNChallengeCert(clientHello *tls.ClientHelloInfo) (*tls.Certificate, bool, error) {
	chalData, distributed, err := cfg.getChallengeInfo(clientHello.ServerName)
	if err != nil {
		return nil, distributed, err
	}

	// fast path: we already created the certificate (this avoids having to re-create
	// it at every handshake that tries to verify, e.g. multi-perspective validation)
	if chalData.data != nil {
		return chalData.data.(*tls.Certificate), distributed, nil
	}

	// otherwise, we can re-create the solution certificate, but it takes a few cycles
	cert, err := acmez.TLSALPN01ChallengeCert(chalData.Challenge)
	if err != nil {
		return nil, distributed, fmt.Errorf("making TLS-ALPN challenge certificate: %v", err)
	}
	if cert == nil {
		return nil, distributed, fmt.Errorf("got nil TLS-ALPN challenge certificate but no error")
	}

	return cert, distributed, nil
}

// getNameFromClientHello returns a normalized form of hello.ServerName.
// If hello.ServerName is empty (i.e. client did not use SNI), then the
// associated connection's local address is used to extract an IP address.
func (*Config) getNameFromClientHello(hello *tls.ClientHelloInfo) string {
	if name := normalizedName(hello.ServerName); name != "" {
		return name
	}
	return localIPFromConn(hello.Conn)
}

// localIPFromConn returns the host portion of c's local address
// and strips the scope ID if one exists (see RFC 4007).
func localIPFromConn(c net.Conn) string {
	if c == nil {
		return ""
	}
	localAddr := c.LocalAddr().String()
	ip, _, err := net.SplitHostPort(localAddr)
	if err != nil {
		// OK; assume there was no port
		ip = localAddr
	}
	// IPv6 addresses can have scope IDs, e.g. "fe80::4c3:3cff:fe4f:7e0b%eth0",
	// but for our purposes, these are useless (unless a valid use case proves
	// otherwise; see issue #3911)
	if scopeIDStart := strings.Index(ip, "%"); scopeIDStart > -1 {
		ip = ip[:scopeIDStart]
	}
	return ip
}

// normalizedName returns a cleaned form of serverName that is
// used for consistency when referring to a SNI value.
func normalizedName(serverName string) string {
	return strings.ToLower(strings.TrimSpace(serverName))
}

// obtainCertWaitChans is used to coordinate obtaining certs for each hostname.
var obtainCertWaitChans = make(map[string]chan struct{})
var obtainCertWaitChansMu sync.Mutex
