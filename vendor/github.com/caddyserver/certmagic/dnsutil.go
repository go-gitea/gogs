package certmagic

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Code in this file adapted from go-acme/lego, July 2020:
// https://github.com/go-acme/lego
// by Ludovic Fernandez and Dominik Menke
//
// It has been modified.

// findZoneByFQDN determines the zone apex for the given fqdn by recursing
// up the domain labels until the nameserver returns a SOA record in the
// answer section.
func findZoneByFQDN(fqdn string, nameservers []string) (string, error) {
	if !strings.HasSuffix(fqdn, ".") {
		fqdn += "."
	}
	soa, err := lookupSoaByFqdn(fqdn, nameservers)
	if err != nil {
		return "", err
	}
	return soa.zone, nil
}

func lookupSoaByFqdn(fqdn string, nameservers []string) (*soaCacheEntry, error) {
	if !strings.HasSuffix(fqdn, ".") {
		fqdn += "."
	}

	fqdnSOACacheMu.Lock()
	defer fqdnSOACacheMu.Unlock()

	// prefer cached version if fresh
	if ent := fqdnSOACache[fqdn]; ent != nil && !ent.isExpired() {
		return ent, nil
	}

	ent, err := fetchSoaByFqdn(fqdn, nameservers)
	if err != nil {
		return nil, err
	}

	// save result to cache, but don't allow
	// the cache to grow out of control
	if len(fqdnSOACache) >= 1000 {
		for key := range fqdnSOACache {
			delete(fqdnSOACache, key)
			break
		}
	}
	fqdnSOACache[fqdn] = ent

	return ent, nil
}

func fetchSoaByFqdn(fqdn string, nameservers []string) (*soaCacheEntry, error) {
	var err error
	var in *dns.Msg

	labelIndexes := dns.Split(fqdn)
	for _, index := range labelIndexes {
		domain := fqdn[index:]

		in, err = dnsQuery(domain, dns.TypeSOA, nameservers, true)
		if err != nil {
			continue
		}
		if in == nil {
			continue
		}

		switch in.Rcode {
		case dns.RcodeSuccess:
			// Check if we got a SOA RR in the answer section
			if len(in.Answer) == 0 {
				continue
			}

			// CNAME records cannot/should not exist at the root of a zone.
			// So we skip a domain when a CNAME is found.
			if dnsMsgContainsCNAME(in) {
				continue
			}

			for _, ans := range in.Answer {
				if soa, ok := ans.(*dns.SOA); ok {
					return newSoaCacheEntry(soa), nil
				}
			}
		case dns.RcodeNameError:
			// NXDOMAIN
		default:
			// Any response code other than NOERROR and NXDOMAIN is treated as error
			return nil, fmt.Errorf("unexpected response code '%s' for %s", dns.RcodeToString[in.Rcode], domain)
		}
	}

	return nil, fmt.Errorf("could not find the start of authority for %s%s", fqdn, formatDNSError(in, err))
}

// dnsMsgContainsCNAME checks for a CNAME answer in msg
func dnsMsgContainsCNAME(msg *dns.Msg) bool {
	for _, ans := range msg.Answer {
		if _, ok := ans.(*dns.CNAME); ok {
			return true
		}
	}
	return false
}

func dnsQuery(fqdn string, rtype uint16, nameservers []string, recursive bool) (*dns.Msg, error) {
	m := createDNSMsg(fqdn, rtype, recursive)
	var in *dns.Msg
	var err error
	for _, ns := range nameservers {
		in, err = sendDNSQuery(m, ns)
		if err == nil && len(in.Answer) > 0 {
			break
		}
	}
	return in, err
}

func createDNSMsg(fqdn string, rtype uint16, recursive bool) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(fqdn, rtype)
	m.SetEdns0(4096, false)
	if !recursive {
		m.RecursionDesired = false
	}
	return m
}

func sendDNSQuery(m *dns.Msg, ns string) (*dns.Msg, error) {
	udp := &dns.Client{Net: "udp", Timeout: dnsTimeout}
	in, _, err := udp.Exchange(m, ns)
	// two kinds of errors we can handle by retrying with TCP:
	// truncation and timeout; see https://github.com/caddyserver/caddy/issues/3639
	truncated := in != nil && in.Truncated
	timeoutErr := err != nil && strings.Contains(err.Error(), "timeout")
	if truncated || timeoutErr {
		tcp := &dns.Client{Net: "tcp", Timeout: dnsTimeout}
		in, _, err = tcp.Exchange(m, ns)
	}
	return in, err
}

func formatDNSError(msg *dns.Msg, err error) string {
	var parts []string
	if msg != nil {
		parts = append(parts, dns.RcodeToString[msg.Rcode])
	}
	if err != nil {
		parts = append(parts, err.Error())
	}
	if len(parts) > 0 {
		return ": " + strings.Join(parts, " ")
	}
	return ""
}

// soaCacheEntry holds a cached SOA record (only selected fields)
type soaCacheEntry struct {
	zone      string    // zone apex (a domain name)
	primaryNs string    // primary nameserver for the zone apex
	expires   time.Time // time when this cache entry should be evicted
}

