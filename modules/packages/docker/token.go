// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/auth/oauth2"
	"code.gitea.io/gitea/modules/setting"
	"github.com/dgrijalva/jwt-go"
)

// Resource describes a resource by type and name.
type Resource struct {
	Type  string
	Class string
	Name  string
}

// ResourceActions stores allowed actions on a named and typed resource.
type ResourceActions struct {
	Type    string   `json:"type"`
	Class   string   `json:"class,omitempty"`
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

// ClaimSet describes the main section of a JSON Web Token.
type ClaimSet struct {
	jwt.StandardClaims
	// Private claims
	Access []ResourceActions `json:"access"`
}

// SignToken signs an id_token with the (symmetric) client secret key
func (token *ClaimSet) SignToken(signingKey oauth2.JWTSigningKey) (string, error) {
	randomBytes := make([]byte, 15)
	if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
		return "", err
	}
	token.Id = base64.URLEncoding.EncodeToString(randomBytes)
	token.Audience = "gitea-token-service"
	token.Issuer = "gitea"
	token.IssuedAt = time.Now().Unix()
	token.NotBefore = token.IssuedAt
	token.ExpiresAt = token.IssuedAt + setting.OAuth2.AccessTokenExpirationTime
	jwtToken := jwt.NewWithClaims(signingKey.SigningMethod(), token)
	jwtToken.Header["kid"] = keyIDEncode(signingKey.KeyID()[:30])
	return jwtToken.SignedString(signingKey.SignKey())
}

// ResolveScopeList converts a scope list from a token request's
// `scope` parameter into a list of standard access objects.
func ResolveScopeList(scopeList string) []ResourceActions {
	scopeSpecs := strings.Split(scopeList, " ")
	accessSet := make(map[Resource]map[string]bool, 2*len(scopeSpecs))

	for _, scopeSpecifier := range scopeSpecs {
		// There should be 3 parts, separated by a `:` character.
		parts := strings.SplitN(scopeSpecifier, ":", 3)
		if len(parts) != 3 {
			continue
		}

		resourceType, resourceName, actions := parts[0], parts[1], parts[2]
		resourceType, resourceClass := splitResourceClass(resourceType)
		if resourceType == "" {
			continue
		}

		requestedResource := Resource{
			Type:  resourceType,
			Class: resourceClass,
			Name:  resourceName,
		}

		requestedAction, has := accessSet[requestedResource]
		if !has {
			requestedAction = make(map[string]bool, 2)
			accessSet[requestedResource] = requestedAction
		}
		// Actions should be a comma-separated list of actions.
		for _, action := range strings.Split(actions, ",") {
			requestedAction[action] = true
		}
	}

	requestedList := make([]ResourceActions, 0, len(accessSet))
	for resource, actions := range accessSet {
		ra := ResourceActions{
			Name:  resource.Name,
			Class: resource.Class,
			Type:  resource.Type,
		}
		for action := range actions {
			ra.Actions = append(ra.Actions, action)
		}
		sort.Strings(ra.Actions)
		requestedList = append(requestedList, ra)
	}
	return requestedList
}

var typeRegexp = regexp.MustCompile(`^([a-z0-9]+)(\([a-z0-9]+\))?$`)

func splitResourceClass(t string) (string, string) {
	matches := typeRegexp.FindStringSubmatch(t)
	if len(matches) < 2 {
		return "", ""
	}
	if len(matches) == 2 || len(matches[2]) < 2 {
		return matches[1], ""
	}
	return matches[1], matches[2][1 : len(matches[2])-1]
}

func keyIDEncode(b []byte) string {
	s := strings.TrimRight(base32.StdEncoding.EncodeToString(b), "=")
	var buf bytes.Buffer
	var i int
	for i = 0; i < len(s)/4-1; i++ {
		start := i * 4
		end := start + 4
		buf.WriteString(s[start:end] + ":")
	}
	buf.WriteString(s[i*4:])
	return buf.String()
}
