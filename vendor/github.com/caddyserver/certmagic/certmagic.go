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

// Package certmagic automates the obtaining and renewal of TLS certificates,
// including TLS & HTTPS best practices such as robust OCSP stapling, caching,
// HTTP->HTTPS redirects, and more.
//
// Its high-level API serves your HTTP handlers over HTTPS if you simply give
// the domain name(s) and the http.Handler; CertMagic will create and run
// the HTTPS server for you, fully managing certificates during the lifetime
// of the server. Similarly, it can be used to start TLS listeners or return
// a ready-to-use tls.Config -- whatever layer you need TLS for, CertMagic
// makes it easy. See the HTTPS, Listen, and TLS functions for that.
//
// If you need more control, create a Cache using NewCache() and then make
// a Config using New(). You can then call Manage() on the config. But if
// you use this lower-level API, you'll have to be sure to solve the HTTP
// and TLS-ALPN challenges yourself (unless you disabled them or use the
// DNS challenge) by using the provided Config.GetCertificate function
// in your tls.Config and/or Config.HTTPChallangeHandler in your HTTP
// handler.
//
// See the package's README for more instruction.
package certmagic

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// HTTPS serves mux for all domainNames using the HTTP
// and HTTPS ports, redirecting all HTTP requests to HTTPS.
// It uses the Default config.
//
// This high-level convenience function is opinionated and
// applies sane defaults for production use, including
// timeouts for HTTP requests and responses. To allow very
// long-lived connections, you should make your own
// http.Server values and use this package's Listen(), TLS(),
// or Config.TLSConfig() functions to customize to your needs.
// For example, servers which need to support large uploads or
// downloads with slow clients may need to use longer timeouts,
// thus this function is not suitable.
//
// Calling this function signifies your acceptance to
// the CA's Subscriber Agreement and/or Terms of Service.
func HTTPS(domainNames []string, mux http.Handler) error {
	if mux == nil {
		mux = http.DefaultServeMux
	}

	DefaultACME.Agreed = true
	cfg := NewDefault()

	err := cfg.ManageSync(domainNames)
	if err != nil {
		return err
	}

	httpWg.Add(1)
	defer httpWg.Done()

	// if we haven't made listeners yet, do so now,
	// and clean them up when all servers are done
	lnMu.Lock()
	if httpLn == nil && httpsLn == nil {
		httpLn, err = net.Listen("tcp", fmt.Sprintf(":%d", HTTPPort))
		if err != nil {
			lnMu.Unlock()
			return err
		}

		tlsConfig := cfg.TLSConfig()
		tlsConfig.NextProtos = append([]string{"h2", "http/1.1"}, tlsConfig.NextProtos...)

		httpsLn, err = tls.Listen("tcp", fmt.Sprintf(":%d", HTTPSPort), tlsConfig)
		if err != nil {
			httpLn.Close()
			httpLn = nil
			lnMu.Unlock()
			return err
		}

		go func() {
			httpWg.Wait()
			lnMu.Lock()
			httpLn.Close()
			httpsLn.Close()
			lnMu.Unlock()
		}()
	}
	hln, hsln := httpLn, httpsLn
	lnMu.Unlock()

	// create HTTP/S servers that are configured
	// with sane default timeouts and appropriate
	// handlers (the HTTP server solves the HTTP
	// challenge and issues redirects to HTTPS,
	// while the HTTPS server simply serves the
	// user's handler)
	httpServer := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       5 * time.Second,
	}
	if len(cfg.Issuers) > 0 {
		if am, ok := cfg.Issuers[0].(*ACMEManager); ok {
			httpServer.Handler = am.HTTPChallengeHandler(http.HandlerFunc(httpRedirectHandler))
		}
	}
	httpsServer := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       5 * time.Minute,
		Handler:           mux,
	}

	log.Printf("%v Serving HTTP->HTTPS on %s and %s",
		domainNames, hln.Addr(), hsln.Addr())

	go httpServer.Serve(hln)
	return httpsServer.Serve(hsln)
}

func httpRedirectHandler(w http.ResponseWriter, r *http.Request) {
	toURL := "https://"

	// since we redirect to the standard HTTPS port, we
	// do not need to include it in the redirect URL
	requestHost := hostOnly(r.Host)

	toURL += requestHost
	toURL += r.URL.RequestURI()

	// get rid of this disgusting unencrypted HTTP connection 🤢
	w.Header().Set("Connection", "close")

	http.Redirect(w, r, toURL, http.StatusMovedPermanently)
}

// TLS enables management of certificates for domainNames
// and returns a valid tls.Config. It uses the Default
// config.
//
// Because this is a convenience function that returns
// only a tls.Config, it does not assume HTTP is being
// served on the HTTP port, so the HTTP challenge is
// disabled (no HTTPChallengeHandler is necessary). The
// package variable Default is modified so that the
// HTTP challenge is disabled.
//
// Calling this function signifies your acceptance to
// the CA's Subscriber Agreement and/or Terms of Service.
func TLS(domainNames []string) (*tls.Config, error) {
	DefaultACME.Agreed = true
	DefaultACME.DisableHTTPChallenge = true
	cfg := NewDefault()
	return cfg.TLSConfig(), cfg.ManageSync(domainNames)
}

