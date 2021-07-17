// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package oauth2

import (
	"fmt"
	"time"

	"code.gitea.io/gitea/modules/timeutil"
	"github.com/dgrijalva/jwt-go"
)

// ___________     __
// \__    ___/___ |  | __ ____   ____
//   |    | /  _ \|  |/ // __ \ /    \
//   |    |(  <_> )    <\  ___/|   |  \
//   |____| \____/|__|_ \\___  >___|  /
//                     \/    \/     \/

// Token represents an Oauth grant

// TokenType represents the type of token for an oauth application
type TokenType int

const (
	// TypeAccessToken is a token with short lifetime to access the api
	TypeAccessToken TokenType = 0
	// TypeRefreshToken is token with long lifetime to refresh access tokens obtained by the client
	TypeRefreshToken = iota
)

// Token represents a JWT token used to authenticate a client
type Token struct {
	GrantID int64     `json:"gnt"`
	Type    TokenType `json:"tt"`
	Counter int64     `json:"cnt,omitempty"`
	jwt.StandardClaims
}

// ParseToken parses a signed jwt string
func ParseToken(jwtToken string) (*Token, error) {
	parsedToken, err := jwt.ParseWithClaims(jwtToken, &Token{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method == nil || token.Method.Alg() != DefaultSigningKey.SigningMethod().Alg() {
			return nil, fmt.Errorf("unexpected signing algo: %v", token.Header["alg"])
		}
		return DefaultSigningKey.VerifyKey(), nil
	})
	if err != nil {
		return nil, err
	}
	var token *Token
	var ok bool
	if token, ok = parsedToken.Claims.(*Token); !ok || !parsedToken.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return token, nil
}

// SignToken signs the token with the JWT secret
func (token *Token) SignToken() (string, error) {
	token.IssuedAt = time.Now().Unix()
	jwtToken := jwt.NewWithClaims(DefaultSigningKey.SigningMethod(), token)
	DefaultSigningKey.PreProcessToken(jwtToken)
	return jwtToken.SignedString(DefaultSigningKey.SignKey())
}

// OIDCToken represents an OpenID Connect id_token
type OIDCToken struct {
	jwt.StandardClaims
	Nonce string `json:"nonce,omitempty"`

	// Scope profile
	Name              string             `json:"name,omitempty"`
	PreferredUsername string             `json:"preferred_username,omitempty"`
	Profile           string             `json:"profile,omitempty"`
	Picture           string             `json:"picture,omitempty"`
	Website           string             `json:"website,omitempty"`
	Locale            string             `json:"locale,omitempty"`
	UpdatedAt         timeutil.TimeStamp `json:"updated_at,omitempty"`

	// Scope email
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
}

// SignToken signs an id_token with the (symmetric) client secret key
func (token *OIDCToken) SignToken(signingKey JWTSigningKey) (string, error) {
	token.IssuedAt = time.Now().Unix()
	jwtToken := jwt.NewWithClaims(signingKey.SigningMethod(), token)
	signingKey.PreProcessToken(jwtToken)
	return jwtToken.SignedString(signingKey.SignKey())
}
