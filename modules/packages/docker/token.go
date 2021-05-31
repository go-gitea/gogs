// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

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

// TokenIssuer represents an issuer capable of generating JWT tokens
type TokenIssuer struct {
	Issuer     string
	Audience   string
	SigningKey crypto.PrivateKey
	Expiration int64 // second
}

// CreateJWT creates and signs a JSON Web Token for the given subject and
// audience with the granted access.
func (issuer *TokenIssuer) CreateJWT(account string, grantedAccessList []ResourceActions) (string, error) {
	randomBytes := make([]byte, 15)
	if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
		return "", err
	}
	randomID := base64.URLEncoding.EncodeToString(randomBytes)

	now := time.Now().Unix()
	token := ClaimSet{
		StandardClaims: jwt.StandardClaims{
			Subject:   account,
			Issuer:    issuer.Issuer,
			Audience:  issuer.Audience,
			ExpiresAt: now + issuer.Expiration,
			NotBefore: now,
			IssuedAt:  now,
			Id:        randomID,
		},
		Access: grantedAccessList,
	}

	var jwtToken *jwt.Token
	switch key := issuer.SigningKey.(type) {
	case *rsa.PrivateKey:
		jwtToken = jwt.NewWithClaims(jwt.SigningMethodRS256, token)
		jwtToken.Header["kid"] = keyIDEncode(key.Public())
	case *ecdsa.PrivateKey:
		jwtToken = jwt.NewWithClaims(jwt.SigningMethodES256, token)
		jwtToken.Header["kid"] = keyIDEncode(key.Public())
	default:
		return "", fmt.Errorf("unable to get PrivateKey %T", issuer.SigningKey)
	}

	return jwtToken.SignedString(issuer.SigningKey)
}

func keyIDEncode(pub crypto.PublicKey) string {
	derBytes, _ := x509.MarshalPKIXPublicKey(pub)
	sum := sha256.Sum256(derBytes)
	s := strings.TrimRight(base32.StdEncoding.EncodeToString(sum[:30]), "=")

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
