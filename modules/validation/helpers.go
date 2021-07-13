// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package validation

import (
	"net"
	"net/url"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

var loopbackIPBlocks []*net.IPNet

var externalTrackerRegex = regexp.MustCompile(`({?)(?:user|repo|index)+?(}?)`)

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8", // IPv4 loopback
		"::1/128",     // IPv6 loopback
	} {
		if _, block, err := net.ParseCIDR(cidr); err == nil {
			loopbackIPBlocks = append(loopbackIPBlocks, block)
		}
	}
}

func isLoopbackIP(ip string) bool {
	pip := net.ParseIP(ip)
	if pip == nil {
		return false
	}
	for _, block := range loopbackIPBlocks {
		if block.Contains(pip) {
			return true
		}
	}
	return false
}

// IsValidURL checks if URL is valid
func IsValidURL(uri string) bool {
	if u, err := url.ParseRequestURI(uri); err != nil ||
		(u.Scheme != "http" && u.Scheme != "https") ||
		!validPort(portOnly(u.Host)) {
		return false
	}

	return true
}

// IsValidSiteURL checks if URL is valid
func IsValidSiteURL(uri string) bool {
	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return false
	}

	if !validPort(portOnly(u.Host)) {
		return false
	}

	for _, scheme := range setting.Service.ValidSiteURLSchemes {
		if scheme == u.Scheme {
			return true
		}
	}
	return false
}

// IsAPIURL checks if URL is current Gitea instance API URL
func IsAPIURL(uri string) bool {
	return strings.HasPrefix(strings.ToLower(uri), strings.ToLower(setting.AppURL+"api"))
}

// IsValidExternalURL checks if URL is valid external URL
func IsValidExternalURL(uri string) bool {
	if !IsValidURL(uri) || IsAPIURL(uri) {
		return false
	}

	u, err := url.ParseRequestURI(uri)
	if err != nil {
		return false
	}

	// Currently check only if not loopback IP is provided to keep compatibility
	if isLoopbackIP(u.Hostname()) || strings.ToLower(u.Hostname()) == "localhost" {
		return false
	}

	// TODO: Later it should be added to allow local network IP addresses
	//       only if allowed by special setting

	return true
}

// IsValidExternalTrackerURLFormat checks if URL matches required syntax for external trackers
func IsValidExternalTrackerURLFormat(uri string) bool {
	if !IsValidExternalURL(uri) {
		return false
	}

	// check for typoed variables like /{index/ or /[repo}
	for _, match := range externalTrackerRegex.FindAllStringSubmatch(uri, -1) {
		if (match[1] == "{" || match[2] == "}") && (match[1] != "{" || match[2] != "}") {
			return false
		}
	}

	return true
}