func newSoaCacheEntry(soa *dns.SOA) *soaCacheEntry {
	return &soaCacheEntry{
		zone:      soa.Hdr.Name,
		primaryNs: soa.Ns,
		expires:   time.Now().Add(time.Duration(soa.Refresh) * time.Second),
	}
}

// isExpired checks whether a cache entry should be considered expired.
func (cache *soaCacheEntry) isExpired() bool {
	return time.Now().After(cache.expires)
}

// systemOrDefaultNameservers attempts to get system nameservers from the
// resolv.conf file given by path before falling back to hard-coded defaults.
func systemOrDefaultNameservers(path string, defaults []string) []string {
	config, err := dns.ClientConfigFromFile(path)
	if err != nil || len(config.Servers) == 0 {
		return defaults
	}
	return config.Servers
}

// populateNameserverPorts ensures that all nameservers have a port number.
func populateNameserverPorts(servers []string) {
	for i := range servers {
		_, port, _ := net.SplitHostPort(servers[i])
		if port == "" {
			servers[i] = net.JoinHostPort(servers[i], "53")
		}
	}
}

// checkDNSPropagation checks if the expected TXT record has been propagated to all authoritative nameservers.
func checkDNSPropagation(fqdn, value string, resolvers []string) (bool, error) {
	if !strings.HasSuffix(fqdn, ".") {
		fqdn += "."
	}

	// Initial attempt to resolve at the recursive NS
	r, err := dnsQuery(fqdn, dns.TypeTXT, resolvers, true)
	if err != nil {
		return false, err
	}

	// TODO: make this configurable, maybe
	// if !p.requireCompletePropagation {
	// 	return true, nil
	// }

	if r.Rcode == dns.RcodeSuccess {
		fqdn = updateDomainWithCName(r, fqdn)
	}

	authoritativeNss, err := lookupNameservers(fqdn, resolvers)
	if err != nil {
		return false, err
	}

	return checkAuthoritativeNss(fqdn, value, authoritativeNss)
}

// checkAuthoritativeNss queries each of the given nameservers for the expected TXT record.
func checkAuthoritativeNss(fqdn, value string, nameservers []string) (bool, error) {
	for _, ns := range nameservers {
		r, err := dnsQuery(fqdn, dns.TypeTXT, []string{net.JoinHostPort(ns, "53")}, false)
		if err != nil {
			return false, err
		}

		if r.Rcode != dns.RcodeSuccess {
			if r.Rcode == dns.RcodeNameError {
				// if Present() succeeded, then it must show up eventually, or else
				// something is really broken in the DNS provider or their API;
				// no need for error here, simply have the caller try again
				return false, nil
			}
			return false, fmt.Errorf("NS %s returned %s for %s", ns, dns.RcodeToString[r.Rcode], fqdn)
		}

		var found bool
		for _, rr := range r.Answer {
			if txt, ok := rr.(*dns.TXT); ok {
				record := strings.Join(txt.Txt, "")
				if record == value {
					found = true
					break
				}
			}
		}

		if !found {
			return false, nil
		}
	}

	return true, nil
}

// lookupNameservers returns the authoritative nameservers for the given fqdn.
func lookupNameservers(fqdn string, resolvers []string) ([]string, error) {
	var authoritativeNss []string

	zone, err := findZoneByFQDN(fqdn, resolvers)
	if err != nil {
		return nil, fmt.Errorf("could not determine the zone: %w", err)
	}

	r, err := dnsQuery(zone, dns.TypeNS, resolvers, true)
	if err != nil {
		return nil, err
	}

	for _, rr := range r.Answer {
		if ns, ok := rr.(*dns.NS); ok {
			authoritativeNss = append(authoritativeNss, strings.ToLower(ns.Ns))
		}
	}

	if len(authoritativeNss) > 0 {
		return authoritativeNss, nil
	}
	return nil, errors.New("could not determine authoritative nameservers")
}

// Update FQDN with CNAME if any
func updateDomainWithCName(r *dns.Msg, fqdn string) string {
	for _, rr := range r.Answer {
		if cn, ok := rr.(*dns.CNAME); ok {
			if cn.Hdr.Name == fqdn {
				return cn.Target
			}
		}
	}
	return fqdn
}

// recursiveNameservers are used to pre-check DNS propagation. It
// picks user-configured nameservers (custom) OR the defaults
// obtained from resolv.conf and defaultNameservers if none is
// configured and ensures that all server addresses have a port value.
func recursiveNameservers(custom []string) []string {
	var servers []string
	if len(custom) == 0 {
		servers = systemOrDefaultNameservers(defaultResolvConf, defaultNameservers)
	} else {
		servers = make([]string, len(custom))
		copy(servers, custom)
	}
	populateNameserverPorts(servers)
	return servers
}

var defaultNameservers = []string{
	"8.8.8.8:53",
	"8.8.4.4:53",
	"1.1.1.1:53",
	"1.0.0.1:53",
}

var dnsTimeout = 10 * time.Second

var (
	fqdnSOACache   = map[string]*soaCacheEntry{}
	fqdnSOACacheMu sync.Mutex
)

const defaultResolvConf = "/etc/resolv.conf"
