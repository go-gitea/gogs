// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package docker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"

	"code.gitea.io/gitea/models"
	"github.com/docker/distribution/registry/auth/token"
	"github.com/docker/libtrust"
)

// AuthzResult auth result
type AuthzResult struct {
	Scope            AuthScope
	AutorizedActions []string
}

// AuthScope auth scope
type AuthScope struct {
	Type    string
	Class   string
	Name    string
	Actions []string
}

// GenerateTokenOptions options to generate a token
type GenerateTokenOptions struct {
	Account      string
	IssuerName   string
	AuthzResults []AuthzResult
	PublicKey    *libtrust.PublicKey
	PrivateKey   *libtrust.PrivateKey
	ServiceName  string
	Expiration   int64
}

// GenerateToken generate token
func GenerateToken(opts GenerateTokenOptions) (string, error) {
	now := time.Now().Unix()

	// Sign something dummy to find out which algorithm is used.
	_, sigAlg, err := (*opts.PrivateKey).Sign(strings.NewReader("dummy"), 0)
	if err != nil {
		return "", fmt.Errorf("failed to sign: %s", err)
	}
	header := token.Header{
		Type:       "JWT",
		SigningAlg: sigAlg,
		KeyID:      (*opts.PublicKey).KeyID(),
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal header: %s", err)
	}

	claims := token.ClaimSet{
		Issuer:     opts.IssuerName,
		Subject:    opts.Account,
		Audience:   opts.ServiceName,
		NotBefore:  now - 10,
		IssuedAt:   now,
		Expiration: now + opts.Expiration,
		JWTID:      fmt.Sprintf("%d", rand.Int63()),
		Access:     []*token.ResourceActions{},
	}
	for _, a := range opts.AuthzResults {
		ra := &token.ResourceActions{
			Type:    a.Scope.Type,
			Name:    a.Scope.Name,
			Actions: a.AutorizedActions,
		}
		if ra.Actions == nil {
			ra.Actions = []string{}
		}
		sort.Strings(ra.Actions)
		claims.Access = append(claims.Access, ra)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %s", err)
	}

	payload := fmt.Sprintf("%s%s%s", joseBase64UrlEncode(headerJSON), token.TokenSeparator, joseBase64UrlEncode(claimsJSON))

	sig, sigAlg2, err := (*opts.PrivateKey).Sign(strings.NewReader(payload), 0)
	if err != nil || sigAlg2 != sigAlg {
		return "", fmt.Errorf("failed to sign token: %s", err)
	}
	return fmt.Sprintf("%s%s%s", payload, token.TokenSeparator, joseBase64UrlEncode(sig)), nil
}

// Copy-pasted from libtrust where it is private.
func joseBase64UrlEncode(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

// PermissionCheck Permission check
func PermissionCheck(doer *models.User, scops []AuthScope) ([]AuthzResult, error) {
	rs := make([]AuthzResult, 0, len(scops))
	repos := make(map[string]*models.Repository)
	repoPerms := make(map[int64]models.Permission)

	for _, scop := range scops {
		hasPush := false
		hasPull := false
		needCreate := false
		for _, act := range scop.Actions {
			if act == "push" {
				hasPush = true
				continue
			}
			if act == "pull" {
				hasPull = true
			}
		}
		if hasPush && doer == nil {
			continue
		}
		splits := strings.SplitN(scop.Name, "/", 3)
		var (
			owner    string
			repoName string
			image    string
		)
		if len(splits) != 3 {
			continue
		}
		owner = splits[0]
		repoName = splits[1]
		image = splits[2]

		repo, has := repos[owner+"/"+repoName]
		var err error
		if !has {
			repo, err = models.GetRepositoryByOwnerAndName(owner, repoName)
			if err != nil {
				if models.IsErrRepoNotExist(err) {
					continue
				}
				return nil, err
			}
			repos[owner+"/"+repoName] = repo
		}

		_, err = models.GetPackage(repo.ID, models.PackageTypeDockerImage, image)
		if err != nil {
			if !models.IsErrPackageNotExist(err) {
				return nil, err
			}
			needCreate = true
		}

		perm, has := repoPerms[repo.ID]
		if !has {
			perm, err = models.GetUserRepoPermission(repo, doer)
			if err != nil {
				return nil, err
			}
			repoPerms[repo.ID] = perm
		}

		events := make([]string, 0, 2)
		accessMode := models.AccessModeRead
		if hasPush {
			accessMode = models.AccessModeRead
			if needCreate {
				accessMode = models.AccessModeAdmin
			}
		}
		if perm.CanAccess(accessMode, models.UnitTypePackages) {
			if hasPush {
				events = append(events, "push")
			}
			if hasPull {
				events = append(events, "pull")
			}
		}

		if len(events) == 0 {
			continue
		}

		rs = append(rs, AuthzResult{
			Scope:            scop,
			AutorizedActions: events,
		})
	}

	return rs, nil
}

var resourceTypeRegex = regexp.MustCompile(`([a-z0-9]+)(\([a-z0-9]+\))?`)

// parseResourceType parse scope type
func parseResourceType(scope string) (string, string, error) {
	parts := resourceTypeRegex.FindStringSubmatch(scope)
	if parts == nil {
		return "", "", fmt.Errorf("malformed scope request")
	}

	switch len(parts) {
	case 3:
		return parts[1], "", nil
	case 4:
		return parts[1], parts[3], nil
	default:
		return "", "", fmt.Errorf("malformed scope request")
	}
}

// SplitScopes split scopes
func SplitScopes(scope string) ([]AuthScope, error) {
	scopes := strings.Split(scope, " ")
	rs := make([]AuthScope, 0, len(scopes))
	for _, scopeStr := range scopes {
		parts := strings.Split(scopeStr, ":")
		var scope AuthScope

		scopeType, scopeClass, err := parseResourceType(parts[0])
		if err != nil {
			return nil, err
		}

		switch len(parts) {
		case 3:
			scope = AuthScope{
				Type:    scopeType,
				Class:   scopeClass,
				Name:    parts[1],
				Actions: strings.Split(parts[2], ","),
			}
		case 4:
			scope = AuthScope{
				Type:    scopeType,
				Class:   scopeClass,
				Name:    parts[1] + ":" + parts[2],
				Actions: strings.Split(parts[3], ","),
			}
		default:
			return nil, fmt.Errorf("invalid scope: %q", scopeStr)
		}
		sort.Strings(scope.Actions)
		rs = append(rs, scope)
	}

	return rs, nil
}
