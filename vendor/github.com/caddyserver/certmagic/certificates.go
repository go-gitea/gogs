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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/ocsp"
)

// Certificate is a tls.Certificate with associated metadata tacked on.
// Even if the metadata can be obtained by parsing the certificate,
// we are more efficient by extracting the metadata onto this struct,
// but at the cost of slightly higher memory use.
type Certificate struct {
	tls.Certificate

	// Names is the list of subject names this
	// certificate is signed for.
	Names []string

	// Optional; user-provided, and arbitrary.
	Tags []string

	// OCSP contains the certificate's parsed OCSP response.
	ocsp *ocsp.Response

	// The hex-encoded hash of this cert's chain's bytes.
	hash string

	// Whether this certificate is under our management.
	managed bool

	// The unique string identifying the issuer of this certificate.
	issuerKey string
}

// NeedsRenewal returns true if the certificate is
// expiring soon (according to cfg) or has expired.
func (cert Certificate) NeedsRenewal(cfg *Config) bool {
	return currentlyInRenewalWindow(cert.Leaf.NotBefore, cert.Leaf.NotAfter, cfg.RenewalWindowRatio)
}

// Expired returns true if the certificate has expired.
func (cert Certificate) Expired() bool {
	if cert.Leaf == nil {
		// ideally cert.Leaf would never be nil, but this can happen for
		// "synthetic" certs like those made to solve the TLS-ALPN challenge
		// which adds a special cert directly  to the cache, since
		// tls.X509KeyPair() discards the leaf; oh well
		return false
	}
	return time.Now().After(cert.Leaf.NotAfter)
}

// currentlyInRenewalWindow returns true if the current time is
// within the renewal window, according to the given start/end
// dates and the ratio of the renewal window. If true is returned,
// the certificate being considered is due for renewal.
func currentlyInRenewalWindow(notBefore, notAfter time.Time, renewalWindowRatio float64) bool {
	if notAfter.IsZero() {
		return false
	}
	lifetime := notAfter.Sub(notBefore)
	if renewalWindowRatio == 0 {
		renewalWindowRatio = DefaultRenewalWindowRatio
	}
	renewalWindow := time.Duration(float64(lifetime) * renewalWindowRatio)
	renewalWindowStart := notAfter.Add(-renewalWindow)
	return time.Now().After(renewalWindowStart)
}

// HasTag returns true if cert.Tags has tag.
func (cert Certificate) HasTag(tag string) bool {
	for _, t := range cert.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// CacheManagedCertificate loads the certificate for domain into the
// cache, from the TLS storage for managed certificates. It returns a
// copy of the Certificate that was put into the cache.
//
// This is a lower-level method; normally you'll call Manage() instead.
//
// This method is safe for concurrent use.
func (cfg *Config) CacheManagedCertificate(domain string) (Certificate, error) {
	cert, err := cfg.loadManagedCertificate(domain)
	if err != nil {
		return cert, err
	}
	cfg.certCache.cacheCertificate(cert)
	cfg.emit("cached_managed_cert", cert.Names)
	return cert, nil
}

// loadManagedCertificate loads the managed certificate for domain from any
// of the configured issuers' storage locations, but it does not add it to
// the cache. It just loads from storage and returns it.
func (cfg *Config) loadManagedCertificate(domain string) (Certificate, error) {
	certRes, err := cfg.loadCertResourceAnyIssuer(domain)
	if err != nil {
		return Certificate{}, err
	}
	cert, err := cfg.makeCertificateWithOCSP(certRes.CertificatePEM, certRes.PrivateKeyPEM)
	if err != nil {
		return cert, err
	}
	cert.managed = true
	cert.issuerKey = certRes.issuerKey
	return cert, nil
}

// CacheUnmanagedCertificatePEMFile loads a certificate for host using certFile
// and keyFile, which must be in PEM format. It stores the certificate in
// the in-memory cache.
//
// This method is safe for concurrent use.
func (cfg *Config) CacheUnmanagedCertificatePEMFile(certFile, keyFile string, tags []string) error {
	cert, err := cfg.makeCertificateFromDiskWithOCSP(cfg.Storage, certFile, keyFile)
	if err != nil {
		return err
	}
	cert.Tags = tags
	cfg.certCache.cacheCertificate(cert)
	cfg.emit("cached_unmanaged_cert", cert.Names)
	return nil
}

// CacheUnmanagedTLSCertificate adds tlsCert to the certificate cache.
// It staples OCSP if possible.
//
// This method is safe for concurrent use.
func (cfg *Config) CacheUnmanagedTLSCertificate(tlsCert tls.Certificate, tags []string) error {
	var cert Certificate
	err := fillCertFromLeaf(&cert, tlsCert)
	if err != nil {
		return err
	}
	_, err = stapleOCSP(cfg.OCSP, cfg.Storage, &cert, nil)
	if err != nil && cfg.Logger != nil {
		cfg.Logger.Warn("stapling OCSP", zap.Error(err))
	}
	cfg.emit("cached_unmanaged_cert", cert.Names)
	cert.Tags = tags
	cfg.certCache.cacheCertificate(cert)
	return nil
}

// CacheUnmanagedCertificatePEMBytes makes a certificate out of the PEM bytes
// of the certificate and key, then caches it in memory.
//
// This method is safe for concurrent use.
func (cfg *Config) CacheUnmanagedCertificatePEMBytes(certBytes, keyBytes []byte, tags []string) error {
	cert, err := cfg.makeCertificateWithOCSP(certBytes, keyBytes)
	if err != nil {
		return err
	}
	cert.Tags = tags
	cfg.certCache.cacheCertificate(cert)
	cfg.emit("cached_unmanaged_cert", cert.Names)
	return nil
}

// makeCertificateFromDiskWithOCSP makes a Certificate by loading the
// certificate and key files. It fills out all the fields in
// the certificate except for the Managed and OnDemand flags.
// (It is up to the caller to set those.) It staples OCSP.
func (cfg Config) makeCertificateFromDiskWithOCSP(storage Storage, certFile, keyFile string) (Certificate, error) {
	certPEMBlock, err := ioutil.ReadFile(certFile)
	if err != nil {
		return Certificate{}, err
	}
	keyPEMBlock, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return Certificate{}, err
	}
	return cfg.makeCertificateWithOCSP(certPEMBlock, keyPEMBlock)
}