// Listen manages certificates for domainName and returns a
// TLS listener. It uses the Default config.
//
// Because this convenience function returns only a TLS-enabled
// listener and does not presume HTTP is also being served,
// the HTTP challenge will be disabled. The package variable
// Default is modified so that the HTTP challenge is disabled.
//
// Calling this function signifies your acceptance to
// the CA's Subscriber Agreement and/or Terms of Service.
func Listen(domainNames []string) (net.Listener, error) {
	DefaultACME.Agreed = true
	DefaultACME.DisableHTTPChallenge = true
	cfg := NewDefault()
	err := cfg.ManageSync(domainNames)
	if err != nil {
		return nil, err
	}
	return tls.Listen("tcp", fmt.Sprintf(":%d", HTTPSPort), cfg.TLSConfig())
}

// ManageSync obtains certificates for domainNames and keeps them
// renewed using the Default config.
//
// This is a slightly lower-level function; you will need to
// wire up support for the ACME challenges yourself. You can
// obtain a Config to help you do that by calling NewDefault().
//
// You will need to ensure that you use a TLS config that gets
// certificates from this Config and that the HTTP and TLS-ALPN
// challenges can be solved. The easiest way to do this is to
// use NewDefault().TLSConfig() as your TLS config and to wrap
// your HTTP handler with NewDefault().HTTPChallengeHandler().
// If you don't have an HTTP server, you will need to disable
// the HTTP challenge.
//
// If you already have a TLS config you want to use, you can
// simply set its GetCertificate field to
// NewDefault().GetCertificate.
//
// Calling this function signifies your acceptance to
// the CA's Subscriber Agreement and/or Terms of Service.
func ManageSync(domainNames []string) error {
	DefaultACME.Agreed = true
	return NewDefault().ManageSync(domainNames)
}

// ManageAsync is the same as ManageSync, except that
// certificates are managed asynchronously. This means
// that the function will return before certificates
// are ready, and errors that occur during certificate
// obtain or renew operations are only logged. It is
// vital that you monitor the logs if using this method,
// which is only recommended for automated/non-interactive
// environments.
func ManageAsync(ctx context.Context, domainNames []string) error {
	DefaultACME.Agreed = true
	return NewDefault().ManageAsync(ctx, domainNames)
}

// OnDemandConfig configures on-demand TLS (certificate
// operations as-needed, like during TLS handshakes,
// rather than immediately).
//
// When this package's high-level convenience functions
// are used (HTTPS, Manage, etc., where the Default
// config is used as a template), this struct regulates
// certificate operations using an implicit whitelist
// containing the names passed into those functions if
// no DecisionFunc is set. This ensures some degree of
// control by default to avoid certificate operations for
// aribtrary domain names. To override this whitelist,
// manually specify a DecisionFunc. To impose rate limits,
// specify your own DecisionFunc.
type OnDemandConfig struct {
	// If set, this function will be called to determine
	// whether a certificate can be obtained or renewed
	// for the given name. If an error is returned, the
	// request will be denied.
	DecisionFunc func(name string) error

	// List of whitelisted hostnames (SNI values) for
	// deferred (on-demand) obtaining of certificates.
	// Used only by higher-level functions in this
	// package to persist the list of hostnames that
	// the config is supposed to manage. This is done
	// because it seems reasonable that if you say
	// "Manage [domain names...]", then only those
	// domain names should be able to have certs;
	// we don't NEED this feature, but it makes sense
	// for higher-level convenience functions to be
	// able to retain their convenience (alternative
	// is: the user manually creates a DecisionFunc
	// that whitelists the same names it already
	// passed into Manage) and without letting clients
	// have their run of any domain names they want.
	// Only enforced if len > 0.
	hostWhitelist []string
}

func (o *OnDemandConfig) whitelistContains(name string) bool {
	for _, n := range o.hostWhitelist {
		if strings.EqualFold(n, name) {
			return true
		}
	}
	return false
}

// isLoopback returns true if the hostname of addr looks
// explicitly like a common local hostname. addr must only
// be a host or a host:port combination.
func isLoopback(addr string) bool {
	host := hostOnly(addr)
	return host == "localhost" ||
		strings.Trim(host, "[]") == "::1" ||
		strings.HasPrefix(host, "127.")
}

// isInternal returns true if the IP of addr
// belongs to a private network IP range. addr
// must only be an IP or an IP:port combination.
// Loopback addresses are considered false.
func isInternal(addr string) bool {
	privateNetworks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"fc00::/7",
	}
	host := hostOnly(addr)
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, privateNetwork := range privateNetworks {
		_, ipnet, _ := net.ParseCIDR(privateNetwork)
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}

// hostOnly returns only the host portion of hostport.
// If there is no port or if there is an error splitting
// the port off, the whole input string is returned.
func hostOnly(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport // OK; probably had no port to begin with
	}
	return host
}

