// Copyright 2019 The Gitea Authors. All rights reserved.
// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"regexp"
	"strings"

	"code.gitea.io/gitea/modules/setting"
)

var scpSyntax = regexp.MustCompile(`^([a-zA-Z0-9_]+@)?([a-zA-Z0-9._-]+):(.*)$`)

// SubModule submodule is a reference on git repository
type SubModule struct {
	Name string
	URL  string
}

// SubModuleFile represents a file with submodule type.
type SubModuleFile struct {
	*Commit

	refURL string
	refID  string
}

// NewSubModuleFile create a new submodule file
func NewSubModuleFile(c *Commit, refURL, refID string) *SubModuleFile {
	return &SubModuleFile{
		Commit: c,
		refURL: refURL,
		refID:  refID,
	}
}

func getRefURL(refURL, urlPrefix, repoFullName, sshDomain string) string {
	if refURL == "" {
		return ""
	}

	if alias, ok := setting.Git.SubModuleMap[refURL]; ok {
		return alias
	}

	refURI := strings.TrimSuffix(refURL, ".git")

	if alias, ok := setting.Git.SubModuleMap[refURI]; ok {
		return alias
	}

	prefixURL, _ := url.Parse(urlPrefix)
	urlPrefixHostname, _, err := net.SplitHostPort(prefixURL.Host)
	if err != nil {
		urlPrefixHostname = prefixURL.Host
	}

	if strings.HasSuffix(urlPrefix, "/") {
		urlPrefix = urlPrefix[:len(urlPrefix)-1]
	}

	// FIXME: Need to consider branch - which will require changes in modules/git/commit.go:GetSubModules
	// Relative url prefix check (according to git submodule documentation)
	if strings.HasPrefix(refURI, "./") || strings.HasPrefix(refURI, "../") {
		return urlPrefix + path.Clean(path.Join("/", repoFullName, refURI))
	}

	if !strings.Contains(refURI, "://") {
		// scp style syntax which contains *no* port number after the : (and is not parsed by net/url)
		// ex: git@try.gitea.io:go-gitea/gitea
		match := scpSyntax.FindAllStringSubmatch(refURI, -1)
		if len(match) > 0 {

			m := match[0]
			refHostname := m[2]
			pth := m[3]

			if !strings.HasPrefix(pth, "/") {
				pth = "/" + pth
			}

			if alias, ok := setting.Git.SubModuleMap[m[1]+refHostname]; ok {
				return alias + pth
			}

			if urlPrefixHostname == refHostname || refHostname == sshDomain {
				return urlPrefix + path.Clean(path.Join("/", pth))
			}
			return "http://" + refHostname + pth
		}
	}

	ref, err := url.Parse(refURI)
	if err != nil {
		return ""
	}

	refHostname, _, err := net.SplitHostPort(ref.Host)
	if err != nil {
		refHostname = ref.Host
	}

	supportedSchemes := []string{"http", "https", "git", "ssh", "git+ssh"}

	if len(setting.Git.SubModuleMap) > 0 && ref.Scheme != "" {
		if ref.Scheme == "ssh" {
			if len(ref.User.Username()) > 0 {
				if alias, ok := setting.Git.SubModuleMap[fmt.Sprintf("%v@%s:%s", ref.User, ref.Host, ref.Path[1:])]; ok {
					return alias
				}
				if alias, ok := setting.Git.SubModuleMap[fmt.Sprintf("%v@%s", ref.User, ref.Host)]; ok {
					return alias + ref.Path
				}
			} else if alias, ok := setting.Git.SubModuleMap[ref.Host+":"+ref.Path[1:]]; ok {
				return alias
			} else if alias, ok := setting.Git.SubModuleMap[ref.Host]; ok {
				return alias + ref.Path
			}
		}
		left := refURI
		right := ""
		for idx := strings.LastIndex(left, "/"); idx > len(ref.Scheme)+3; {
			right = left[idx:] + right
			left = left[:idx]
			if alias, ok := setting.Git.SubModuleMap[left]; ok {
				return alias + right
			}
			idx = strings.LastIndex(left, "/")
		}
	}

	for _, scheme := range supportedSchemes {
		if ref.Scheme == scheme {
			if ref.Scheme == "http" || ref.Scheme == "https" {
				if len(ref.User.Username()) > 0 {
					return ref.Scheme + "://" + fmt.Sprintf("%v", ref.User) + "@" + ref.Host + ref.Path
				}
				return ref.Scheme + "://" + ref.Host + ref.Path
			} else if urlPrefixHostname == refHostname || refHostname == sshDomain {
				return urlPrefix + path.Clean(path.Join("/", ref.Path))
			} else {
				return "http://" + refHostname + ref.Path
			}
		}
	}

	return ""
}

// RefURL guesses and returns reference URL.
func (sf *SubModuleFile) RefURL(urlPrefix, repoFullName, sshDomain string) string {
	return getRefURL(sf.refURL, urlPrefix, repoFullName, sshDomain)
}

// RefID returns reference ID.
func (sf *SubModuleFile) RefID() string {
	return sf.refID
}