// makeCertificateWithOCSP is the same as makeCertificate except that it also
// staples OCSP to the certificate.
func (cfg Config) makeCertificateWithOCSP(certPEMBlock, keyPEMBlock []byte) (Certificate, error) {
	cert, err := makeCertificate(certPEMBlock, keyPEMBlock)
	if err != nil {
		return cert, err
	}
	_, err = stapleOCSP(cfg.OCSP, cfg.Storage, &cert, certPEMBlock)
	if err != nil && cfg.Logger != nil {
		cfg.Logger.Warn("stapling OCSP", zap.Error(err))
	}
	return cert, nil
}

// makeCertificate turns a certificate PEM bundle and a key PEM block into
// a Certificate with necessary metadata from parsing its bytes filled into
// its struct fields for convenience (except for the OnDemand and Managed
// flags; it is up to the caller to set those properties!). This function
// does NOT staple OCSP.
func makeCertificate(certPEMBlock, keyPEMBlock []byte) (Certificate, error) {
	var cert Certificate

	// Convert to a tls.Certificate
	tlsCert, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return cert, err
	}

	// Extract necessary metadata
	err = fillCertFromLeaf(&cert, tlsCert)
	if err != nil {
		return cert, err
	}

	return cert, nil
}

// fillCertFromLeaf populates cert from tlsCert. If it succeeds, it
// guarantees that cert.Leaf is non-nil.
func fillCertFromLeaf(cert *Certificate, tlsCert tls.Certificate) error {
	if len(tlsCert.Certificate) == 0 {
		return fmt.Errorf("certificate is empty")
	}
	cert.Certificate = tlsCert

	// the leaf cert should be the one for the site; we must set
	// the tls.Certificate.Leaf field so that TLS handshakes are
	// more efficient
	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return err
	}
	cert.Certificate.Leaf = leaf

	// for convenience, we do want to assemble all the
	// subjects on the certificate into one list
	if leaf.Subject.CommonName != "" { // TODO: CommonName is deprecated
		cert.Names = []string{strings.ToLower(leaf.Subject.CommonName)}
	}
	for _, name := range leaf.DNSNames {
		if name != leaf.Subject.CommonName { // TODO: CommonName is deprecated
			cert.Names = append(cert.Names, strings.ToLower(name))
		}
	}
	for _, ip := range leaf.IPAddresses {
		if ipStr := ip.String(); ipStr != leaf.Subject.CommonName { // TODO: CommonName is deprecated
			cert.Names = append(cert.Names, strings.ToLower(ipStr))
		}
	}
	for _, email := range leaf.EmailAddresses {
		if email != leaf.Subject.CommonName { // TODO: CommonName is deprecated
			cert.Names = append(cert.Names, strings.ToLower(email))
		}
	}
	for _, u := range leaf.URIs {
		if u.String() != leaf.Subject.CommonName { // TODO: CommonName is deprecated
			cert.Names = append(cert.Names, u.String())
		}
	}
	if len(cert.Names) == 0 {
		return fmt.Errorf("certificate has no names")
	}

	// save the hash of this certificate (chain) and
	// expiration date, for necessity and efficiency
	cert.hash = hashCertificateChain(cert.Certificate.Certificate)

	return nil
}