// PreChecker is an interface that can be optionally implemented by
// Issuers. Pre-checks are performed before each call (or batch of
// identical calls) to Issue(), giving the issuer the option to ensure
// it has all the necessary information/state.
type PreChecker interface {
	PreCheck(ctx context.Context, names []string, interactive bool) error
}

// Issuer is a type that can issue certificates.
type Issuer interface {
	// Issue obtains a certificate for the given CSR. It
	// must honor context cancellation if it is long-running.
	// It can also use the context to find out if the current
	// call is part of a retry, via AttemptsCtxKey.
	Issue(ctx context.Context, request *x509.CertificateRequest) (*IssuedCertificate, error)

	// IssuerKey must return a string that uniquely identifies
	// this particular configuration of the Issuer such that
	// any certificates obtained by this Issuer will be treated
	// as identical if they have the same SANs.
	//
	// Certificates obtained from Issuers with the same IssuerKey
	// will overwrite others with the same SANs. For example, an
	// Issuer might be able to obtain certificates from different
	// CAs, say A and B. It is likely that the CAs have different
	// use cases and purposes (e.g. testing and production), so
	// their respective certificates should not overwrite eaach
	// other.
	IssuerKey() string
}

// Revoker can revoke certificates. Reason codes are defined
// by RFC 5280 §5.3.1: https://tools.ietf.org/html/rfc5280#section-5.3.1
// and are available as constants in our ACME library.
type Revoker interface {
	Revoke(ctx context.Context, cert CertificateResource, reason int) error
}

// KeyGenerator can generate a private key.
type KeyGenerator interface {
	// GenerateKey generates a private key. The returned
	// PrivateKey must be able to expose its associated
	// public key.
	GenerateKey() (crypto.PrivateKey, error)
}

// IssuedCertificate represents a certificate that was just issued.
type IssuedCertificate struct {
	// The PEM-encoding of DER-encoded ASN.1 data.
	Certificate []byte

	// Any extra information to serialize alongside the
	// certificate in storage.
	Metadata interface{}
}

// CertificateResource associates a certificate with its private
// key and other useful information, for use in maintaining the
// certificate.
type CertificateResource struct {
	// The list of names on the certificate;
	// for convenience only.
	SANs []string `json:"sans,omitempty"`

	// The PEM-encoding of DER-encoded ASN.1 data
	// for the cert or chain.
	CertificatePEM []byte `json:"-"`

	// The PEM-encoding of the certificate's private key.
	PrivateKeyPEM []byte `json:"-"`

	// Any extra information associated with the certificate,
	// usually provided by the issuer implementation.
	IssuerData interface{} `json:"issuer_data,omitempty"`

	// The unique string identifying the issuer of the
	// certificate; internally useful for storage access.
	issuerKey string `json:"-"`
}

// NamesKey returns the list of SANs as a single string,
// truncated to some ridiculously long size limit. It
// can act as a key for the set of names on the resource.
func (cr *CertificateResource) NamesKey() string {
	sort.Strings(cr.SANs)
	result := strings.Join(cr.SANs, ",")
	if len(result) > 1024 {
		const trunc = "_trunc"
		result = result[:1024-len(trunc)] + trunc
	}
	return result
}

// Default contains the package defaults for the
// various Config fields. This is used as a template
// when creating your own Configs with New() or
// NewDefault(), and it is also used as the Config
// by all the high-level functions in this package
// that abstract away most configuration (HTTPS(),
// TLS(), Listen(), etc).
//
// The fields of this value will be used for Config
// fields which are unset. Feel free to modify these
// defaults, but do not use this Config by itself: it
// is only a template. Valid configurations can be
// obtained by calling New() (if you have your own
// certificate cache) or NewDefault() (if you only
// need a single config and want to use the default
// cache).
//
// Even if the Issuers or Storage fields are not set,
// defaults will be applied in the call to New().
var Default = Config{
	RenewalWindowRatio: DefaultRenewalWindowRatio,
	Storage:            defaultFileStorage,
	KeySource:          DefaultKeyGenerator,
}

const (
	// HTTPChallengePort is the officially-designated port for
	// the HTTP challenge according to the ACME spec.
	HTTPChallengePort = 80

	// TLSALPNChallengePort is the officially-designated port for
	// the TLS-ALPN challenge according to the ACME spec.
	TLSALPNChallengePort = 443
)

// Port variables must remain their defaults unless you
// forward packets from the defaults to whatever these
// are set to; otherwise ACME challenges will fail.
var (
	// HTTPPort is the port on which to serve HTTP
	// and, as such, the HTTP challenge (unless
	// Default.AltHTTPPort is set).
	HTTPPort = 80

	// HTTPSPort is the port on which to serve HTTPS
	// and, as such, the TLS-ALPN challenge
	// (unless Default.AltTLSALPNPort is set).
	HTTPSPort = 443
)

// Variables for conveniently serving HTTPS.
var (
	httpLn, httpsLn net.Listener
	lnMu            sync.Mutex
	httpWg          sync.WaitGroup
)

// Maximum size for the stack trace when recovering from panics.
const stackTraceBufferSize = 1024 * 128