// managedCertInStorageExpiresSoon returns true if cert (being a
// managed certificate) is expiring within RenewDurationBefore.
// It returns false if there was an error checking the expiration
// of the certificate as found in storage, or if the certificate
// in storage is NOT expiring soon. A certificate that is expiring
// soon in our cache but is not expiring soon in storage probably
// means that another instance renewed the certificate in the
// meantime, and it would be a good idea to simply load the cert
// into our cache rather than repeating the renewal process again.
func (cfg *Config) managedCertInStorageExpiresSoon(cert Certificate) (bool, error) {
	certRes, err := cfg.loadCertResourceAnyIssuer(cert.Names[0])
	if err != nil {
		return false, err
	}
	_, needsRenew := cfg.managedCertNeedsRenewal(certRes)
	return needsRenew, nil
}

// reloadManagedCertificate reloads the certificate corresponding to the name(s)
// on oldCert into the cache, from storage. This also replaces the old certificate
// with the new one, so that all configurations that used the old cert now point
// to the new cert. It assumes that the new certificate for oldCert.Names[0] is
// already in storage.
func (cfg *Config) reloadManagedCertificate(oldCert Certificate) error {
	if cfg.Logger != nil {
		cfg.Logger.Info("reloading managed certificate", zap.Strings("identifiers", oldCert.Names))
	}
	newCert, err := cfg.loadManagedCertificate(oldCert.Names[0])
	if err != nil {
		return fmt.Errorf("loading managed certificate for %v from storage: %v", oldCert.Names, err)
	}
	cfg.certCache.replaceCertificate(oldCert, newCert)
	return nil
}

// SubjectQualifiesForCert returns true if subj is a name which,
// as a quick sanity check, looks like it could be the subject
// of a certificate. Requirements are:
// - must not be empty
// - must not start or end with a dot (RFC 1034)
// - must not contain common accidental special characters
func SubjectQualifiesForCert(subj string) bool {
	// must not be empty
	return strings.TrimSpace(subj) != "" &&

		// must not start or end with a dot
		!strings.HasPrefix(subj, ".") &&
		!strings.HasSuffix(subj, ".") &&

		// if it has a wildcard, must be a left-most label (or exactly "*"
		// which won't be trusted by browsers but still technically works)
		(!strings.Contains(subj, "*") || strings.HasPrefix(subj, "*.") || subj == "*") &&

		// must not contain other common special characters
		!strings.ContainsAny(subj, "()[]{}<> \t\n\"\\!@#$%^&|;'+=")
}

// SubjectQualifiesForPublicCert returns true if the subject
// name appears eligible for automagic TLS with a public
// CA such as Let's Encrypt. For example: localhost and IP
// addresses are not eligible because we cannot obtain certs
// for those names with a public CA. Wildcard names are
// allowed, as long as they conform to CABF requirements (only
// one wildcard label, and it must be the left-most label).
func SubjectQualifiesForPublicCert(subj string) bool {
	// must at least qualify for a certificate
	return SubjectQualifiesForCert(subj) &&

		// localhost, .localhost TLD, and .local TLD are ineligible
		!SubjectIsInternal(subj) &&

		// cannot be an IP address (as of yet), see
		// https://community.letsencrypt.org/t/certificate-for-static-ip/84/2?u=mholt
		!SubjectIsIP(subj) &&

		// only one wildcard label allowed, and it must be left-most, with 3+ labels
		(!strings.Contains(subj, "*") ||
			(strings.Count(subj, "*") == 1 &&
				strings.Count(subj, ".") > 1 &&
				len(subj) > 2 &&
				strings.HasPrefix(subj, "*.")))
}

// SubjectIsIP returns true if subj is an IP address.
func SubjectIsIP(subj string) bool {
	return net.ParseIP(subj) != nil
}

// SubjectIsInternal returns true if subj is an internal-facing
// hostname or address.
func SubjectIsInternal(subj string) bool {
	return subj == "localhost" ||
		strings.HasSuffix(subj, ".localhost") ||
		strings.HasSuffix(subj, ".local")
}

// MatchWildcard returns true if subject (a candidate DNS name)
// matches wildcard (a reference DNS name), mostly according to
// RFC 6125-compliant wildcard rules. See also RFC 2818 which
// states that IP addresses must match exactly, but this function
// does not attempt to distinguish IP addresses from internal or
// external DNS names that happen to look like IP addresses.
// It uses DNS wildcard matching logic.
// https://tools.ietf.org/html/rfc2818#section-3.1
func MatchWildcard(subject, wildcard string) bool {
	if subject == wildcard {
		return true
	}
	if !strings.Contains(wildcard, "*") {
		return false
	}
	labels := strings.Split(subject, ".")
	for i := range labels {
		if labels[i] == "" {
			continue // invalid label
		}
		labels[i] = "*"
		candidate := strings.Join(labels, ".")
		if candidate == wildcard {
			return true
		}
	}
	return false
}
